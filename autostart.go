// Windows-Autostart (sicher, ohne Admin): setzt/entfernt einen Wert im per-User
// HKCU Run-Key. Betrifft nur den aktuellen Benutzer, keine erhoehten Rechte noetig.
package main

import (
	"errors"
	"os"

	"golang.org/x/sys/windows/registry"
)

const (
	runKeyPath   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValueName = "SC Cargo Manager"
)

// autostartEnabled prueft, ob der Autostart-Eintrag existiert.
func autostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(runValueName)
	return err == nil
}

// setAutostart aktiviert/deaktiviert den Autostart fuer den aktuellen User.
func setAutostart(enable bool) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if enable {
		exe, err := os.Executable()
		if err != nil {
			return err
		}
		return k.SetStringValue(runValueName, `"`+exe+`"`) // gequotet wegen Leerzeichen im Pfad
	}
	if err := k.DeleteValue(runValueName); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

// selfHealAutostart aktualisiert bei aktivem Autostart den Pfad auf die aktuelle
// exe (falls das Programm verschoben wurde). Beim Start aufrufen.
func selfHealAutostart() {
	if autostartEnabled() {
		_ = setAutostart(true)
	}
}
