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
)

//go:embed web/index.html
var indexHTML string

//go:embed web/hero-guardian.webp
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
	Kind          string  `json:"kind"`          // "" / "cargo" = Handelsware, "ore" = Roherz (Mining)
	When          string  `json:"when"`          // RFC3339
	Location      string  `json:"location"`      // Ort / Station wo gekauft/geladen
	CommodityID   int     `json:"commodityId"`   // citizenhq commodity id (fuer best-buyer)
	CommodityName string  `json:"commodityName"` // Anzeigename
	Units         float64 `json:"units"`         // Menge in SCU (= qtyCount * qtySize)
	QtyCount      float64 `json:"qtyCount"`      // Anzahl Container (fuer Rechnungs-Tooltip)
	QtySize       float64 `json:"qtySize"`       // Container-Groesse in SCU
	BuyPerUnit    float64 `json:"buyPerUnit"`    // Einkaufspreis pro SCU (aUEC)
	Paid          float64 `json:"paid"`          // gesamt bezahlt (aUEC)
	SellTarget    string  `json:"sellTarget"`    // gewaehltes Verkaufsziel (Terminal)
	SellSystem    string  `json:"sellSystem"`    // System des Verkaufsziels
	Ship          string  `json:"ship"`          // Transportschiff (Name)
	ShipSCU       float64 `json:"shipSCU"`       // Ladekapazitaet des Schiffs (SCU)
	Sold          bool    `json:"sold"`          // verkauft?
	SoldTotal     float64 `json:"soldTotal"`     // tatsaechlicher Erloes (aUEC)
	SoldWhen      string  `json:"soldWhen"`      // Verkaufszeit (RFC3339)
	AlertPrice    float64 `json:"alertPrice"`    // Alarm ausloesen wenn Bestpreis-Erloes >= diesem Wert (0 = aus)
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

func (s *store) markSold(id int64, total float64, when string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == id {
			s.items[i].Sold = true
			s.items[i].SoldTotal = total
			s.items[i].SoldWhen = when
			return s.items[i], s.saveLocked()
		}
	}
	return Entry{}, errors.New("nicht gefunden")
}

// mergeEntry addiert Menge + Bezahlt auf einen bestehenden Eintrag (gleiche Ladung
// an gleicher Location nachgeladen). Box-Aufschluesselung wird geleert (jetzt gemischt).
func (s *store) mergeEntry(id int64, addUnits, addPaid float64) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == id {
			s.items[i].Units += addUnits
			s.items[i].Paid += addPaid
			s.items[i].QtyCount = 0
			s.items[i].QtySize = 0
			return s.items[i], s.saveLocked()
		}
	}
	return Entry{}, errors.New("nicht gefunden")
}

func (s *store) setLocation(id int64, location string) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == id {
			s.items[i].Location = location
			return s.items[i], s.saveLocked()
		}
	}
	return Entry{}, errors.New("nicht gefunden")
}

func (s *store) setAlert(id int64, price float64) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == id {
			s.items[i].AlertPrice = price
			return s.items[i], s.saveLocked()
		}
	}
	return Entry{}, errors.New("nicht gefunden")
}

func (s *store) markUnsold(id int64) (Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == id {
			s.items[i].Sold = false
			s.items[i].SoldTotal = 0
			s.items[i].SoldWhen = ""
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
	LeanMode       bool    `json:"leanMode"` // WebView2 schlank starten (weniger RAM), Neustart noetig
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

func (ss *settingsStore) saveLocked() (Settings, error) {
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

func (ss *settingsStore) setShip(name string, scu float64) (Settings, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.data.DefaultShip = name
	ss.data.DefaultShipSCU = scu
	return ss.saveLocked()
}

func (ss *settingsStore) setLean(on bool) (Settings, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.data.LeanMode = on
	return ss.saveLocked()
}

// ---------------------------------------------------------------------------
// API-Cache (Stammdaten auf Platte: Sofort-Start + offline-faehig)
// ---------------------------------------------------------------------------

// cacheKeyOK laesst nur harmlose Dateinamen zu (a-z0-9-_).
func cacheKeyOK(k string) bool {
	if k == "" || len(k) > 64 {
		return false
	}
	for _, r := range k {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

func cacheGet(dir, key string) string {
	if !cacheKeyOK(key) {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(dir, key+".json"))
	if err != nil {
		return ""
	}
	return string(b)
}

func cacheSet(dir, key, data string) error {
	if !cacheKeyOK(key) {
		return errors.New("ungueltiger Cache-Key")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp := filepath.Join(dir, key+".json.tmp")
	if err := os.WriteFile(tmp, []byte(data), 0644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, key+".json"))
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
	cacheDir := filepath.Join(filepath.Dir(dp), "cache")

	// Ab hier plattformspezifisch: Fenster erzeugen, Bindings setzen, laufen lassen.
	runUI(st, set, dp, cacheDir)
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
