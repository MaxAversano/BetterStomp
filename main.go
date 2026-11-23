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
				openAppWindow("http://" + addr + "/")
				os.Exit(0)
			}
		}
	}
	return nil
}

func main() {
	initDataDir()
	if f, err := os.OpenFile(filepath.Join(dataDir, "log.txt"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644); err == nil {
		log.SetOutput(f)
	}
	ln := listenFree()
	if ln == nil {
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
		idle := lp == 0 && time.Since(start) > 3*time.Minute || lp != 0 && time.Since(time.Unix(lp, 0)) > 20*time.Second
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
