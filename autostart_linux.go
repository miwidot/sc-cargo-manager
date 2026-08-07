//go:build linux

// Linux-Autostart (ohne root): legt eine .desktop-Datei unter
// ~/.config/autostart/ an bzw. entfernt sie. Betrifft nur den aktuellen Benutzer.
package main

import (
	"os"
	"path/filepath"
)

func autostartFile() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "autostart", "sc-cargo-manager.desktop")
}

// autostartEnabled prueft, ob die Autostart-.desktop-Datei existiert.
func autostartEnabled() bool {
	_, err := os.Stat(autostartFile())
	return err == nil
}

// setAutostart legt die .desktop-Datei an (mit aktuellem exe-Pfad) oder entfernt sie.
func setAutostart(enable bool) error {
	f := autostartFile()
	if !enable {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
		return err
	}
	content := "[Desktop Entry]\n" +
		"Type=Application\n" +
		"Name=SC Cargo Manager\n" +
		"Exec=" + exe + "\n" +
		"X-GNOME-Autostart-enabled=true\n"
	return os.WriteFile(f, []byte(content), 0644)
}

// selfHealAutostart aktualisiert bei aktivem Autostart den exe-Pfad (falls verschoben).
func selfHealAutostart() {
	if autostartEnabled() {
		_ = setAutostart(true)
	}
}
