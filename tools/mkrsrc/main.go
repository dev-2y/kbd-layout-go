// Command mkrsrc writes a COFF object (a .syso file) holding the executable's
// icon and version info. The Go linker picks up any *_windows_<arch>.syso in the
// package directory and embeds it as the exe's resources, which is what gives
// layout-tray.exe its file icon (and lets the program load its application
// icon by resource id 1).
//
// It uses only the standard library, so building the icon needs nothing but Go:
// no MinGW, no windres, no network.
//
//	go run ./tools/mkrsrc -ico app.ico -arch amd64 -o rsrc_windows_amd64.syso
package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

// Resource types.
const (
	rtIcon      = 3
	rtGroupIcon = 14
	rtVersion   = 16
)

const (
	langUSEnglish = 0x0409
	codepageUni   = 1200 // Unicode, for the Translation entry
	idApp         = 1    // lowest icon group id: the exe's file icon (IDI_APP)
)

// Version strings, the same as the Rust project name.
var versionStrings = [][2]string{
	{"FileDescription", "Layout Tray"},
	{"FileVersion", ""}, // filled with -version
	{"InternalName", "layout-tray"},
	{"OriginalFilename", "layout-tray.exe"},
	{"ProductName", "Layout Tray"},
	{"ProductVersion", ""}, // filled with -version
}

type arch struct {
	machine uint16
	reloc   uint16 // image-relative 32-bit address relocation
}

var archs = map[string]arch{
	"amd64": {0x8664, 0x0003}, // IMAGE_REL_AMD64_ADDR32NB
	"386":   {0x014c, 0x0007}, // IMAGE_REL_I386_DIR32NB
	"arm64": {0xAA64, 0x0002}, // IMAGE_REL_ARM64_ADDR32NB
}

func main() {
	icoPath := flag.String("ico", "app.ico", "icon file to embed")
	out := flag.String("o", "", "output .syso (default rsrc_windows_<arch>.syso)")
	archName := flag.String("arch", "amd64", "target architecture: amd64, 386 or arm64")
	version := flag.String("version", "1.1.0.0", "file and product version, four numbers")
	flag.Parse()

	a, ok := archs[*archName]
	if !ok {
		fatal(fmt.Errorf("unknown -arch %q", *archName))
	}
	if *out == "" {
		*out = "rsrc_windows_" + *archName + ".syso"
	}

	ico, err := os.ReadFile(*icoPath)
	if err != nil {
		fatal(err)
	}
	res, err := buildResources(ico, *version)
	if err != nil {
		fatal(err)
	}
	section, relocs := buildSection(res)
	if err := os.WriteFile(*out, buildCOFF(a, section, relocs), 0o644); err != nil {
		fatal(err)
	}
	fmt.Printf("wrote %s (%d resources, %d bytes of resource data)\n", *out, len(res), len(section))
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "mkrsrc:", err)
	os.Exit(1)
}

// ---------------------------------------------------------------- resources

type resource struct {
	typ, id uint16
	data    []byte
}

// buildResources turns an .ico file and a version into the resources the exe
// needs: one RT_ICON per image, one RT_GROUP_ICON listing them, and RT_VERSION.
func buildResources(ico []byte, version string) ([]resource, error) {
	images, err := parseICO(ico)
	if err != nil {
		return nil, err
	}

	var res []resource
	group := new(bytes.Buffer)
	le := binary.LittleEndian
	binary.Write(group, le, [3]uint16{0, 1, uint16(len(images))}) // GRPICONDIR

	for i, im := range images {
		id := uint16(i + 1)
		res = append(res, resource{rtIcon, id, im.data})
		group.Write([]byte{im.width, im.height, im.colors, 0}) // GRPICONDIRENTRY
		binary.Write(group, le, im.planes)
		binary.Write(group, le, im.bitCount)
		binary.Write(group, le, uint32(len(im.data)))
		binary.Write(group, le, id)
	}
	res = append(res, resource{rtGroupIcon, idApp, group.Bytes()})

	vi, err := buildVersionInfo(version)
	if err != nil {
		return nil, err
	}
	res = append(res, resource{rtVersion, 1, vi})
	return res, nil
}

type icoImage struct {
	width, height, colors byte
	planes, bitCount      uint16
	data                  []byte // the image exactly as stored in the .ico (DIB or PNG)
}

