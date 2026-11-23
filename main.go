package main

import (
	"embed"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

//go:embed web
var webFS embed.FS

const appPort = "47800"
const appURL = "http://127.0.0.1:" + appPort + "/"

func main() {
	initDataDir()
	if f, err := os.OpenFile(filepath.Join(dataDir, "log.txt"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
		log.SetOutput(f)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:"+appPort)
	if err != nil {
		// BetterStomp è già aperto: riapro solo la finestra
		openAppWindow(appURL)
		return
	}
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
	go patchLoop()
	go startOverlay()

	start := time.Now()
	go openAppWindow(appURL)

	// resta aperto finché la finestra è aperta, oppure finché c'è una partita in corso
	// (così l'overlay continua a funzionare anche se chiudi la finestra durante il game)
	for {
		time.Sleep(3 * time.Second)
		lp := lastPing.Load()
		idle := lp == 0 && time.Since(start) > 3*time.Minute || lp != 0 && time.Since(time.Unix(lp, 0)) > 150*time.Second
		if !idle {
			continue
		}
		if d, _ := live.snapshot(); d != nil || getClientState().Phase == "InProgress" {
			continue
		}
		engine.save()
		os.Exit(0)
	}
}
