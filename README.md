# SC Cargo Manager

Eigenständiges Windows-Desktop-Programm für Star-Citizen-Cargo/Hauling — ersetzt Excel.
Ladungen erfassen, live den **besten Verkaufsplatz** finden, **Profit-Routen** berechnen
und dein Standard-Schiff samt **Überlade-Warnung** verwalten. Daten von
[citizenhq.space](https://citizenhq.space).

## Download & Start

**Windows:** Neueste `sc-cargo-manager.exe` unter **[Releases](https://github.com/miwidot/sc-cargo-manager/releases)**
laden und starten. Kein Installer, kein Server, kein offener Port → keine Firewall-Abfrage.
Das Programm **aktualisiert sich selbst** (fragt beim Start nach neuen Versionen).

**Linux:** `sc-cargo-manager-linux` aus den Releases laden. Benötigt WebKitGTK 4.1 + GTK3
(Arch: `sudo pacman -S webkit2gtk-4.1 gtk3` · Ubuntu/Debian: `sudo apt install libwebkit2gtk-4.1-0 libgtk-3-0`),
dann `chmod +x sc-cargo-manager-linux && ./sc-cargo-manager-linux`. Auf Linux mit nativer
Fensterdeko (die eigene Titelleiste ist Windows-spezifisch); Funktionen identisch.

Aus Quellcode: Windows → Doppelklick auf **`start.bat`**; Linux → gegen 4.1 bauen:
```
PKG_CONFIG_PATH="/tmp/pc:$(pkg-config --variable pc_path pkg-config)"
printf 'Name: webkit2gtk-4.0\nVersion: 2.44\nRequires: webkit2gtk-4.1\n' > /tmp/pc/webkit2gtk-4.0.pc
CGO_ENABLED=1 go build .
```
(benötigt [Go](https://go.dev/dl) + `webkit2gtk-4.1`/`gtk3`-Dev-Pakete.)

## Funktionen

- **Cargo-Log** — Einträge (Location, Material, Menge = Anzahl × Container, Datum, Einkauf),
  Gesamt-Statistik (SCU, Materialien, Fuhren), Ziel-Auswahl je Eintrag mit System-Filter.
- **Live bester Verkaufsplatz** — Top-Abnehmer mit Preis, Nachfrage, Bestand, Datenaktualität.
- **Routen** — Profit-Routen (günstig kaufen → teuer verkaufen), Rundtrip mit Rückroute,
  Kauf-Empfehlung anhand der Schiffskapazität, Filter für Datenaktualität.
- **Standard-Schiff** — global gesetzt, treibt Kapazität & Überlade-Warnung.
- **Einstellungen (⚙)** — Windows-Autostart (per-User, ohne Admin), Update-Prüfung.
- **Auto-Update** — lädt neue Releases selbst und startet neu.
- Winziger RAM-Verbrauch (~25 MB), natives WebView2-Fenster (auf Windows 11 vorhanden).

## Speicherung

JSON-Datei unter `%APPDATA%\sc-cargo-manager\` (`data.json`, `settings.json`).
Menschenlesbar, einfach zu sichern.

## Technik

- **Go** (Standardbibliothek) + [`jchv/go-webview2`](https://github.com/jchv/go-webview2) (reines Go).
- UI: eingebettete `web/index.html` (Vanilla JS). API direkt per `fetch` (CORS erlaubt).
- Autostart via HKCU Run-Key (`golang.org/x/sys/windows/registry`).
- Icon eingebettet über `resource.syso`.

## Neue Version veröffentlichen (für Maintainer)

1. `version` in `update.go` erhöhen (z.B. `1.0.1`).
2. `build.bat` ausführen → baut **und signiert** `sc-cargo-manager.exe`
   (Code-Signing via `sign.bat`, Zertifikat aus dem Windows-User-Store, Certum-Timestamp).
3. Tag + Release mit der Exe:
   ```
   git commit -am "v1.0.1"
   git tag v1.0.1 && git push --tags
   gh release create v1.0.1 sc-cargo-manager.exe --title "v1.0.1" --notes "…"
   ```
   Bestehende Installationen erkennen das Update automatisch beim nächsten Start.
