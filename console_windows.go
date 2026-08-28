//go:build windows

package main

import (
	"os"
	"syscall"
)

var stdoutFile, stderrFile *os.File

func reopen(name string, h syscall.Handle, target **os.File) {
	f, err := os.OpenFile(name, os.O_RDWR, 0)
	if err != nil {
		return
	}
	*target = f
	switch h {
	case syscall.Stdout:
		os.Stdout = f
	case syscall.Stderr:
		os.Stderr = f
	}
}
