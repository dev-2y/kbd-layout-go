package main

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"os"
	"sort"
	"testing"
	"unicode/utf16"
)

const icoPath = "../../app.ico"

func realResources(t *testing.T) ([]resource, []byte) {
	t.Helper()
	ico, err := os.ReadFile(icoPath)
	if err != nil {
		t.Fatal(err)
	}
	res, err := buildResources(ico, "1.1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	return res, ico
}

// readTree walks the resource directory exactly as the loader does (named
// entries are not used here) and returns type -> id -> data, plus the offsets of
// the data entries.
func readTree(t *testing.T, sec []byte) (map[uint16]map[uint16][]byte, []uint32) {
	t.Helper()
	le := binary.LittleEndian
	out := map[uint16]map[uint16][]byte{}
	var entries []uint32

	readDir := func(at int) [][2]uint32 {
		if le.Uint16(sec[at+12:]) != 0 {
			t.Fatalf("dir at %d has named entries", at)
		}
		n := int(le.Uint16(sec[at+14:]))
		var e [][2]uint32
		var prev int64 = -1
		for i := 0; i < n; i++ {
			id := le.Uint32(sec[at+16+8*i:])
			if int64(id) <= prev {
				t.Fatalf("dir at %d: ids not strictly ascending (%d after %d)", at, id, prev)
			}
			prev = int64(id)
			e = append(e, [2]uint32{id, le.Uint32(sec[at+16+8*i+4:])})
		}
		return e
	}

	for _, te := range readDir(0) {
		if te[1]&0x80000000 == 0 {
			t.Fatal("type entry is not a subdirectory")
		}
		typ := uint16(te[0])
		out[typ] = map[uint16][]byte{}
		for _, ie := range readDir(int(te[1] &^ 0x80000000)) {
			if ie[1]&0x80000000 == 0 {
				t.Fatal("id entry is not a subdirectory")
			}
			langs := readDir(int(ie[1] &^ 0x80000000))
			if len(langs) != 1 || langs[0][0] != langUSEnglish || langs[0][1]&0x80000000 != 0 {
				t.Fatalf("bad language directory: %v", langs)
			}
			de := int(langs[0][1])
			entries = append(entries, uint32(de))
			off, size := int(le.Uint32(sec[de:])), int(le.Uint32(sec[de+4:]))
			if off%4 != 0 {
				t.Fatalf("data at %d is not 4-aligned", off)
			}
			out[typ][uint16(ie[0])] = sec[off : off+size]
		}
	}
	return out, entries
}

func TestResourceTreeMatchesTheIcon(t *testing.T) {
	res, ico := realResources(t)
	images, err := parseICO(ico)
	if err != nil {
		t.Fatal(err)
	}
	section, relocs := buildSection(res)
	tree, entries := readTree(t, section)

	if len(tree) != 3 {
		t.Fatalf("got %d resource types, want 3 (icon, group icon, version)", len(tree))
	}
	if len(tree[rtIcon]) != len(images) {
		t.Fatalf("%d RT_ICON resources for %d images", len(tree[rtIcon]), len(images))
	}
	for i, im := range images {
		if !bytes.Equal(tree[rtIcon][uint16(i+1)], im.data) {
			t.Errorf("RT_ICON %d differs from image %d in the .ico", i+1, i)
		}
	}

	// The group must list every icon, with matching sizes and RT_ICON ids.
	group, ok := tree[rtGroupIcon][idApp]
	if !ok || len(tree[rtGroupIcon]) != 1 {
		t.Fatalf("want exactly one group icon with id %d", idApp)
	}
	le := binary.LittleEndian
	if le.Uint16(group[0:]) != 0 || le.Uint16(group[2:]) != 1 || int(le.Uint16(group[4:])) != len(images) {
		t.Fatalf("bad GRPICONDIR header % x", group[:6])
	}
	if len(group) != 6+14*len(images) {
		t.Fatalf("group is %d bytes, want %d", len(group), 6+14*len(images))
	}
	for i, im := range images {
		e := group[6+14*i:]
		if e[0] != im.width || e[1] != im.height || e[2] != im.colors ||
			le.Uint16(e[4:]) != im.planes || le.Uint16(e[6:]) != im.bitCount ||
			int(le.Uint32(e[8:])) != len(im.data) || le.Uint16(e[12:]) != uint16(i+1) {
			t.Errorf("group entry %d does not describe image %d: % x", i, i, e[:14])
		}
	}

	// One relocation per data entry, at the OffsetToData field.
	sort.Slice(relocs, func(i, j int) bool { return relocs[i] < relocs[j] })
	sort.Slice(entries, func(i, j int) bool { return entries[i] < entries[j] })
	if len(relocs) != len(entries) {
		t.Fatalf("%d relocations for %d data entries", len(relocs), len(entries))
	}
	for i := range relocs {
		if relocs[i] != entries[i] {
			t.Errorf("relocation %d at %d, data entry at %d", i, relocs[i], entries[i])
		}
	}
}

type vnode struct {
	key      string
	valueLen int
	typ      int
	value    []byte
	children []vnode
}

func readUTF16Z(b []byte) (string, int) {
	var u []uint16
	for i := 0; i+1 < len(b); i += 2 {
		c := binary.LittleEndian.Uint16(b[i:])
		if c == 0 {
			return string(utf16.Decode(u)), i + 2
		}
		u = append(u, c)
	}
	return string(utf16.Decode(u)), len(b)
}

// parseVNode reads one node starting at b[0] and returns it with its wLength.
func parseVNode(t *testing.T, b []byte) (vnode, int) {
	t.Helper()
	le := binary.LittleEndian
	length := int(le.Uint16(b[0:]))
	if length < 6 || length > len(b) {
		t.Fatalf("bad wLength %d (have %d bytes)", length, len(b))
	}
	n := vnode{valueLen: int(le.Uint16(b[2:])), typ: int(le.Uint16(b[4:]))}
	key, kn := readUTF16Z(b[6:length])
	n.key = key
	pos := align4(6 + kn)
	vbytes := n.valueLen
	if n.typ == 1 {
		vbytes *= 2 // text values count characters
	}
	n.value = b[pos : pos+vbytes]
	pos += vbytes
	for align4(pos) < length {
		pos = align4(pos)
		c, cl := parseVNode(t, b[pos:length])
		n.children = append(n.children, c)
		pos += cl
	}
	return n, length
}

func TestVersionInfoStructure(t *testing.T) {
	res, _ := realResources(t)
	var blob []byte
	for _, r := range res {
		if r.typ == rtVersion {
			blob = r.data
		}
	}
	root, length := parseVNode(t, blob)
	if length != len(blob) {
		t.Fatalf("wLength %d, blob is %d bytes", length, len(blob))
	}
	if root.key != "VS_VERSION_INFO" || root.typ != 0 || root.valueLen != 52 {
		t.Fatalf("bad root: %q type %d value %d", root.key, root.typ, root.valueLen)
	}

	le := binary.LittleEndian
	if le.Uint32(root.value[0:]) != 0xFEEF04BD {
		t.Fatal("bad VS_FIXEDFILEINFO signature")
	}
	if le.Uint32(root.value[8:]) != 1<<16|1 || le.Uint32(root.value[12:]) != 0 ||
		le.Uint32(root.value[16:]) != 1<<16|1 || le.Uint32(root.value[20:]) != 0 {
		t.Error("file/product version is not 1.1.0.0")
	}
	if le.Uint32(root.value[32:]) != 0x00040004 || le.Uint32(root.value[36:]) != 1 {
		t.Error("wrong OS or file type")
	}

	if len(root.children) != 2 || root.children[0].key != "StringFileInfo" || root.children[1].key != "VarFileInfo" {
		t.Fatalf("unexpected children: %+v", root.children)
	}
	tables := root.children[0].children
	if len(tables) != 1 || tables[0].key != "040904B0" {
		t.Fatalf("unexpected string tables: %+v", tables)
	}
	got := map[string]string{}
	for _, s := range tables[0].children {
		v, _ := readUTF16Z(s.value)
		got[s.key] = v
		// wValueLength of a text value is its length in UTF-16 units, NUL included.
		if want := len(utf16.Encode([]rune(v))) + 1; s.valueLen != want {
			t.Errorf("%s: wValueLength %d, want %d", s.key, s.valueLen, want)
		}
	}
	want := map[string]string{
		"FileDescription":  "Layout Tray",
		"FileVersion":      "1.1.0.0",
		"InternalName":     "layout-tray",
		"OriginalFilename": "layout-tray.exe",
		"ProductName":      "Layout Tray",
		"ProductVersion":   "1.1.0.0",
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s = %q, want %q", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Errorf("got %d strings, want %d: %v", len(got), len(want), got)
	}

	vars := root.children[1].children
	if len(vars) != 1 || vars[0].key != "Translation" || !bytes.Equal(vars[0].value, []byte{0x09, 0x04, 0xB0, 0x04}) {
		t.Errorf("bad Translation: %+v", vars)
	}
}

func TestVersionRejectsGarbage(t *testing.T) {
	for _, v := range []string{"", "1.1", "1.1.0", "1.1.0.0.0", "a.b.c.d", "1.1.0.70000"} {
		if _, err := buildVersionInfo(v); err == nil {
			t.Errorf("version %q accepted", v)
		}
	}
}

func TestParseICORejectsGarbage(t *testing.T) {
	if _, err := parseICO([]byte("not an icon")); err == nil {
		t.Error("garbage accepted")
	}
	if _, err := parseICO([]byte{0, 0, 1, 0, 1, 0}); err == nil {
		t.Error("truncated directory accepted")
	}
	bad := []byte{0, 0, 1, 0, 1, 0, 16, 16, 0, 0, 1, 0, 32, 0, 0xFF, 0, 0, 0, 22, 0, 0, 0} // size 255, only 0 bytes follow
	if _, err := parseICO(bad); err == nil {
		t.Error("image outside the file accepted")
	}
}

// The object must be something the standard COFF reader (and so the Go linker's
// loader) understands: one .rsrc section, its relocations and one symbol.
func TestCOFFObjectIsWellFormed(t *testing.T) {
	res, _ := realResources(t)
	section, relocs := buildSection(res)
	for name, a := range archs {
		obj := buildCOFF(a, section, relocs)
		f, err := pe.NewFile(bytes.NewReader(obj))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if f.Machine != a.machine || len(f.Sections) != 1 {
			t.Fatalf("%s: machine 0x%x, %d sections", name, f.Machine, len(f.Sections))
		}
		s := f.Sections[0]
		if s.Name != ".rsrc" || int(s.Size) != len(section) {
			t.Fatalf("%s: section %q size %d", name, s.Name, s.Size)
		}
		data, err := s.Data()
		if err != nil || !bytes.Equal(data, section) {
			t.Fatalf("%s: section data differs (%v)", name, err)
		}
		if len(s.Relocs) != len(relocs) {
			t.Fatalf("%s: %d relocations, want %d", name, len(s.Relocs), len(relocs))
		}
		for i, r := range s.Relocs {
			if r.VirtualAddress != relocs[i] || r.SymbolTableIndex != 0 || r.Type != a.reloc {
				t.Fatalf("%s: relocation %d = %+v", name, i, r)
			}
		}
		if len(f.COFFSymbols) != 1 || f.COFFSymbols[0].SectionNumber != 1 {
			t.Fatalf("%s: symbols %+v", name, f.COFFSymbols)
		}
		if n, _ := f.COFFSymbols[0].FullName(f.StringTable); n != ".rsrc" {
			t.Fatalf("%s: symbol name %q", name, n)
		}
	}
}

func TestDeterministic(t *testing.T) {
	res, _ := realResources(t)
	s1, r1 := buildSection(res)
	s2, r2 := buildSection(res)
	a := archs["amd64"]
	if !bytes.Equal(buildCOFF(a, s1, r1), buildCOFF(a, s2, r2)) {
		t.Fatal("output is not reproducible")
	}
}
