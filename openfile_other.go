//go:build !windows

package main

func openFile(path string) {}
func attachConsole()       {}

// RunGUI は Windows 以外ではGUIを提供しない。
func RunGUI(cfg *Config, workDir string) error {
	return errNoGUI
}
