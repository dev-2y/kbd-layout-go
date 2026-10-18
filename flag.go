package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/draw"
	"image/png"
)

//go:embed assets/br_flag_32.png
var brFlagPNG []byte

//go:embed assets/us_flag_32.png
var usFlagPNG []byte

// Flag is a decoded flag: premultiplied BGRA, the byte order of a Windows
// 32-bit DIB, top row first.
type Flag struct {
	W, H int
	BGRA []byte
}

func decodeFlag(data []byte) (*Flag, error) {
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return nil, fmt.Errorf("empty image")
	}
	// RGBA is already premultiplied; only the channel order differs.
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), img, b.Min, draw.Src)
	f := &Flag{W: b.Dx(), H: b.Dy(), BGRA: make([]byte, len(rgba.Pix))}
	for i := 0; i < len(rgba.Pix); i += 4 {
		f.BGRA[i+0] = rgba.Pix[i+2]
		f.BGRA[i+1] = rgba.Pix[i+1]
		f.BGRA[i+2] = rgba.Pix[i+0]
		f.BGRA[i+3] = rgba.Pix[i+3]
	}
	return f, nil
}

// square scales the flag to the given width, keeping its aspect ratio, and
// centres it vertically on a transparent sz x sz canvas, which is what the
// shell wants for a tray icon. Scaling is an area average on premultiplied
// pixels, so at the flag's native size the result is the identical pixels.
func (f *Flag) square(sz int) []byte {
	out := make([]byte, sz*sz*4)
	dw := sz
	dh := (f.H*sz + f.W/2) / f.W
	if dh < 1 {
		dh = 1
	}
	if dh > sz {
		dh = sz
	}
	top := (sz - dh) / 2
	for y := 0; y < dh; y++ {
		sy0, sy1 := span(y, dh, f.H)
		for x := 0; x < dw; x++ {
			sx0, sx1 := span(x, dw, f.W)
			var acc [4]int
			n := 0
			for sy := sy0; sy < sy1; sy++ {
				for sx := sx0; sx < sx1; sx++ {
					p := f.BGRA[(sy*f.W+sx)*4:]
					acc[0] += int(p[0])
					acc[1] += int(p[1])
					acc[2] += int(p[2])
					acc[3] += int(p[3])
					n++
				}
			}
			o := ((top+y)*sz + x) * 4
			for c := 0; c < 4; c++ {
				out[o+c] = byte((acc[c] + n/2) / n)
			}
		}
	}
	return out
}

// span maps destination cell i of dn onto the source range [lo, hi) of sn,
// never empty (so upscaling repeats pixels).
func span(i, dn, sn int) (lo, hi int) {
	lo = i * sn / dn
	hi = ((i+1)*sn + dn - 1) / dn
	if hi <= lo {
		hi = lo + 1
	}
	if hi > sn {
		hi = sn
	}
	return
}
