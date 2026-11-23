package main

import (
	"encoding/json"
	"html"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Note della patch: pagina ufficiale Riot (it-it) riorganizzata in schede, più le
// differenze verificate calcolate da Data Dragon. Si aggiorna da sola ogni 20 minuti.

var (
	patchMu     sync.RWMutex
	patchReport *PatchReport
	patchErr    string
)

var (
	reHref     = regexp.MustCompile(`/it-it/news/game-updates/[a-z0-9-]*patch[a-z0-9-]*notes[a-z0-9-]*/?`)
	reH2       = regexp.MustCompile(`(?is)<h2[^>]*>(.*?)</h2>`)
	reH3       = regexp.MustCompile(`(?is)<h3[^>]*>(.*?)</h3>`)
	reH4       = regexp.MustCompile(`(?is)<h4[^>]*>(.*?)</h4>`)
	reLi       = regexp.MustCompile(`(?is)<li[^>]*>(.*?)</li>`)
	reSummary  = regexp.MustCompile(`(?is)<p[^>]*class="[^"]*summary[^"]*"[^>]*>(.*?)</p>`)
	reQuote    = regexp.MustCompile(`(?is)<blockquote[^>]*>(.*?)</blockquote>`)
	reTag      = regexp.MustCompile(`(?s)<[^>]+>`)
	reSpace    = regexp.MustCompile(`\s+`)
	reTitle    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	reOgImage  = regexp.MustCompile(`(?is)<meta[^>]+property="og:image"[^>]+content="([^"]+)"`)
	reNextData = regexp.MustCompile(`(?is)<script[^>]+id="__NEXT_DATA__"[^>]*>(.*?)</script>`)
	reNum      = regexp.MustCompile(`-?\d+(?:[.,]\d+)?`)
)

func stripHTML(s string) string {
	s = reTag.ReplaceAllString(s, " ")
	s = html.UnescapeString(s)
	return strings.TrimSpace(reSpace.ReplaceAllString(s, " "))
}

func collectStrings(v any, out *[]string) {
	switch t := v.(type) {
	case string:
		if strings.Contains(t, "<h") || strings.Contains(t, "<li") {
			*out = append(*out, t)
		}
	case []any:
		for _, x := range t {
			collectStrings(x, out)
		}
	case map[string]any:
		for _, x := range t {
			collectStrings(x, out)
		}
	}
}

func firstNum(s string) (float64, bool) {
	m := reNum.FindString(s)
	if m == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.ReplaceAll(m, ",", "."), 64)
	return f, err == nil
}

func parseChange(line string) PatchChange {
	c := PatchChange{}
	arrow := ""
	for _, a := range []string{"⇒", "→", "=>"} {
		if strings.Contains(line, a) {
			arrow = a
			break
		}
	}
	if arrow == "" {
		c.Label = line
		return c
	}
	parts := strings.SplitN(line, arrow, 2)
	left, right := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	if i := strings.Index(left, ":"); i > 0 && i < 70 {
		c.Label = strings.TrimSpace(left[:i])
		c.Old = strings.TrimSpace(left[i+1:])
	} else {
		c.Old = left
	}
	c.New = right
	o, ok1 := firstNum(c.Old)
	n, ok2 := firstNum(c.New)
	if ok1 && ok2 && o != n {
		lower := false
		l := strings.ToLower(c.Label)
		for _, k := range []string{"ricarica", "costo", "tempo", "cooldown", "mana", "durata della ricarica", "prezzo", "salute persa"} {
			if strings.Contains(l, k) {
				lower = true
			}
		}
		if (n > o) != lower {
			c.Good = 1
		} else {
			c.Good = -1
		}
	}
	return c
}

func parseEntry(name, chunk string) PatchEntry {
	e := PatchEntry{Name: stripHTML(name)}
	if m := reSummary.FindStringSubmatch(chunk); m != nil {
		e.Summary = stripHTML(m[1])
	}
	if m := reQuote.FindStringSubmatch(chunk); m != nil {
		e.Context = stripHTML(m[1])
	}
	h4 := reH4.FindAllStringSubmatchIndex(chunk, -1)
	addGroup := func(title, part string) {
		var ch []PatchChange
		for _, li := range reLi.FindAllStringSubmatch(part, -1) {
			t := stripHTML(li[1])
			if t != "" {
				ch = append(ch, parseChange(t))
			}
		}
		if len(ch) > 0 {
			e.Groups = append(e.Groups, PatchGroup{Title: title, Changes: ch})
		}
	}
	if len(h4) == 0 {
		addGroup("", chunk)
	} else {
		addGroup("", chunk[:h4[0][0]])
		for i, m := range h4 {
			end := len(chunk)
			if i+1 < len(h4) {
				end = h4[i+1][0]
			}
			addGroup(stripHTML(chunk[m[2]:m[3]]), chunk[m[1]:end])
		}
	}
	e.Verdict = verdictOf(e.Groups)
	if len(e.Groups) == 0 {
		e.Verdict = "modifica"
	}
	return e
}

