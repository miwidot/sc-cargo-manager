// Plattformneutrale Go<->JS-Bindings. Beide Webview-Backends (go-webview2 auf
// Windows, webview_go auf Linux/macOS) erfuellen das binder-Interface, sodass die
// eigentliche App-Logik nur einmal existiert. Fenster-/Titelleisten-Bindings sind
// plattformspezifisch und liegen in ui_windows.go bzw. ui_linux.go.
package main

// binder ist die gemeinsame Teilmenge beider Webview-Typen.
type binder interface {
	Bind(name string, f interface{}) error
}

// bindCore registriert alle plattformunabhaengigen Funktionen am Webview.
// openExternal(), beep(), autostartEnabled()/setAutostart() sind per Build-Tag
// je Plattform implementiert.
func bindCore(w binder, st *store, set *settingsStore, dataFile, cacheDir string) {
	must(w.Bind("goOpenExternal", func(url string) error { openExternal(url); return nil }))
	must(w.Bind("goBeep", func() error { beep(); return nil }))

	// API-Cache: Stammdaten auf Platte (Sofort-Start + offline)
	must(w.Bind("goCacheGet", func(key string) (string, error) { return cacheGet(cacheDir, key), nil }))
	must(w.Bind("goCacheSet", func(key, data string) (bool, error) {
		if err := cacheSet(cacheDir, key, data); err != nil {
			return false, err
		}
		return true, nil
	}))
	must(w.Bind("goSetLean", func(on bool) (Settings, error) { return set.setLean(on) }))

	// Log-Eintraege
	must(w.Bind("goLoadEntries", func() ([]Entry, error) { return st.list(), nil }))
	must(w.Bind("goAddEntry", func(e Entry) (Entry, error) { return st.add(e) }))
	must(w.Bind("goDeleteEntry", func(id int64) (bool, error) {
		if err := st.delete(id); err != nil {
			return false, err
		}
		return true, nil
	}))
	must(w.Bind("goSetTarget", func(id int64, target, system string) (Entry, error) { return st.setTarget(id, target, system) }))
	must(w.Bind("goMarkSold", func(id int64, total float64, when string) (Entry, error) { return st.markSold(id, total, when) }))
	must(w.Bind("goMarkUnsold", func(id int64) (Entry, error) { return st.markUnsold(id) }))
	must(w.Bind("goSetAlert", func(id int64, price float64) (Entry, error) { return st.setAlert(id, price) }))
	must(w.Bind("goSetLocation", func(id int64, location string) (Entry, error) { return st.setLocation(id, location) }))
	must(w.Bind("goMergeEntry", func(id int64, addUnits, addPaid float64) (Entry, error) { return st.mergeEntry(id, addUnits, addPaid) }))

	// Einstellungen
	must(w.Bind("goGetSettings", func() (Settings, error) { return set.get(), nil }))
	must(w.Bind("goSetShip", func(name string, scu float64) (Settings, error) { return set.setShip(name, scu) }))
	must(w.Bind("goDataPath", func() (string, error) { return dataFile, nil }))
	must(w.Bind("goGetAutostart", func() (bool, error) { return autostartEnabled(), nil }))
	must(w.Bind("goSetAutostart", func(enable bool) (bool, error) {
		if err := setAutostart(enable); err != nil {
			return false, err
		}
		return autostartEnabled(), nil
	}))

	// Auto-Update
	must(w.Bind("goVersion", func() (string, error) { return version, nil }))
	must(w.Bind("goCheckUpdate", func() (UpdateInfo, error) { return checkUpdate() }))
	must(w.Bind("goApplyUpdate", func(url string) (bool, error) {
		if err := applyUpdate(url); err != nil {
			return false, err
		}
		return true, nil
	}))
}
