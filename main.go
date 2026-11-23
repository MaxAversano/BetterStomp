package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

//go:embed web
var webFS embed.FS

var appURL string

// cerca una porta libera; se su una porta risponde già BetterStomp riapre solo la finestra.
// (prima usava sempre la 47800: se un'altra app, es. Casa TV, la occupava, si apriva quella)
func listenFree() net.Listener {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	for p := 47823; p < 47860; p++ {
		addr := fmt.Sprintf("127.0.0.1:%d", p)
		if ln, err := net.Listen("tcp", addr); err == nil {
			appURL = "http://" + addr + "/"
			return ln
		}
		if resp, err := client.Get("http://" + addr + "/api/whoami"); err == nil {
			var who struct {
				App string `json:"app"`
			}
			json.NewDecoder(resp.Body).Decode(&who)
			resp.Body.Close()
			if who.App == "BetterStomp" {
				// già aperta: chiede all'istanza in corso di mostrare la sua finestra
				if r2, err := client.Get("http://" + addr + "/api/win?a=open"); err == nil && r2.StatusCode < 300 {
					r2.Body.Close()
				} else {
					openAppWindowExternal("http://" + addr + "/")
				}
				os.Exit(0)
			}
		}
	}
	return nil
}

func main() {
	initDataDir()
	ln := listenFree() // se l'app è già aperta, qui mostra la sua finestra ed esce
	if ln == nil {
		return
	}
	if f, err := os.OpenFile(filepath.Join(dataDir, "log.txt"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
		log.SetOutput(f)
	}
	if f, err := os.OpenFile(filepath.Join(dataDir, "crash.txt"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		debug.SetCrashOutput(f, debug.CrashOptions{}) // eventuali crash finiscono qui
	}
	log.Printf("avvio su %s", appURL)
	go opggWarmup()
	go updateLoop()
	loadSettings()
	loadPersonal()
	setDPIAware()

	mux := http.NewServeMux()
	setupAPI(mux)
	sub, _ := fs.Sub(webFS, "web")
	files := http.FileServer(http.FS(sub))
	mux.HandleFunc("/obs", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = "/obs.html"
		files.ServeHTTP(w, r)
	})
	mux.Handle("/", files)
	go http.Serve(ln, mux)

	go loadStatic()
	go lcuLoop()
	go liveLoop()
	go engineLoop()
	go remoteLoop()
	go legacyLoop()
	go patchLoop()
	go startOverlay()
	go startOCR()

	start := time.Now()
	go framelessLoop()
	go cleanupOldExe()
	hidden := false
	for _, a := range os.Args[1:] {
		hidden = hidden || a == "--nascosto"
	}
	if !hidden {
		go openAppWindow(appURL)
	}

	// resta aperto finché la finestra è aperta, oppure finché c'è una partita in corso
	// (così l'overlay continua a funzionare anche se chiudi la finestra durante il game)
	for {
		time.Sleep(3 * time.Second)
		lp := lastPing.Load()
		idle := lp == 0 && time.Since(start) > 3*time.Minute || lp != 0 && time.Since(time.Unix(lp, 0)) > 20*time.Second
		if !idle {
			continue
		}
		if d, _ := live.snapshot(); d != nil || getClientState().Phase == "InProgress" {
			continue
		}
		if opggSyncState().Running { // finisce di scaricare i dati OP.GG, poi si chiude
			continue
		}
		if getSettings().Autostart { // con l'avvio automatico resta nella barra, pronto per la prossima partita
			continue
		}
		log.Printf("chiusura: finestra chiusa (ultimo ping %v)", time.Unix(lp, 0).Format("15:04:05"))
		appExit()
	}
}

func restartApp() {
	time.Sleep(300 * time.Millisecond)
	if exe, err := os.Executable(); err == nil {
		relaunchLater(exe) // riparte dopo un paio di secondi, quando questa istanza è chiusa
	}
	appExit()
}

// aggiornamento senza chiudere a mano: l'exe in uso si può rinominare, non sovrascrivere
func selfUpdate() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	upd := filepath.Join(filepath.Dir(exe), "BetterStomp.update.exe")
	if _, err := os.Stat(upd); err != nil {
		return err
	}
	old := exe + ".old"
	os.Remove(old)
	if err := os.Rename(exe, old); err != nil {
		return err
	}
	if err := os.Rename(upd, exe); err != nil {
		os.Rename(old, exe)
		return err
	}
	return nil
}

func cleanupOldExe() {
	if exe, err := os.Executable(); err == nil {
		// aggiornamento lasciato in cartella mentre l'app era chiusa: lo applica al prossimo avvio
		if upd := filepath.Join(filepath.Dir(exe), "BetterStomp.update.exe"); fileExists(upd) {
			if selfUpdate() == nil {
				restartApp()
				return
			}
			os.Remove(upd)
		}
		for i := 0; i < 10; i++ {
			if os.Remove(exe+".old") == nil || !fileExists(exe+".old") {
				return
			}
			time.Sleep(time.Second)
		}
	}
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }
