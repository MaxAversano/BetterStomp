package main

// Due automatismi per non dover più usare FileZilla o riavviare a mano:
//  - se nella cartella compare BetterStomp.update.exe, l'app propone "Aggiorna ora" (e si aggiorna da sola
//    quando la finestra è chiusa e non sei in partita);
//  - i file PHP nella cartella "Server per aversano.design\bs-data" vengono caricati sul server quando cambiano.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var serverFiles = []string{"app.php", "data.php", "lib.php", "player.php", "run.php", "ping.php", "index.php"}

func localUpdateLoop() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	upd := filepath.Join(filepath.Dir(exe), "BetterStomp.update.exe")
	for {
		time.Sleep(20 * time.Second)
		if !fileExists(upd) {
			continue
		}
		updMu.Lock()
		if updReady == "" && updState == "" {
			updReady = "locale"
		}
		ready := updReady != ""
		updMu.Unlock()
		if !ready {
			continue
		}
		// finestra chiusa da un po' e nessuna partita: si aggiorna da sola
		lp := lastPing.Load()
		idle := lp == 0 || time.Since(time.Unix(lp, 0)) > 60*time.Second
		if d, _ := live.snapshot(); idle && d == nil && getClientState().Phase != "InProgress" && getClientState().Phase != "ChampSelect" {
			log.Printf("aggiornamento in sottofondo: riavvio")
			if selfUpdate() == nil {
				restartApp()
				return
			}
		}
	}
}

func serverSyncLoop() {
	time.Sleep(30 * time.Second)
	for {
		syncServerFiles()
		time.Sleep(time.Minute)
	}
}

func syncServerFiles() {
	s := getSettings()
	if s.RemoteToken == "" || !strings.Contains(s.RemoteURL, "aversano.design") {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Join(filepath.Dir(exe), "Server per aversano.design", "bs-data")
	if !fileExists(dir) {
		return
	}
	statePath := filepath.Join(dataDir, "server_sync.json")
	done := map[string]string{}
	readJSON(statePath, &done)
	changed := false
	hc := &http.Client{Timeout: time.Minute}
	for _, name := range serverFiles {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || len(data) < 10 {
			continue
		}
		sum := sha256.Sum256(data)
		h := hex.EncodeToString(sum[:])
		if done[name] == h {
			continue
		}
		u := fmt.Sprintf("%s?put=%s&k=%s", updateURL, name, url.QueryEscape(s.RemoteToken))
		resp, err := hc.Post(u, "application/octet-stream", bytes.NewReader(data))
		if err != nil {
			log.Printf("server: %s non caricato: %v", name, err)
			return
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			// la prima volta app.php sul server è ancora la versione senza "put": lo segno e riprovo più tardi
			log.Printf("server: %s non caricato: HTTP %d %s", name, resp.StatusCode, strings.TrimSpace(string(body)))
			return
		}
		log.Printf("server: %s aggiornato", name)
		done[name] = h
		changed = true
	}
	if changed {
		writeJSON(statePath, done)
	}
}
