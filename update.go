package main

// Aggiornamento automatico dal server di Max (aversano.design):
//  - chi ha il codice del server (Max) pubblica da solo la propria versione se è più nuova;
//  - tutti gli altri scaricano la versione pubblicata e la installano con un clic (o al prossimo avvio).

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// impostata in compilazione: -X main.appVersion=AAAA.MM.GG.HHMM
var appVersion = "0"

const updateURL = "https://www.aversano.design/bs-data/app.php"

var (
	updMu    sync.Mutex
	updReady string // versione scaricata e pronta da installare
	updState string
)

type remoteVersion struct {
	Version string `json:"version"`
	Size    int64  `json:"size"`
	Sha256  string `json:"sha256"`
}

func updateInfo() map[string]any {
	updMu.Lock()
	defer updMu.Unlock()
	return map[string]any{"current": appVersion, "ready": updReady, "state": updState}
}

func updateLoop() {
	time.Sleep(15 * time.Second)
	for {
		if err := checkUpdate(); err != nil {
			log.Printf("aggiornamenti: %v", err)
		}
		time.Sleep(3 * time.Hour)
	}
}

// versioni nel formato AAAA.MM.GG.HHMM: basta il confronto tra stringhe
func newer(a, b string) bool { return a != "" && a != "0" && a > b }

func checkUpdate() error {
	hc := &http.Client{Timeout: 30 * time.Second}
	resp, err := hc.Get(updateURL + "?v=1")
	if err != nil {
		return err
	}
	var rv remoteVersion
	json.NewDecoder(resp.Body).Decode(&rv)
	resp.Body.Close()
	s := getSettings()
	if s.RemoteToken != "" && strings.Contains(s.RemoteURL, "aversano.design") && newer(appVersion, rv.Version) {
		return publishSelf(s.RemoteToken)
	}
	if !newer(rv.Version, appVersion) || appVersion == "0" {
		return nil
	}
	updMu.Lock()
	already := updReady == rv.Version
	updMu.Unlock()
	if already {
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	dst := filepath.Join(filepath.Dir(exe), "BetterStomp.update.exe")
	tmp := dst + ".part"
	updMu.Lock()
	updState = "download"
	updMu.Unlock()
	defer func() {
		updMu.Lock()
		updState = ""
		updMu.Unlock()
	}()
	hc.Timeout = 10 * time.Minute
	resp, err = hc.Get(updateURL + "?get=1")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), resp.Body)
	f.Close()
	if err != nil || hex.EncodeToString(h.Sum(nil)) != rv.Sha256 {
		os.Remove(tmp)
		return fmt.Errorf("download non valido")
	}
	os.Remove(dst)
	if err := os.Rename(tmp, dst); err != nil {
		return err
	}
	updMu.Lock()
	updReady = rv.Version
	updMu.Unlock()
	log.Printf("aggiornamento %s scaricato", rv.Version)
	notify("BetterStomp", "È pronta una nuova versione: aprila dall'app con \"Aggiorna ora\"")
	return nil
}

// carica sul server la versione in uso, a pezzi da 1 MB
func publishSelf(token string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	const chunk = 1 << 20
	n := (len(data) + chunk - 1) / chunk
	hc := &http.Client{Timeout: 2 * time.Minute}
	for i := 0; i < n; i++ {
		end := (i + 1) * chunk
		if end > len(data) {
			end = len(data)
		}
		u := fmt.Sprintf("%s?up=1&k=%s&i=%d&n=%d&ver=%s&sha=%s", updateURL, url.QueryEscape(token), i, n, appVersion, sha)
		resp, err := hc.Post(u, "application/octet-stream", bytes.NewReader(data[i*chunk:end]))
		if err != nil {
			return err
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			return fmt.Errorf("pubblicazione: HTTP %d %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}
	}
	log.Printf("versione %s pubblicata sul server per gli amici", appVersion)
	return nil
}
