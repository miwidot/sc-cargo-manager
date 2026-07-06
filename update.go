// Auto-Update: prueft GitHub Releases der Org miwidot, laedt bei Bedarf die neue
// .exe herunter, tauscht die laufende Datei aus (unter Windows erlaubt) und startet neu.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// version ist die aktuelle Programmversion (bei Release erhoehen + Tag vX.Y.Z pushen).
const version = "1.0.7"

const releaseAPI = "https://api.github.com/repos/miwidot/sc-cargo-manager/releases/latest"

// UpdateInfo wird ans Frontend geliefert.
type UpdateInfo struct {
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	Available   bool   `json:"available"`
	DownloadURL string `json:"downloadUrl"`
	Notes       string `json:"notes"`
	HTMLURL     string `json:"htmlUrl"`
}

type ghRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// checkUpdate fragt das neueste Release ab und vergleicht die Version.
func checkUpdate() (UpdateInfo, error) {
	info := UpdateInfo{Current: version}
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", releaseAPI, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "sc-cargo-manager") // GitHub API verlangt einen User-Agent (sonst 403)
	resp, err := client.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return info, fmt.Errorf("github %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return info, err
	}
	info.Latest = strings.TrimPrefix(rel.TagName, "v")
	info.HTMLURL = rel.HTMLURL
	info.Notes = rel.Body
	for _, a := range rel.Assets {
		if strings.HasSuffix(strings.ToLower(a.Name), ".exe") {
			info.DownloadURL = a.BrowserDownloadURL
			break
		}
	}
	info.Available = info.DownloadURL != "" && semverNewer(info.Latest, version)
	return info, nil
}

// semverNewer: ist a (x.y.z) neuer als b?
func semverNewer(a, b string) bool {
	pa, pb := parseVer(a), parseVer(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func parseVer(s string) [3]int {
	var out [3]int
	parts := strings.SplitN(strings.TrimSpace(s), ".", 3)
	for i := 0; i < len(parts) && i < 3; i++ {
		digits := strings.TrimFunc(parts[i], func(r rune) bool { return r < '0' || r > '9' })
		out[i], _ = strconv.Atoi(digits)
	}
	return out
}

// applyUpdate laedt die neue .exe, tauscht sie gegen die laufende aus und startet neu.
func applyUpdate(url string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	tmp := exe + ".new"
	if err := download(url, tmp); err != nil {
		return err
	}
	old := exe + ".old"
	_ = os.Remove(old)
	if err := os.Rename(exe, old); err != nil { // laufende exe umbenennen (Windows erlaubt das)
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, exe); err != nil { // neue exe an den Originalpfad
		_ = os.Rename(old, exe) // Rollback
		return err
	}
	cmd := exec.Command(exe) // neue Version starten ...
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { time.Sleep(300 * time.Millisecond); os.Exit(0) }() // ... und aktuelle beenden
	return nil
}

func download(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "sc-cargo-manager")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %d", resp.StatusCode)
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// cleanupOldUpdate entfernt Reste vom letzten Update (.old, .new) beim Start.
// Die .old ist direkt nach dem Update evtl. noch gesperrt (alter Prozess beendet
// sich gerade) -> im Hintergrund mehrfach versuchen, bis es klappt.
func cleanupOldUpdate() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = os.Remove(exe + ".new") // Download-Rest, falls ein Update abbrach
	old := exe + ".old"
	if _, err := os.Stat(old); err != nil {
		return // keine alte Version vorhanden
	}
	go func() {
		for i := 0; i < 30; i++ { // bis ~9s: warten bis alter Prozess beendet ist
			if os.Remove(old) == nil {
				return
			}
			time.Sleep(300 * time.Millisecond)
		}
	}()
}
