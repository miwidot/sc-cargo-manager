// SC Cargo Manager
//
// Eigenstaendiges Desktop-Programm (Go + WebView2), OHNE lokalen Server / Port.
// Die HTML-UI wird in-process im WebView2-Fenster gerendert. Speichern/Laden
// laeuft ueber direkte Go<->JS Funktions-Bindings (kein HTTP, kein Socket ->
// keine Firewall-Abfrage). Die citizenhq.space API wird per fetch direkt
// abgefragt (CORS = *), also kein Proxy noetig.
//
// Persistenz: JSON-Datei unter %APPDATA%\sc-cargo-manager\data.json
package main

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	webview "github.com/jchv/go-webview2"
)

//go:embed web/index.html
var indexHTML string

//go:embed web/hero-asteroids.webp
var heroBG []byte

// pageHTML setzt das eingebettete Hintergrundbild als Data-URI in die UI ein.
func pageHTML() string {
	dataURI := "data:image/webp;base64," + base64.StdEncoding.EncodeToString(heroBG)
	return strings.Replace(indexHTML, "__HERO_BG__", dataURI, 1)
}

// ---------------------------------------------------------------------------
// Datenmodell + JSON-Persistenz
// ---------------------------------------------------------------------------

// Entry ist ein einzelner Transport-Log-Eintrag.
type Entry struct {
	ID            int64   `json:"id"`
	When          string  `json:"when"`          // RFC3339
	Location      string  `json:"location"`      // Ort / Station wo gekauft/geladen
	CommodityID   int     `json:"commodityId"`   // citizenhq commodity id (fuer best-buyer)
	CommodityName string  `json:"commodityName"` // Anzeigename
	Units         float64 `json:"units"`         // Menge in SCU
	BuyPerUnit    float64 `json:"buyPerUnit"`    // Einkaufspreis pro SCU (aUEC)
	Paid          float64 `json:"paid"`          // gesamt bezahlt (aUEC)
	SellTarget    string  `json:"sellTarget"`    // gewaehltes Verkaufsziel (Terminal)
	SellSystem    string  `json:"sellSystem"`    // System des Verkaufsziels
	Ship          string  `json:"ship"`          // Transportschiff (Name)
	ShipSCU       float64 `json:"shipSCU"`       // Ladekapazitaet des Schiffs (SCU)
}

// store haelt alle Eintraege im Speicher und persistiert sie als JSON-Datei.
type store struct {
	mu     sync.Mutex
	path   string
	nextID int64
	items  []Entry
}

func newStore(path string) (*store, error) {
	s := &store{path: path, nextID: 1}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &s.items); err != nil {
			return nil, fmt.Errorf("data.json beschaedigt: %w", err)
		}
	}
	for _, e := range s.items {
		if e.ID >= s.nextID {
			s.nextID = e.ID + 1
		}
	}
	return s, nil
}

// saveLocked schreibt den aktuellen Stand atomar auf die Platte.
// Caller muss s.mu halten.
func (s *store) saveLocked() error {
	data, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *store) list() []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, len(s.items))
	copy(out, s.items)
	sort.Slice(out, func(i, j int) bool { return out[i].When > out[j].When }) // neueste zuerst
	if out == nil {
		out = []Entry{}
	}
	return out
}

func (s *store) add(e Entry) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.ID = s.nextID
	s.nextID++
	if e.When == "" {
		e.When = time.Now().Format(time.RFC3339)
	}
	s.items = append(s.items, e)
	return e, s.saveLocked()
}

func (s *store) setTarget(id int64, target, system string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == id {
			s.items[i].SellTarget = target
			s.items[i].SellSystem = system
			return s.items[i], s.saveLocked()
		}
	}
	return Entry{}, errors.New("nicht gefunden")
}

func (s *store) delete(id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, e := range s.items {
		if e.ID == id {
			s.items = slices.Delete(s.items, i, i+1)
			return s.saveLocked()
		}
	}
	return errors.New("nicht gefunden")
}

// ---------------------------------------------------------------------------
// Einstellungen (globaler Standard, z.B. Default-Schiff)
// ---------------------------------------------------------------------------

// Settings haelt globale Voreinstellungen, die nicht pro Eintrag gelten.
type Settings struct {
	DefaultShip    string  `json:"defaultShip"`
	DefaultShipSCU float64 `json:"defaultShipSCU"`
}

type settingsStore struct {
	mu   sync.Mutex
	path string
	data Settings
}

func newSettingsStore(path string) *settingsStore {
	ss := &settingsStore{path: path}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &ss.data)
	}
	return ss
}

func (ss *settingsStore) get() Settings {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.data
}

func (ss *settingsStore) setShip(name string, scu float64) (Settings, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.data.DefaultShip = name
	ss.data.DefaultShipSCU = scu
	b, err := json.MarshalIndent(ss.data, "", "  ")
	if err != nil {
		return ss.data, err
	}
	tmp := ss.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return ss.data, err
	}
	return ss.data, os.Rename(tmp, ss.path)
}