func parseOfficialNotes(page string) (title, image string, champs, items, other []PatchEntry) {
	if m := reTitle.FindStringSubmatch(page); m != nil {
		title = stripHTML(m[1])
	}
	if m := reOgImage.FindStringSubmatch(page); m != nil {
		image = html.UnescapeString(m[1])
	}
	body := page
	if !strings.Contains(page, "change-title") {
		if m := reNextData.FindStringSubmatch(page); m != nil {
			var v any
			if json.Unmarshal([]byte(m[1]), &v) == nil {
				var strs []string
				collectStrings(v, &strs)
				body = strings.Join(strs, "\n")
			}
		}
	}
	h2 := reH2.FindAllStringSubmatchIndex(body, -1)
	for i, m := range h2 {
		end := len(body)
		if i+1 < len(h2) {
			end = h2[i+1][0]
		}
		section := stripHTML(body[m[2]:m[3]])
		chunk := body[m[1]:end]
		ls := strings.ToLower(section)
		if strings.Contains(ls, "skin") || strings.Contains(ls, "cromatic") {
			continue
		}
		h3 := reH3.FindAllStringSubmatchIndex(chunk, -1)
		var entries []PatchEntry
		for j, h := range h3 {
			e2 := len(chunk)
			if j+1 < len(h3) {
				e2 = h3[j+1][0]
			}
			en := parseEntry(chunk[h[2]:h[3]], chunk[h[1]:e2])
			if en.Name != "" && (len(en.Groups) > 0 || en.Summary != "") {
				entries = append(entries, en)
			}
		}
		if len(h3) == 0 {
			en := parseEntry(section, chunk)
			if len(en.Groups) > 0 {
				entries = append(entries, en)
			}
		}
		for k := range entries {
			entries[k].Context = strings.TrimSpace(entries[k].Context)
		}
		switch {
		case strings.Contains(ls, "campion"):
			for k := range entries {
				entries[k].Kind = "champion"
				entries[k].Key = champByName(entries[k].Name)
				static.mu.RLock()
				if c, ok := static.Champs[entries[k].Key]; ok {
					entries[k].ID = c.ID
				}
				static.mu.RUnlock()
			}
			champs = append(champs, entries...)
		case strings.Contains(ls, "oggett"):
			for k := range entries {
				entries[k].Kind = "item"
			}
			items = append(items, entries...)
		default:
			for k := range entries {
				entries[k].Kind = "other"
				if entries[k].Name != section {
					entries[k].Name = section + " · " + entries[k].Name
				}
			}
			other = append(other, entries...)
		}
	}
	return
}

func findItemIDByName(name string) int {
	static.mu.RLock()
	defer static.mu.RUnlock()
	n := strings.ToLower(strings.TrimSpace(name))
	for id, it := range static.Items {
		if strings.ToLower(it.Name) == n && it.Maps["11"] {
			return id
		}
	}
	return 0
}

func refreshPatchNotes(force bool) {
	static.mu.RLock()
	v, versions := static.Version, static.Versions
	static.mu.RUnlock()
	if v == "" {
		return
	}
	mm := majorMinor(v)
	cache := filepath.Join(dataDir, "patch_"+mm+".json")
	if !force {
		var r PatchReport
		if readJSON(cache, &r) == nil && r.Source == "official" && time.Since(time.Unix(r.Fetched, 0)) < 6*time.Hour {
			patchMu.Lock()
			patchReport = &r
			patchMu.Unlock()
			return
		}
	}
	rep := &PatchReport{Patch: mm, Fetched: time.Now().Unix()}
	prev := previousPatchVersion(versions, v)
	rep.Previous = majorMinor(prev)

	// 1) note ufficiali Riot
	if list, err := httpGetBytes("https://www.leagueoflegends.com/it-it/news/tags/patch-notes/"); err == nil {
		if href := reHref.FindString(string(list)); href != "" {
			url := "https://www.leagueoflegends.com" + href
			if page, err := httpGetBytes(url); err == nil {
				t, img, c, it, o := parseOfficialNotes(string(page))
				if len(c)+len(it)+len(o) > 0 {
					rep.Source, rep.URL, rep.Title, rep.Image = "official", url, t, img
					rep.Champions, rep.Items, rep.Other = c, it, o
					for k := range rep.Items {
						rep.Items[k].Key = findItemIDByName(rep.Items[k].Name)
					}
				}
			}
		}
	}
	// 2) differenze Data Dragon (sempre disponibili, usate se la pagina ufficiale non c'è ancora)
	if rep.Source == "" && prev != "" {
		c, it, err := ddragonDiff(v, prev)
		if err == nil {
			rep.Source = "ddragon"
			rep.Title = "Patch " + mm + " · modifiche rilevate dai dati di gioco"
			rep.Champions, rep.Items = c, it
		} else {
			patchMu.Lock()
			patchErr = err.Error()
			patchMu.Unlock()
		}
	}
	if rep.Source != "" {
		writeJSON(cache, rep)
		patchMu.Lock()
		patchReport = rep
		patchErr = ""
		patchMu.Unlock()
	}
}

func patchLoop() {
	for !staticReady() {
		time.Sleep(time.Second)
	}
	refreshPatchNotes(false)
	for {
		time.Sleep(20 * time.Minute)
		// nuova versione di Data Dragon = nuova patch uscita sul client
		old := static.version()
		var versions []string
		if b, err := httpGetBytes("https://ddragon.leagueoflegends.com/api/versions.json"); err == nil && json.Unmarshal(b, &versions) == nil && len(versions) > 0 {
			if versions[0] != old {
				static.load(versions[0], versions)
				refreshPatchNotes(true)
				continue
			}
		}
		patchMu.RLock()
		r := patchReport
		patchMu.RUnlock()
		if r == nil || r.Source != "official" || time.Since(time.Unix(r.Fetched, 0)) > 3*time.Hour {
			refreshPatchNotes(true)
		}
	}
}

func staticReady() bool { static.mu.RLock(); defer static.mu.RUnlock(); return static.ready }
