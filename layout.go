package main

// Layout is the keyboard layout the tray icon reports.
type Layout int

const (
	LayoutOther Layout = iota // anything else: the icon stays as it was
	LayoutPtBR                // Portuguese (Brazil), ABNT2
	LayoutEnUS                // English (United States), US-International
)

// Device-layout ids found in the high word of an HKL.
const (
	hklLayoutPtBR = 0xF010
	hklLayoutEnUS = 0xF001
)

// classifyHKL decides which layout a keyboard layout handle denotes. Like the
// Rust original it looks at the high word (the layout id), not the language id.
func classifyHKL(hkl uintptr) Layout {
	switch (hkl >> 16) & 0xFFFF {
	case hklLayoutPtBR:
		return LayoutPtBR
	case hklLayoutEnUS:
		return LayoutEnUS
	}
	return LayoutOther
}

// Tooltip is the text shown when hovering the tray icon.
func (l Layout) Tooltip() string {
	switch l {
	case LayoutPtBR:
		return "PT-BR (ABNT2)"
	case LayoutEnUS:
		return "US International"
	}
	return ""
}

// nextShown applies the "Other keeps the previous icon" rule: it returns the
// layout the tray should show after observing `seen`, and whether that differs
// from `shown`.
func nextShown(shown, seen Layout) (Layout, bool) {
	if seen == LayoutOther || seen == shown {
		return shown, false
	}
	return seen, true
}