func parseICO(b []byte) ([]icoImage, error) {
	le := binary.LittleEndian
	if len(b) < 6 || le.Uint16(b[0:]) != 0 || le.Uint16(b[2:]) != 1 {
		return nil, errors.New("not an .ico file")
	}
	n := int(le.Uint16(b[4:]))
	if n == 0 || len(b) < 6+16*n {
		return nil, errors.New("icon directory is truncated")
	}
	images := make([]icoImage, n)
	for i := range images {
		e := b[6+16*i:]
		size, off := int(le.Uint32(e[8:])), int(le.Uint32(e[12:]))
		if off < 6+16*n || size <= 0 || off+size > len(b) {
			return nil, fmt.Errorf("icon image %d points outside the file", i)
		}
		images[i] = icoImage{
			width: e[0], height: e[1], colors: e[2],
			planes: le.Uint16(e[4:]), bitCount: le.Uint16(e[6:]),
			data: b[off : off+size],
		}
	}
	return images, nil
}

// ------------------------------------------------------------- version info

// buildVersionInfo writes a VS_VERSIONINFO structure: fixed file info, a
// StringFileInfo with the strings above (US English, Unicode) and a VarFileInfo
// with the matching Translation. Every node starts on a 32-bit boundary and its
// wLength covers its own header, key, value and children.
func buildVersionInfo(version string) ([]byte, error) {
	var v [4]uint16
	parts := strings.Split(version, ".")
	if len(parts) != 4 {
		return nil, fmt.Errorf("version %q: want four numbers like 1.1.0.0", version)
	}
	for i, p := range parts {
		n, err := strconv.ParseUint(p, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("version %q: %v", version, err)
		}
		v[i] = uint16(n)
	}
	ms, ls := uint32(v[0])<<16|uint32(v[1]), uint32(v[2])<<16|uint32(v[3])

	// VS_FIXEDFILEINFO
	fixed := new(bytes.Buffer)
	for _, d := range []uint32{
		0xFEEF04BD,     // dwSignature
		0x00010000,     // dwStrucVersion
		ms, ls, ms, ls, // file version, product version
		0x3F,       // dwFileFlagsMask
		0,          // dwFileFlags
		0x00040004, // dwFileOS: VOS_NT_WINDOWS32
		1,          // dwFileType: VFT_APP
		0, 0, 0,    // subtype, date
	} {
		binary.Write(fixed, binary.LittleEndian, d)
	}

	var strs [][]byte
	for _, kv := range versionStrings {
		val := kv[1]
		if val == "" {
			val = version
		}
		u := utf16Z(val)
		strs = append(strs, versionNode(kv[0], uint16(len(u)/2), 1, u, nil))
	}
	table := versionNode(fmt.Sprintf("%04X%04X", langUSEnglish, codepageUni), 0, 1, nil, strs)
	stringFileInfo := versionNode("StringFileInfo", 0, 1, nil, [][]byte{table})

	translation := make([]byte, 4)
	binary.LittleEndian.PutUint16(translation[0:], langUSEnglish)
	binary.LittleEndian.PutUint16(translation[2:], codepageUni)
	varFileInfo := versionNode("VarFileInfo", 0, 1, nil,
		[][]byte{versionNode("Translation", 4, 0, translation, nil)})

	return versionNode("VS_VERSION_INFO", uint16(fixed.Len()), 0, fixed.Bytes(),
		[][]byte{stringFileInfo, varFileInfo}), nil
}

// versionNode builds one node: wLength, wValueLength, wType, key, padding,
// value, then each child preceded by padding.
func versionNode(key string, valueLen, typ uint16, value []byte, children [][]byte) []byte {
	var b bytes.Buffer
	le := binary.LittleEndian
	binary.Write(&b, le, [3]uint16{0, valueLen, typ}) // wLength patched below
	b.Write(utf16Z(key))
	pad4(&b)
	b.Write(value)
	for _, c := range children {
		pad4(&b)
		b.Write(c)
	}
	out := b.Bytes()
	le.PutUint16(out[0:], uint16(len(out)))
	return out
}

func utf16Z(s string) []byte {
	u := utf16.Encode([]rune(s))
	u = append(u, 0)
	out := make([]byte, 2*len(u))
	for i, c := range u {
		binary.LittleEndian.PutUint16(out[2*i:], c)
	}
	return out
}

func pad4(b *bytes.Buffer) {
	for b.Len()%4 != 0 {
		b.WriteByte(0)
	}
}

func align4(n int) int { return (n + 3) &^ 3 }

// ------------------------------------------------------- resource directory

