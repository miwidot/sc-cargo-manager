//go:build linux

// Linux-Plattformhelfer: externen Browser via xdg-open, Ton als No-Op.
package main

import "os/exec"

// openExternal oeffnet eine URL im Standard-Browser (xdg-open).
func openExternal(url string) {
	_ = exec.Command("xdg-open", url).Start()
}

// beep: unter Linux kein Systemton (No-Op).
func beep() {}
