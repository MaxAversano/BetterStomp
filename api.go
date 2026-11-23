package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

var lastPing atomic.Int64

func jsonOut(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

func atoi(s string) int { n, _ := strconv.Atoi(s); return n }

func setupAPI(mux *http.ServeMux) {
	mux.HandleFunc("/api/whoami", func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, map[string]string{"app": "BetterStomp"})
	})

	// diagnostica: risposta grezza del client di League (solo dal PC locale)
	mux.HandleFunc("/api/debug/lcu", func(w http.ResponseWriter, r *http.Request) {
		b, code, err := lcu.req("GET", r.URL.Query().Get("path"), nil)
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(code)
		w.Write(b)
	})

	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		lastPing.Store(time.Now().Unix())
		jsonOut(w, map[string]bool{"ok": true})
	})

	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		lastPing.Store(time.Now().Unix())
		d, gt := live.snapshot()
		jsonOut(w, map[string]any{
			"client":     getClientState(),
			"version":    static.version(),
			"engine":     engine.Status(),
			"inGame":     d != nil,
			"gameTime":   gt,
			"autoTier":   autoTier(),
			"riotKey":    riot.getStatus(),
			"frameless":  isFrameless(),
			"profileVer": profileVer.Load(),
			"window":     windowKind(),
		})
	})

	mux.HandleFunc("/api/win", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("a") == "update" { // sostituisce l'exe con BetterStomp.update.exe e riavvia
			if err := selfUpdate(); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			w.WriteHeader(204)
			go restartApp()
			return
		}
		if q.Get("a") == "restart" { // riavvia l'app (dopo un aggiornamento dell'exe)
			w.WriteHeader(204)
			go restartApp()
			return
		}
		if q.Get("a") == "open" {
			go openAppWindow(appURL)
			w.WriteHeader(204)
			return
		}
		if q.Get("a") == "debug" {
			jsonOut(w, frameInfo())
			return
		}
		if q.Get("a") == "inner" {
			h, _ := strconv.ParseFloat(q.Get("h"), 64)
			d, _ := strconv.ParseFloat(q.Get("d"), 64)
			setInner(h, d)
		} else {
			windowAction(q.Get("a"))
		}
		w.WriteHeader(204)
	})

	mux.HandleFunc("/api/settings", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			var s Settings
			if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
				http.Error(w, err.Error(), 400)
				return
			}
			old := getSettings()
			if err := saveSettings(s); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
			if old.RiotAPIKey != s.RiotAPIKey {
				riot.setStatus("")
			}
		}
		jsonOut(w, getSettings())
	})

	// modifica di singoli valori (es. "overlay.cs.x"): non sovrascrive ciò che è cambiato altrove,
	// come le posizioni trascinate direttamente in gioco
	mux.HandleFunc("/api/settings/patch", func(w http.ResponseWriter, r *http.Request) {
		var changes map[string]any
		if err := json.NewDecoder(r.Body).Decode(&changes); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		old := getSettings()
		b, _ := json.Marshal(old)
		var m map[string]any
		json.Unmarshal(b, &m)
		for path, v := range changes {
			keys := strings.Split(path, ".")
			cur := m
			for i, k := range keys {
				if i == len(keys)-1 {
					cur[k] = v
					break
				}
				next, ok := cur[k].(map[string]any)
				if !ok {
					next = map[string]any{}
					cur[k] = next
				}
				cur = next
			}
		}
		b, _ = json.Marshal(m)
		ns := defaultSettings()
		if err := json.Unmarshal(b, &ns); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		saveSettings(ns)
		if old.Autostart != ns.Autostart {
			go setAutostart(ns.Autostart)
		}
		if old.RiotAPIKey != ns.RiotAPIKey {
			riot.setStatus("")
		}
		jsonOut(w, ns)
	})

	mux.HandleFunc("/api/settings/reset-overlay", func(w http.ResponseWriter, r *http.Request) {
		s := getSettings()
		s.Overlay = defaultSettings().Overlay
		saveSettings(s)
		jsonOut(w, s)
	})

	mux.HandleFunc("/api/static/", func(w http.ResponseWriter, r *http.Request) {
		file := strings.TrimPrefix(r.URL.Path, "/api/static/")
		if strings.Contains(file, "..") || file == "" {
			http.Error(w, "bad", 400)
			return
		}
		v := static.version()
		if v == "" {
			http.Error(w, "dati non ancora pronti", 503)
			return
		}
		b, err := ddFile(v, "it_IT", file)
		if err != nil {
			http.Error(w, err.Error(), 502)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "max-age=3600")
		w.Write(b)
	})

	mux.HandleFunc("/api/tierlist", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		tier := q.Get("tier")
		if tier == "" || tier == "auto" {
			tier = autoTier()
		}
		rows, matches, patch := engine.TierList(q.Get("mode"), tier, q.Get("pos"))
		jsonOut(w, map[string]any{"rows": rows, "matches": matches, "patch": patch, "tier": tier})
	})

	mux.HandleFunc("/api/champion", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		b := resolveBuild(atoi(q.Get("id")), q.Get("pos"), q.Get("mode"), q.Get("tier"), q.Get("personal") != "0", atoi(q.Get("variant")))
		pb, has := getPersonal(atoi(q.Get("id")))
		jsonOut(w, map[string]any{"build": b, "personal": pb, "hasPersonal": has})
	})

	mux.HandleFunc("/api/suggest", func(w http.ResponseWriter, r *http.Request) {
		var q SuggestReq
		json.NewDecoder(r.Body).Decode(&q)
		if q.Tier == "" || q.Tier == "auto" {
			q.Tier = autoTier()
		}
		jsonOut(w, engine.Suggest(q))
	})

	mux.HandleFunc("/api/personal", func(w http.ResponseWriter, r *http.Request) {
		id := atoi(r.URL.Query().Get("id"))
		if r.Method == "POST" {
			var b PersonalBuild
			json.NewDecoder(r.Body).Decode(&b)
			if err := setPersonal(id, b); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}
		b, ok := getPersonal(id)
		jsonOut(w, map[string]any{"build": b, "exists": ok})
	})

	mux.HandleFunc("/api/import", func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Champ   int    `json:"champ"`
			Pos     string `json:"pos"`
			Mode    string `json:"mode"`
			Runes   bool   `json:"runes"`
			Spells  bool   `json:"spells"`
			Items   bool   `json:"items"`
			Variant int    `json:"variant"`
			Custom  *struct {
				Runes       *RunePage `json:"runes"`
				Spells      []int     `json:"spells"`
				Starting    []int     `json:"starting"`
				Core        []int     `json:"core"`
				Situational []int     `json:"situational"`
			} `json:"custom"`
		}
		json.NewDecoder(r.Body).Decode(&q)
		var msg string
		if q.Custom != nil {
			b := BuildResult{Runes: q.Custom.Runes, Spells: q.Custom.Spells, Starting: q.Custom.Starting, Core: q.Custom.Core, Situational: q.Custom.Situational}
			msg = importResolved(q.Champ, q.Pos, b, q.Runes, q.Spells, q.Items)
		} else {
			msg = importBuild(q.Champ, q.Pos, q.Mode, q.Runes, q.Spells, q.Items, q.Variant)
		}
		jsonOut(w, map[string]string{"message": msg})
	})

	mux.HandleFunc("/api/bench", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		jsonOut(w, engine.Benchmarks(0, q.Get("pos"), q.Get("tier")))
	})

	mux.HandleFunc("/api/lp", func(w http.ResponseWriter, r *http.Request) {
		puuid := ""
		if s := getClientState().Summoner; s != nil {
			puuid = s.Puuid
		}
		jsonOut(w, lpHistory(puuid))
	})

	mux.HandleFunc("/api/cs/mates", func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, champSelectMates())
	})

	mux.HandleFunc("/api/profile", func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, ownProfile(r.URL.Query().Get("force") == "1"))
	})

	mux.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		jsonOut(w, searchProfile(q.Get("riotId"), q.Get("region")))
	})

	mux.HandleFunc("/api/patch", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("force") == "1" {
			refreshPatchNotes(true)
		}
		patchMu.RLock()
		rep, perr := patchReport, patchErr
		patchMu.RUnlock()
		if rep == nil && staticReady() {
			refreshPatchNotes(false)
			patchMu.RLock()
			rep, perr = patchReport, patchErr
			patchMu.RUnlock()
		}
		jsonOut(w, map[string]any{"report": rep, "error": perr})
	})

	mux.HandleFunc("/api/overlay/model", func(w http.ResponseWriter, r *http.Request) {
		jsonOut(w, buildOverlayModel(r.URL.Query().Get("preview") == "1"))
	})

	mux.HandleFunc("/api/obs", func(w http.ResponseWriter, r *http.Request) {
		cs := getClientState()
		s := getSettings().Stream
		out := map[string]any{"stream": s, "connected": cs.Connected}
		if cs.Summoner != nil {
			out["name"] = cs.Summoner.GameName
			out["tag"] = cs.Summoner.TagLine
			out["icon"] = cs.Summoner.ProfileIconID
		}
		q := s.Queue
		if q == "" {
			q = "RANKED_SOLO_5x5"
		}
		if rq, ok := cs.Ranked[q]; ok {
			out["rank"] = rq
		}
		if cs.Connected {
			p := ownProfile(false)
			out["today"] = p.Today
		}
		out["version"] = static.version()
		jsonOut(w, out)
	})
}
