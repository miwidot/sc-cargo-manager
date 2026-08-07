//go:build linux

// Linux-UI: webview_go (WebKitGTK). Natives Fenster mit normaler Fensterdeko —
// die eigene HTML-Titelleiste (reines Win32) wird per goPlatform ausgeblendet.
package main

import webview "github.com/webview/webview_go"

func runUI(st *store, set *settingsStore, dataFile, cacheDir string) {
	w := webview.New(false)
	if w == nil {
		fatal("Webview konnte nicht gestartet werden.\n" +
			"Bitte WebKitGTK installieren (z.B. 'webkit2gtk' + 'gtk3').")
	}
	defer w.Destroy()
	w.SetTitle("SC Cargo Manager v" + version)
	w.SetSize(1180, 820, webview.HintNone)

	// Linux nutzt native Fensterdeko -> Titelleisten-Bindings als No-Op.
	must(w.Bind("goPlatform", func() (string, error) { return "linux", nil }))
	must(w.Bind("goWinDrag", func() error { return nil }))
	must(w.Bind("goWinMin", func() error { return nil }))
	must(w.Bind("goWinMax", func() error { return nil }))
	must(w.Bind("goWinClose", func() error { w.Terminate(); return nil }))
	must(w.Bind("goFlash", func() error { return nil }))

	bindCore(w, st, set, dataFile, cacheDir)

	w.SetHtml(pageHTML())
	w.Run()
}