// ---------------------------------------------------------------------------
// Speicherort
// ---------------------------------------------------------------------------

func dataPath() (string, error) {
	base, err := os.UserConfigDir() // %APPDATA% auf Windows
	if err != nil {
		base = "."
	}
	dir := filepath.Join(base, "sc-cargo-manager")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}
	newPath := filepath.Join(dir, "data.json")

	// Einmalige Migration vom alten Ordner (sc-transport-log), damit
	// bereits gespeicherte Eintraege nach der Umbenennung erhalten bleiben.
	if _, err := os.Stat(newPath); os.IsNotExist(err) {
		oldPath := filepath.Join(base, "sc-transport-log", "data.json")
		if data, e := os.ReadFile(oldPath); e == nil {
			_ = os.WriteFile(newPath, data, 0644)
		}
	}
	return newPath, nil
}

// ---------------------------------------------------------------------------
// main — WebView2 Fenster, keine Netzwerk-Server
// ---------------------------------------------------------------------------

func main() {
	// WebView2 muss auf dem Main-OS-Thread laufen.
	runtime.LockOSThread()

	cleanupOldUpdate()  // Reste vom letzten Auto-Update entfernen
	selfHealAutostart() // Autostart-Pfad auf aktuelle exe aktualisieren (falls aktiv)

	dp, err := dataPath()
	if err != nil {
		fatal("Speicherort nicht verfuegbar: " + err.Error())
	}
	st, err := newStore(dp)
	if err != nil {
		fatal("Konnte Daten nicht laden: " + err.Error())
	}
	set := newSettingsStore(filepath.Join(filepath.Dir(dp), "settings.json"))

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

	// --- Rahmenloses Fenster + eigene Titelleiste ---
	hwnd := hwndOf(w.Window())
	makeFrameless(hwnd)
	customFrame(hwnd) // WM_NCCALCSIZE: oberen Rahmen-Inset weg -> kein weisser Streifen
	must(w.Bind("goWinDrag", func() error { w.Dispatch(func() { winDrag(hwnd) }); return nil }))
	must(w.Bind("goWinMin", func() error { winMinimize(hwnd); return nil }))
	must(w.Bind("goWinMax", func() error { winToggleMax(hwnd); return nil }))
	must(w.Bind("goWinClose", func() error { winClose(hwnd); return nil }))
	must(w.Bind("goOpenExternal", func(url string) error { openExternal(url); return nil }))

	// --- Go-Funktionen fuer JavaScript verfuegbar machen (kein HTTP) ---

	// goLoadEntries() -> Entry[]
	must(w.Bind("goLoadEntries", func() ([]Entry, error) {
		return st.list(), nil
	}))

	// goAddEntry(entry) -> Entry (mit vergebener id)
	must(w.Bind("goAddEntry", func(e Entry) (Entry, error) {
		return st.add(e)
	}))

	// goDeleteEntry(id) -> bool
	must(w.Bind("goDeleteEntry", func(id int64) (bool, error) {
		if err := st.delete(id); err != nil {
			return false, err
		}
		return true, nil
	}))

	// goGetSettings() -> Settings (globale Voreinstellungen laden)
	must(w.Bind("goGetSettings", func() (Settings, error) {
		return set.get(), nil
	}))

	// goSetShip(name, scu) -> Settings (Default-Schiff speichern)
	must(w.Bind("goSetShip", func(name string, scu float64) (Settings, error) {
		return set.setShip(name, scu)
	}))

	// goSetTarget(id, terminal, system) -> Entry (gewaehltes Verkaufsziel speichern)
	must(w.Bind("goSetTarget", func(id int64, target, system string) (Entry, error) {
		return st.setTarget(id, target, system)
	}))

	// goDataPath() -> string (Anzeige wo die Datei liegt)
	must(w.Bind("goDataPath", func() (string, error) {
		return dp, nil
	}))

	// --- Einstellungen: Windows-Autostart ---
	must(w.Bind("goGetAutostart", func() (bool, error) { return autostartEnabled(), nil }))
	must(w.Bind("goSetAutostart", func(enable bool) (bool, error) {
		if err := setAutostart(enable); err != nil {
			return false, err
		}
		return autostartEnabled(), nil
	}))

	// --- Auto-Update ---
	must(w.Bind("goVersion", func() (string, error) { return version, nil }))
	must(w.Bind("goCheckUpdate", func() (UpdateInfo, error) { return checkUpdate() }))
	must(w.Bind("goApplyUpdate", func(url string) (bool, error) {
		if err := applyUpdate(url); err != nil {
			return false, err
		}
		return true, nil
	}))

	w.SetHtml(pageHTML())
	w.Run()
}

func must(err error) {
	if err != nil {
		fatal("Bind-Fehler: " + err.Error())
	}
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
