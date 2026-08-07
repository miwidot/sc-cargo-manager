//go:build windows

// Windows-UI: go-webview2 (Microsoft Edge WebView2), rahmenloses Fenster mit
// eigener HTML-Titelleiste und optionalem RAM-Sparmodus (Ein-Prozess).
package main

import (
	"os"

	webview "github.com/jchv/go-webview2"
)

func runUI(st *store, set *settingsStore, dataFile, cacheDir string) {
	// Schlanker Modus (per Einstellung): WebView2 als Ein-Prozess ohne GPU-Prozess,
	// V8-Heap gedeckelt. Spart deutlich RAM (Neustart noetig). Per Env ueberschreibbar.
	if set.get().LeanMode && os.Getenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS") == "" {
		os.Setenv("WEBVIEW2_ADDITIONAL_BROWSER_ARGUMENTS",
			"--single-process --disable-gpu --disable-software-rasterizer --js-flags=--max-old-space-size=128")
	}

	w := webview.NewWithOptions(webview.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview.WindowOptions{
			Title:  "SC Cargo Manager v" + version,
			Width:  1180,
			Height: 820,
			Center: true,
		},
	})
	if w == nil {
		fatal("WebView2 konnte nicht gestartet werden.\n" +
			"Bitte die 'Microsoft Edge WebView2 Runtime' installieren:\n" +
			"https://developer.microsoft.com/microsoft-edge/webview2/")
	}
	defer w.Destroy()

	// Rahmenloses Fenster + eigene Titelleiste
	hwnd := hwndOf(w.Window())
	makeFrameless(hwnd)
	customFrame(hwnd) // WM_NCCALCSIZE: oberen Rahmen-Inset weg -> kein weisser Streifen
	must(w.Bind("goPlatform", func() (string, error) { return "windows", nil }))
	must(w.Bind("goWinDrag", func() error { w.Dispatch(func() { winDrag(hwnd) }); return nil }))
	must(w.Bind("goWinMin", func() error { winMinimize(hwnd); return nil }))
	must(w.Bind("goWinMax", func() error { winToggleMax(hwnd); return nil }))
	must(w.Bind("goWinClose", func() error { winClose(hwnd); return nil }))
	must(w.Bind("goFlash", func() error { flashWindow(hwnd); return nil }))

	bindCore(w, st, set, dataFile, cacheDir)

	w.SetHtml(pageHTML())
	w.Run()
}
