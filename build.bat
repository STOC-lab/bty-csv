@echo off
REM Windows 向けビルド（GUI付き単一EXE）
setlocal

REM マニフェスト埋め込みツール。Common Controls 6.0 と DPI 対応に必須。
where rsrc >nul 2>nul
if errorlevel 1 (
  echo rsrc をインストールします...
  go install github.com/akavel/rsrc@latest || exit /b 1
)

rsrc -manifest app.manifest -arch amd64 -o rsrc_windows_amd64.syso || exit /b 1

set CGO_ENABLED=0
go build -trimpath -ldflags="-s -w -H=windowsgui" -o bty_csv.exe . || exit /b 1

echo.
echo built: bty_csv.exe
dir bty_csv.exe
