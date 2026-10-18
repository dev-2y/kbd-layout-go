@echo off
setlocal

cd /d "%~dp0"

where go >nul 2>&1
if errorlevel 1 (
    echo go not found. Install Go with:
    echo     winget install --id GoLang.Go
    exit /b 1
)

rem Icon and version info: tools\mkrsrc is a small Go program (nothing to
rem install, no windres) that writes rsrc_windows_amd64.syso from app.ico. The
rem Go linker embeds any *_windows_amd64.syso it finds. A copy is already in the
rem project, so this step only matters after changing app.ico.
go run ./tools/mkrsrc -ico app.ico -arch amd64 -o rsrc_windows_amd64.syso
if errorlevel 1 exit /b 1

set "GOOS=windows"
set "GOARCH=amd64"

go test ./...
if errorlevel 1 exit /b 1

rem -H=windowsgui: GUI subsystem, no console window.
go build -trimpath -ldflags "-s -w -H=windowsgui" -o layout-tray.exe .
if errorlevel 1 exit /b 1
echo Built layout-tray.exe
