// Rahmenloses Fenster + eigene Titelleiste: entfernt die native Windows-Titelleiste
// (WS_CAPTION) und stellt Funktionen fuer Ziehen/Minimieren/Maximieren/Schliessen
// bereit, die aus der HTML-Titelleiste heraus aufgerufen werden.
package main

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	pGetWindowLongPtr = user32.NewProc("GetWindowLongPtrW")
	pSetWindowLongPtr = user32.NewProc("SetWindowLongPtrW")
	pSetWindowPos     = user32.NewProc("SetWindowPos")
	pReleaseCapture   = user32.NewProc("ReleaseCapture")
	pSendMessage      = user32.NewProc("SendMessageW")
	pShowWindow       = user32.NewProc("ShowWindow")
	pIsZoomed         = user32.NewProc("IsZoomed")
	pPostMessage      = user32.NewProc("PostMessageW")
)

const (
	gwlStyle        = ^uintptr(15) // GWL_STYLE = -16
	wsCaption       = 0x00C00000
	swpFrameChanged = 0x0020
	swpNoMove       = 0x0002
	swpNoSize       = 0x0001
	swpNoZOrder     = 0x0004
	wmNCLButtonDown = 0x00A1
	htCaption       = 2
	wmClose         = 0x0010
	swMinimize      = 6
	swMaximize      = 3
	swRestore       = 9
)

func hwndOf(win unsafe.Pointer) uintptr { return uintptr(win) }

// makeFrameless entfernt die native Titelleiste (Rahmen zum Resizen bleibt erhalten).
func makeFrameless(hwnd uintptr) {
	style, _, _ := pGetWindowLongPtr.Call(hwnd, gwlStyle)
	style &^= wsCaption
	pSetWindowLongPtr.Call(hwnd, gwlStyle, style)
	pSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0, swpFrameChanged|swpNoMove|swpNoSize|swpNoZOrder)
}

// winDrag startet das System-Verschieben (wie Ziehen an der Titelleiste, inkl. Aero-Snap).
func winDrag(hwnd uintptr) {
	pReleaseCapture.Call()
	pSendMessage.Call(hwnd, wmNCLButtonDown, htCaption, 0)
}

func winMinimize(hwnd uintptr) { pShowWindow.Call(hwnd, swMinimize) }

func winToggleMax(hwnd uintptr) {
	if z, _, _ := pIsZoomed.Call(hwnd); z != 0 {
		pShowWindow.Call(hwnd, swRestore)
	} else {
		pShowWindow.Call(hwnd, swMaximize)
	}
}

func winClose(hwnd uintptr) { pPostMessage.Call(hwnd, wmClose, 0, 0) }
