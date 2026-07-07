// Rahmenloses Fenster + eigene Titelleiste: entfernt die native Windows-Titelleiste
// (WS_CAPTION) und stellt Funktionen fuer Ziehen/Minimieren/Maximieren/Schliessen
// bereit, die aus der HTML-Titelleiste heraus aufgerufen werden.
package main

import (
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

// openExternal oeffnet eine URL im Standard-Browser (nicht im WebView).
func openExternal(url string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

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
	pCallWindowProc   = user32.NewProc("CallWindowProcW")
	pFlashWindowEx    = user32.NewProc("FlashWindowEx")
	pMessageBeep      = user32.NewProc("MessageBeep")
)

type flashwinfo struct {
	cbSize    uint32
	hwnd      uintptr
	dwFlags   uint32
	uCount    uint32
	dwTimeout uint32
}

// flashWindow laesst den Taskbar-Button blinken bis das Fenster fokussiert wird.
func flashWindow(hwnd uintptr) {
	fw := flashwinfo{hwnd: hwnd, dwFlags: 0x0000000E} // FLASHW_TRAY | FLASHW_TIMERNOFG
	fw.cbSize = uint32(unsafe.Sizeof(fw))
	pFlashWindowEx.Call(uintptr(unsafe.Pointer(&fw)))
}

// messageBeep spielt den Standard-Benachrichtigungston.
func messageBeep() { pMessageBeep.Call(0x00000040) } // MB_ICONASTERISK

const (
	gwlStyle        = ^uintptr(15) // GWL_STYLE = -16
	gwlpWndProc     = ^uintptr(3)  // GWLP_WNDPROC = -4
	wsCaption       = 0x00C00000
	swpFrameChanged = 0x0020
	swpNoMove       = 0x0002
	swpNoSize       = 0x0001
	swpNoZOrder     = 0x0004
	wmNCLButtonDown = 0x00A1
	wmNCCalcSize    = 0x0083
	htCaption       = 2
	wmClose         = 0x0010
	swMinimize      = 6
	swMaximize      = 3
	swRestore       = 9
)

// Referenz auf den originalen WndProc (fuer CallWindowProc im Subclass).
var origWndProc uintptr

func hwndOf(win unsafe.Pointer) uintptr { return uintptr(win) }

// makeFrameless entfernt die native Titelleiste (Rahmen zum Resizen bleibt erhalten).
func makeFrameless(hwnd uintptr) {
	style, _, _ := pGetWindowLongPtr.Call(hwnd, gwlStyle)
	style &^= wsCaption
	pSetWindowLongPtr.Call(hwnd, gwlStyle, style)
	pSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0, swpFrameChanged|swpNoMove|swpNoSize|swpNoZOrder)
}

// customFrame subclasst den WndProc und faengt WM_NCCALCSIZE ab: der obere
// Rahmen-Inset wird entfernt (Client reicht bis zur Fenster-Oberkante) -> kein
// weisser 1px-Streifen mehr. Seiten/unten behalten den Rahmen zum Resizen.
// Bei maximiertem Fenster bleibt der Standard-Inset (sonst Clipping).
func customFrame(hwnd uintptr) {
	cb := windows.NewCallback(func(hw, msg, wp, lp uintptr) uintptr {
		if msg == wmNCCalcSize && wp != 0 {
			top := *(*int32)(unsafe.Pointer(lp + 4)) // rgrc[0].top (RECT: left@0, top@4)
			ret, _, _ := pCallWindowProc.Call(origWndProc, hw, msg, wp, lp)
			if z, _, _ := pIsZoomed.Call(hw); z == 0 {
				*(*int32)(unsafe.Pointer(lp + 4)) = top // Client bis Oberkante -> kein Top-Rahmen
			}
			return ret
		}
		ret, _, _ := pCallWindowProc.Call(origWndProc, hw, msg, wp, lp)
		return ret
	})
	origWndProc, _, _ = pSetWindowLongPtr.Call(hwnd, gwlpWndProc, cb)
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
