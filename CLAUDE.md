# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**kbd-layout-go** is a Go port of the layout-tray Rust project. It's a Windows tray icon that shows the active keyboard layout as flags: 🇧🇷 for Portuguese (ABNT2) and 🇺🇸 for US-International.

**Key architecture**:
- Pure Windows API via syscall (no cgo, no external libraries)
- All resources embedded in the executable (PNG flags via `//go:embed`, icons via COFF .syso)
- Single-threaded Win32 message loop with timer-driven polling
- Event-driven tray icon management with DPI awareness

## Build & Development

### Building
```bash
# Standard build (runs tests, generates executable)
build.cmd

# Just generate the .syso resource file from app.ico
go run ./tools/mkrsrc -ico app.ico -arch amd64 -o rsrc_windows_amd64.syso

# Build executable directly
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w -H=windowsgui" -o layout-tray.exe .
```

### Testing
```bash
# Run all tests
go test ./...

# Run specific test file
go test ./layout_test.go ./layout.go

# Run with verbose output
go test -v ./...
```

### Code Style
- Follow standard Go formatting (`gofmt`)
- All Windows-specific code uses `//go:build windows`
- Use `uintptr` for Windows handles, never `int` or `uint32` directly
- Resources (`assets/`) are embedded via `//go:embed`

## Key Code Structure

### Core Components

1. **`main_windows.go`** - Entry point
   - Sets up mutex for single instance
   - Creates hidden Win32 window and message loop
   - Registers `TaskbarCreated` system message handler

2. **`tray_windows.go`** - Tray icon management
   - `g` global struct on UI thread
   - `currentLayout()` reads layout from foreground window's thread
   - `trayRefresh()` updates icon with scaled flag
   - `wndProc()` handles Win32 messages (timer, menu, DPI changes)

3. **`layout.go`** - Layout classification
   - `Layout` enum: `LayoutPtBR`, `LayoutEnUS`, `LayoutOther`
   - `classifyHKL()`: extracts layout ID from high word of HKL
   - Rule: `LayoutOther` keeps previous icon (no change)

4. **`flag.go`** - Flag image processing
   - PNG decoding and RGBA→BGRA conversion
   - `Flag.square()`: area-average scaling to square icon with transparent border
   - Premultiplied alpha handling for Windows DIB

5. **`win32_windows.go`** - Windows API bindings
   - Syscall procedure declarations (`procGetKeyboardLayout`, etc.)
   - Win32 structures (`wndClassEx`, `notifyIconData`, etc.)
   - Constants and helper functions (`utf16Ptr`, `loword`)

6. **`icon_windows.go`** - Icon creation
   - `iconFromBGRA()`: creates Win32 icon from BGRA pixels
   - `smallIconSize()`: gets tray icon size for current DPI

### Resource System

- **`tools/mkrsrc/main.go`** - COFF resource builder
  - Generates `.syso` file from `app.ico`
  - Embeds icon groups and version info
  - Pure Go, no windres or MinGW required

- **`assets/*.png`** - Flag images
  - 32x32 PNG with alpha
  - Embedded via `//go:embed` directives in `flag.go`

### Build Pipeline

1. `app.ico` → `tools/mkrsrc` → `rsrc_windows_amd64.syso`
2. Go linker embeds `.syso` as Windows resources
3. `build.cmd` runs tests, builds with GUI subsystem flags

## Windows-Specific Notes

### HKL Layout IDs
- `0xF010`: Portuguese Brazil (ABNT2)
- `0xF001`: US International
- High word of HKL identifies layout, low word is language ID
- Other layouts are classified as `LayoutOther`

### DPI Handling
- `procSetProcessDPIAware()` called at startup
- `smallIconSize()` queries current DPI for icon scaling
- `wmDPIChanged` message triggers icon refresh

### Single Instance
- Named mutex: `Local\LayoutTray_singleton`
- Handle kept open until process exits

### Explorer Restart Handling
- `TaskbarCreated` system message re-adds tray icon
- No manual tray icon cleanup needed

## Development Guidelines

### When Modifying
1. Test layout classification changes thoroughly (`layout_test.go`)
2. Verify flag scaling at different DPIs (150%, 200%)
3. Check single instance behavior
4. Ensure Explorer restart recovery works

### When Adding Features
- Keep dependency-free approach (stdlib only)
- Maintain Windows XP compatibility (no WinRT APIs)
- Preserve mutex and DPI awareness
- Update `tools/mkrsrc` if adding new resource types

### Performance Considerations
- Timer interval: 500ms (balance between responsiveness and CPU)
- Area-average scaling only happens on layout change
- Icons destroyed after replacement to avoid GDI leaks

## Troubleshooting

### Common Issues
- **Icon doesn't appear**: Check `TaskbarCreated` handling, DPI awareness
- **Wrong layout detected**: Verify foreground window thread ownership
- **Build fails**: Ensure `rsrc_windows_amd64.syso` exists or run `tools/mkrsrc`

### Debug Mode
Add console output by removing `-H=windowsgui` from linker flags (creates console window for debugging).
