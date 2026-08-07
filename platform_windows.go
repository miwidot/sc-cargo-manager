//go:build windows

// Windows-Plattformhelfer: externen Browser oeffnen, Benachrichtigungston.
package main

import "os/exec"

// openExternal oeffnet eine URL im Standard-Browser (nicht im WebView).
func openExternal(url string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

// beep spielt den System-Benachrichtigungston (MessageBeep, siehe win.go).
func beep() { messageBeep() }
