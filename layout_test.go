package main

import "testing"

func TestClassifyHKL(t *testing.T) {
	cases := []struct {
		hkl  uintptr
		want Layout
	}{
		{0xF0100416, LayoutPtBR}, // pt-BR ABNT2
		{0xF0010409, LayoutEnUS}, // en-US, US-International
		{0x04160416, LayoutOther},
		{0x04090409, LayoutOther}, // plain US keyboard
		{0xF0110409, LayoutOther},
		{0xF0100000, LayoutPtBR}, // language id is ignored
		{0xF010, LayoutOther},    // high word zero
		{0, LayoutOther},
	}
	for _, c := range cases {
		if got := classifyHKL(c.hkl); got != c.want {
			t.Errorf("%#x: got %d, want %d", c.hkl, got, c.want)
		}
	}
}

func TestClassifyHKLIgnoresBitsAbove32(t *testing.T) {
	if uint64(^uintptr(0)) > 0xFFFFFFFF {
		if got := classifyHKL(uintptr(0xFFFF_0000_0000) | 0xF0100416); got != LayoutPtBR {
			t.Errorf("got %d", got)
		}
	}
}

func TestNextShown(t *testing.T) {
	cases := []struct {
		shown, seen, want Layout
		changed           bool
	}{
		{LayoutEnUS, LayoutPtBR, LayoutPtBR, true},
		{LayoutPtBR, LayoutEnUS, LayoutEnUS, true},
		{LayoutPtBR, LayoutPtBR, LayoutPtBR, false},
		{LayoutPtBR, LayoutOther, LayoutPtBR, false}, // keeps the previous icon
		{LayoutEnUS, LayoutOther, LayoutEnUS, false},
	}
	for _, c := range cases {
		got, changed := nextShown(c.shown, c.seen)
		if got != c.want || changed != c.changed {
			t.Errorf("%d->%d: got %d,%v", c.shown, c.seen, got, changed)
		}
	}
}

func TestTooltips(t *testing.T) {
	if LayoutPtBR.Tooltip() != "PT-BR (ABNT2)" || LayoutEnUS.Tooltip() != "US International" || LayoutOther.Tooltip() != "" {
		t.Fatal("tooltip text changed")
	}
}
