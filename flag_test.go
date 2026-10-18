package main

import "testing"

func TestEmbeddedFlagsDecode(t *testing.T) {
	for name, data := range map[string][]byte{"br": brFlagPNG, "us": usFlagPNG} {
		f, err := decodeFlag(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if f.W != 32 || f.H != 22 || len(f.BGRA) != 32*22*4 {
			t.Errorf("%s: %dx%d, %d bytes", name, f.W, f.H, len(f.BGRA))
		}
	}
	if _, err := decodeFlag([]byte("nope")); err == nil {
		t.Error("garbage decoded")
	}
}

func TestFlagsDiffer(t *testing.T) {
	br, _ := decodeFlag(brFlagPNG)
	us, _ := decodeFlag(usFlagPNG)
	// centre pixel: BR is the blue disc (B dominant), US is not.
	c := (11*32 + 16) * 4
	if br.BGRA[c] < 100 || br.BGRA[c+2] > 50 {
		t.Errorf("BR centre not blue: %v", br.BGRA[c:c+4])
	}
	if string(br.BGRA) == string(us.BGRA) {
		t.Error("flags are identical")
	}
}

func TestSquareNativeSizeKeepsPixels(t *testing.T) {
	f, _ := decodeFlag(usFlagPNG)
	out := f.square(32)
	top := (32 - 22) / 2
	for y := 0; y < 22; y++ {
		for x := 0; x < 32; x++ {
			a := f.BGRA[(y*32+x)*4 : (y*32+x)*4+4]
			b := out[((top+y)*32+x)*4 : ((top+y)*32+x)*4+4]
			if string(a) != string(b) {
				t.Fatalf("pixel %d,%d: %v != %v", x, y, a, b)
			}
		}
	}
	// the bands above and below are transparent
	for i := 0; i < top*32*4; i++ {
		if out[i] != 0 || out[len(out)-1-i] != 0 {
			t.Fatal("padding not transparent")
		}
	}
}

func TestSquareSizes(t *testing.T) {
	f, _ := decodeFlag(brFlagPNG)
	for _, sz := range []int{16, 20, 24, 32, 40, 48, 64} {
		out := f.square(sz)
		if len(out) != sz*sz*4 {
			t.Fatalf("%d: len %d", sz, len(out))
		}
		// opaque flag pixels stay fully opaque; only padding is transparent
		opaque := 0
		for i := 3; i < len(out); i += 4 {
			if out[i] == 255 {
				opaque++
			} else if out[i] != 0 {
				t.Fatalf("%d: partial alpha %d at %d", sz, out[i], i)
			}
		}
		wantRows := (22*sz + 16) / 32
		if opaque != wantRows*sz {
			t.Errorf("%d: %d opaque pixels, want %d", sz, opaque, wantRows*sz)
		}
	}
}

func TestSpan(t *testing.T) {
	for dn := 1; dn <= 70; dn++ {
		for sn := 1; sn <= 40; sn++ {
			prev := 0
			for i := 0; i < dn; i++ {
				lo, hi := span(i, dn, sn)
				if lo < 0 || hi > sn || hi <= lo || lo < prev-1 {
					t.Fatalf("span(%d,%d,%d) = %d,%d", i, dn, sn, lo, hi)
				}
				prev = lo
			}
			if lo, _ := span(0, dn, sn); lo != 0 {
				t.Fatal("first span must start at 0")
			}
			if _, hi := span(dn-1, dn, sn); hi != sn {
				t.Fatalf("last span must end at %d, got %d", sn, hi)
			}
		}
	}
}

func TestSquareAveragesAndRounds(t *testing.T) {
	// 2x2 flag squeezed into one pixel: blue channel 1,0,0,1 averages to 0.5,
	// which must round up; alpha 255 everywhere stays 255.
	f := &Flag{W: 2, H: 2, BGRA: []byte{
		1, 0, 0, 255, 0, 0, 0, 255,
		0, 0, 0, 255, 1, 0, 0, 255,
	}}
	got := f.square(1)
	if got[0] != 1 || got[3] != 255 {
		t.Errorf("got %v", got)
	}
}