// buildSection lays out the resource tree (type -> id -> one language) and the
// data. It returns the section bytes and the offset of every data entry's
// OffsetToData field: those hold section-relative offsets that the linker must
// turn into image-relative addresses, hence one relocation each.
func buildSection(res []resource) (section []byte, relocs []uint32) {
	res = append([]resource(nil), res...)
	sort.Slice(res, func(i, j int) bool {
		if res[i].typ != res[j].typ {
			return res[i].typ < res[j].typ
		}
		return res[i].id < res[j].id
	})

	var types []uint16
	idsOf := map[uint16][]int{} // type -> indexes into res, ascending by id
	for i, r := range res {
		if len(types) == 0 || types[len(types)-1] != r.typ {
			types = append(types, r.typ)
		}
		idsOf[r.typ] = append(idsOf[r.typ], i)
	}

	// Layout: root | type directories | id directories | data entries | data.
	off := 16 + 8*len(types)
	typeDir := map[uint16]int{}
	for _, t := range types {
		typeDir[t] = off
		off += 16 + 8*len(idsOf[t])
	}
	idDir := make([]int, len(res))
	for i := range res {
		idDir[i] = off
		off += 16 + 8 // one language per resource
	}
	entry := make([]int, len(res))
	for i := range res {
		entry[i] = off
		off += 16
	}
	blob := make([]int, len(res))
	for i, r := range res {
		off = align4(off)
		blob[i] = off
		off += len(r.data)
	}
	section = make([]byte, align4(off))

	le := binary.LittleEndian
	const subdir = 0x80000000
	dir := func(at int, entries [][2]uint32) { // IMAGE_RESOURCE_DIRECTORY + entries
		le.PutUint16(section[at+14:], uint16(len(entries))) // NumberOfIdEntries
		for i, e := range entries {
			le.PutUint32(section[at+16+8*i:], e[0])
			le.PutUint32(section[at+16+8*i+4:], e[1])
		}
	}

	var root [][2]uint32
	for _, t := range types {
		root = append(root, [2]uint32{uint32(t), subdir | uint32(typeDir[t])})
		var ids [][2]uint32
		for _, i := range idsOf[t] {
			ids = append(ids, [2]uint32{uint32(res[i].id), subdir | uint32(idDir[i])})
		}
		dir(typeDir[t], ids)
	}
	dir(0, root)

	for i, r := range res {
		dir(idDir[i], [][2]uint32{{langUSEnglish, uint32(entry[i])}})
		// IMAGE_RESOURCE_DATA_ENTRY: OffsetToData, Size, CodePage, Reserved
		le.PutUint32(section[entry[i]:], uint32(blob[i]))
		le.PutUint32(section[entry[i]+4:], uint32(len(r.data)))
		relocs = append(relocs, uint32(entry[i]))
		copy(section[blob[i]:], r.data)
	}
	return section, relocs
}

// -------------------------------------------------------------------- COFF

// buildCOFF wraps the resource section in a minimal COFF object file: one
// ".rsrc" section, the relocations for the data entries, a single section
// symbol and an empty string table.
func buildCOFF(a arch, section []byte, relocs []uint32) []byte {
	var b bytes.Buffer
	le := binary.LittleEndian
	w := func(v any) { binary.Write(&b, le, v) }

	const headerSize = 20 + 40
	symbolTable := headerSize + len(section) + 10*len(relocs)

	// IMAGE_FILE_HEADER
	w(a.machine)
	w(uint16(1)) // NumberOfSections
	w(uint32(0)) // TimeDateStamp: zero, so the output is reproducible
	w(uint32(symbolTable))
	w(uint32(1)) // NumberOfSymbols
	w(uint16(0)) // SizeOfOptionalHeader
	w(uint16(0)) // Characteristics

	// IMAGE_SECTION_HEADER
	var name [8]byte
	copy(name[:], ".rsrc")
	b.Write(name[:])
	w(uint32(0))                         // VirtualSize
	w(uint32(0))                         // VirtualAddress
	w(uint32(len(section)))              // SizeOfRawData
	w(uint32(headerSize))                // PointerToRawData
	w(uint32(headerSize + len(section))) // PointerToRelocations
	w(uint32(0))                         // PointerToLinenumbers
	w(uint16(len(relocs)))               // NumberOfRelocations
	w(uint16(0))                         // NumberOfLinenumbers
	w(uint32(0x40000040))                // CNT_INITIALIZED_DATA | MEM_READ

	b.Write(section)

	for _, r := range relocs { // IMAGE_RELOCATION: VirtualAddress, SymbolTableIndex, Type
		w(r)
		w(uint32(0))
		w(a.reloc)
	}

	// IMAGE_SYMBOL: the .rsrc section symbol that the relocations refer to.
	b.Write(name[:])
	w(uint32(0)) // Value
	w(int16(1))  // SectionNumber
	w(uint16(0)) // Type
	w(uint8(3))  // StorageClass: IMAGE_SYM_CLASS_STATIC
	w(uint8(0))  // NumberOfAuxSymbols

	w(uint32(4)) // string table: just its own size
	return b.Bytes()
}
