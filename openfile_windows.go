//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

var (
	shell32     = syscall.NewLazyDLL("shell32.dll")
	shellExecW  = shell32.NewProc("ShellExecuteW")
	kernel32    = syscall.NewLazyDLL("kernel32.dll")
	attachCons  = kernel32.NewProc("AttachConsole")
	allocCons   = kernel32.NewProc("AllocConsole")
)

func openFile(path string) {
	op, _ := syscall.UTF16PtrFromString("open")
	p, _ := syscall.UTF16PtrFromString(path)
	shellExecW.Call(0, uintptr(unsafe.Pointer(op)), uintptr(unsafe.Pointer(p)), 0, 0, 1)
}

// attachConsole は -H windowsgui でビルドした EXE を
// コマンドラインから起動したときに親コンソールへ出力を戻す。
func attachConsole() {
	const attachParentProcess = ^uintptr(0) // (DWORD)-1
	r, _, _ := attachCons.Call(attachParentProcess)
	if r == 0 {
		if r2, _, _ := allocCons.Call(); r2 == 0 {
			return
		}
	}
	reopen("CONOUT$", syscall.Stdout, &stdoutFile)
	reopen("CONOUT$", syscall.Stderr, &stderrFile)
}
