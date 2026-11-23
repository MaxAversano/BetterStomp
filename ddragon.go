package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------- Data Dragon (dati ufficiali Riot) ----------

var webClient = &http.Client{Timeout: 25 * time.Second}

type DDItem struct {
	Name string `json:"name"`
	Gold struct {
		Total       int  `json:"total"`
		Purchasable bool `json:"purchasable"`
	} `json:"gold"`
	Into             []string           `json:"into"`
	From             []string           `json:"from"`
	Tags             []string           `json:"tags"`
	Maps             map[string]bool    `json:"maps"`
	Stats            map[string]float64 `json:"stats"`
	Depth            int                `json:"depth"`
	InStore          *bool              `json:"inStore"`
	RequiredChampion string             `json:"requiredChampion"`
}

type DDChamp struct {
	ID    string   `json:"id"`
	Key   string   `json:"key"`
	Name  string   `json:"name"`
	Title string   `json:"title"`
	Tags  []string `json:"tags"`
}

type Static struct {
	mu       sync.RWMutex
	Version  string
	Versions []string
	Champs   map[int]DDChamp
	Items    map[int]DDItem
	ready    bool
}

var static = &Static{Champs: map[int]DDChamp{}, Items: map[int]DDItem{}}

func ddPath(v, lang, file string) string {
	return filepath.Join(dataDir, "ddragon", v, lang, file)
}

func httpGetBytes(url string) ([]byte, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "BetterStomp/1.0")
	resp, err := webClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// file DDragon con cache su disco
func ddFile(v, lang, file string) ([]byte, error) {
	p := ddPath(v, lang, file)
	if b, err := os.ReadFile(p); err == nil {
		return b, nil
	}
	url := fmt.Sprintf("https://ddragon.leagueoflegends.com/cdn/%s/data/%s/%s", v, lang, file)
	b, err := httpGetBytes(url)
	if err != nil {
		return nil, err
	}
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, b, 0o644)
	return b, nil
}

func loadStatic() {
	for attempt := 0; ; attempt++ {
		var versions []string
		b, err := httpGetBytes("https://ddragon.leagueoflegends.com/api/versions.json")
		if err == nil && json.Unmarshal(b, &versions) == nil && len(versions) > 0 {
			os.MkdirAll(filepath.Join(dataDir, "ddragon"), 0o755)
			os.WriteFile(filepath.Join(dataDir, "ddragon", "versions.json"), b, 0o644)
		} else if readJSON(filepath.Join(dataDir, "ddragon", "versions.json"), &versions) != nil {
			time.Sleep(time.Duration(5+attempt*5) * time.Second)
			continue
		}
		v := versions[0]
		if err := static.load(v, versions); err == nil {
			return
		}
		time.Sleep(10 * time.Second)
	}
}

func (s *Static) load(v string, versions []string) error {
	cb, err := ddFile(v, "it_IT", "champion.json")
	if err != nil {
		return err
	}
	ib, err := ddFile(v, "it_IT", "item.json")
	if err != nil {
		return err
	}
	var cj struct {
		Data map[string]DDChamp `json:"data"`
	}
	var ij struct {
		Data map[string]DDItem `json:"data"`
	}
	if err := json.Unmarshal(cb, &cj); err != nil {
		return err
	}
	if err := json.Unmarshal(ib, &ij); err != nil {
		return err
	}
	champs := map[int]DDChamp{}
	for _, c := range cj.Data {
		k, _ := strconv.Atoi(c.Key)
		champs[k] = c
	}
	items := map[int]DDItem{}
	for id, it := range ij.Data {
		k, _ := strconv.Atoi(id)
		items[k] = it
	}
	s.mu.Lock()
	s.Version, s.Versions, s.Champs, s.Items, s.ready = v, versions, champs, items, true
	s.mu.Unlock()
	// precarico gli altri file usati dall'interfaccia
	go func() {
		ddFile(v, "it_IT", "runesReforged.json")
		ddFile(v, "it_IT", "summoner.json")
	}()
	return nil
}

func (s *Static) version() string { s.mu.RLock(); defer s.mu.RUnlock(); return s.Version }

func champName(id int) string {
	static.mu.RLock()
	defer static.mu.RUnlock()
	if c, ok := static.Champs[id]; ok {
		return c.Name
	}
	return fmt.Sprintf("#%d", id)
}

func champByName(name string) int {
	static.mu.RLock()
	defer static.mu.RUnlock()
	n := strings.ToLower(strings.ReplaceAll(name, " ", ""))
	for k, c := range static.Champs {
		if strings.ToLower(c.ID) == n || strings.ToLower(strings.ReplaceAll(c.Name, " ", "")) == n {
			return k
		}
	}
	return 0
}

func itemInfo(id int) (DDItem, bool) {
	static.mu.RLock()
	defer static.mu.RUnlock()
	it, ok := static.Items[id]
	return it, ok
}

func hasTag(it DDItem, t string) bool {
	for _, x := range it.Tags {
		if x == t {
			return true
		}
	}
	return false
}

// classificazione oggetti per le build
func isBoots(id int) bool {
	it, ok := itemInfo(id)
	return ok && hasTag(it, "Boots") && it.Gold.Total > 300 && len(it.Into) == 0
}
func isCompleted(id int) bool {
	it, ok := itemInfo(id)
	if !ok || id == 0 || hasTag(it, "Consumable") || hasTag(it, "Trinket") || hasTag(it, "Boots") {
		return false
	}
	return len(it.Into) == 0 && it.Gold.Total >= 900
}

func majorMinor(v string) string {
	p := strings.Split(v, ".")
	if len(p) >= 2 {
		return p[0] + "." + p[1]
	}
	return v
}

// ---------- Patch: differenze calcolate tra due versioni di Data Dragon ----------

type PatchChange struct {
	Label string `json:"label"`
	Old   string `json:"old"`
	New   string `json:"new"`
	Good  int    `json:"good"` // 1 buff, -1 nerf, 0 neutro
}

type PatchEntry struct {
	Kind    string        `json:"kind"` // champion | item
	ID      string        `json:"id"`
	Key     int           `json:"key"`
	Name    string        `json:"name"`
	Verdict string        `json:"verdict"`
	Changes []PatchChange `json:"changes"`
	Groups  []PatchGroup  `json:"groups,omitempty"`
	Summary string        `json:"summary,omitempty"`
	Context string        `json:"context,omitempty"`
}

type PatchGroup struct {
	Title   string        `json:"title"`
	Changes []PatchChange `json:"changes"`
}

type PatchReport struct {
	Patch     string       `json:"patch"`
	Previous  string       `json:"previous"`
	Source    string       `json:"source"`
	URL       string       `json:"url"`
	Title     string       `json:"title"`
	Image     string       `json:"image"`
	Champions []PatchEntry `json:"champions"`
	Items     []PatchEntry `json:"items"`
	Other     []PatchEntry `json:"other"`
	Fetched   int64        `json:"fetched"`
}

var statLabels = map[string]string{
	"hp": "Salute base", "hpperlevel": "Salute per livello", "mp": "Mana base", "mpperlevel": "Mana per livello",
	"movespeed": "Velocità movimento", "armor": "Armatura base", "armorperlevel": "Armatura per livello",
	"spellblock": "Resistenza magica", "spellblockperlevel": "Res. magica per livello", "attackrange": "Gittata d'attacco",
	"hpregen": "Rigenerazione salute", "hpregenperlevel": "Rigen. salute per livello", "attackdamage": "Attacco fisico base",
	"attackdamageperlevel": "Attacco fisico per livello", "attackspeedperlevel": "Velocità d'attacco per livello",
	"attackspeed": "Velocità d'attacco base", "mpregen": "Rigenerazione mana",
}

type fullChamp struct {
	ID     string             `json:"id"`
	Key    string             `json:"key"`
	Name   string             `json:"name"`
	Stats  map[string]float64 `json:"stats"`
	Spells []struct {
		Name     string    `json:"name"`
		Cooldown []float64 `json:"cooldown"`
		Cost     []float64 `json:"cost"`
		Range    []float64 `json:"range"`
	} `json:"spells"`
}

func fmtNums(a []float64) string {
	var p []string
	same := true
	for _, x := range a {
		if x != a[0] {
			same = false
		}
	}
	if same && len(a) > 0 {
		return strconv.FormatFloat(a[0], 'f', -1, 64)
	}
	for _, x := range a {
		p = append(p, strconv.FormatFloat(x, 'f', -1, 64))
	}
	return strings.Join(p, "/")
}

func sumF(a []float64) float64 {
	t := 0.0
	for _, x := range a {
		t += x
	}
	return t
}

func ddragonDiff(cur, prev string) (champs, items []PatchEntry, err error) {
	cb, err := ddFile(cur, "it_IT", "championFull.json")
	if err != nil {
		return nil, nil, err
	}
	pb, err := ddFile(prev, "it_IT", "championFull.json")
	if err != nil {
		return nil, nil, err
	}
	var c, p struct {
		Data map[string]fullChamp `json:"data"`
	}
	json.Unmarshal(cb, &c)
	json.Unmarshal(pb, &p)
	keys := []string{"Q", "W", "E", "R"}
	for id, nc := range c.Data {
		oc, ok := p.Data[id]
		k, _ := strconv.Atoi(nc.Key)
		e := PatchEntry{Kind: "champion", ID: id, Key: k, Name: nc.Name}
		if !ok {
			e.Verdict = "nuovo"
			e.Summary = "Nuovo campione"
			champs = append(champs, e)
			continue
		}
		var base []PatchChange
		for sk, nv := range nc.Stats {
			ov := oc.Stats[sk]
			if nv != ov && statLabels[sk] != "" {
				g := 1
				if nv < ov {
					g = -1
				}
				base = append(base, PatchChange{statLabels[sk], strconv.FormatFloat(ov, 'f', -1, 64), strconv.FormatFloat(nv, 'f', -1, 64), g})
			}
		}
		sort.Slice(base, func(i, j int) bool { return base[i].Label < base[j].Label })
		if len(base) > 0 {
			e.Groups = append(e.Groups, PatchGroup{"Statistiche base", base})
		}
		for i := range nc.Spells {
			if i >= len(oc.Spells) || i >= 4 {
				break
			}
			ns, os_ := nc.Spells[i], oc.Spells[i]
			var ch []PatchChange
			if fmtNums(ns.Cooldown) != fmtNums(os_.Cooldown) {
				g := 1
				if sumF(ns.Cooldown) > sumF(os_.Cooldown) {
					g = -1
				}
				ch = append(ch, PatchChange{"Ricarica", fmtNums(os_.Cooldown), fmtNums(ns.Cooldown), g})
			}
			if fmtNums(ns.Cost) != fmtNums(os_.Cost) {
				g := 1
				if sumF(ns.Cost) > sumF(os_.Cost) {
					g = -1
				}
				ch = append(ch, PatchChange{"Costo", fmtNums(os_.Cost), fmtNums(ns.Cost), g})
			}
			if fmtNums(ns.Range) != fmtNums(os_.Range) {
				g := 1
				if sumF(ns.Range) < sumF(os_.Range) {
					g = -1
				}
				ch = append(ch, PatchChange{"Gittata", fmtNums(os_.Range), fmtNums(ns.Range), g})
			}
			if len(ch) > 0 {
				e.Groups = append(e.Groups, PatchGroup{keys[i] + " - " + ns.Name, ch})
			}
		}
		if len(e.Groups) > 0 {
			e.Verdict = verdictOf(e.Groups)
			champs = append(champs, e)
		}
	}
	ci, err := ddFile(cur, "it_IT", "item.json")
	if err == nil {
		pi, err2 := ddFile(prev, "it_IT", "item.json")
		if err2 == nil {
			var a, b struct {
				Data map[string]DDItem `json:"data"`
			}
			json.Unmarshal(ci, &a)
			json.Unmarshal(pi, &b)
			for id, ni := range a.Data {
				if !ni.Maps["11"] || !ni.Gold.Purchasable {
					continue
				}
				oi, ok := b.Data[id]
				k, _ := strconv.Atoi(id)
				e := PatchEntry{Kind: "item", ID: id, Key: k, Name: ni.Name}
				if !ok {
					e.Verdict = "nuovo"
					e.Summary = "Nuovo oggetto"
					items = append(items, e)
					continue
				}
				var ch []PatchChange
				if ni.Gold.Total != oi.Gold.Total {
					g := 1
					if ni.Gold.Total > oi.Gold.Total {
						g = -1
					}
					ch = append(ch, PatchChange{"Costo totale", strconv.Itoa(oi.Gold.Total), strconv.Itoa(ni.Gold.Total), g})
				}
				for sk, nv := range ni.Stats {
					if ov := oi.Stats[sk]; ov != nv {
						g := 1
						if nv < ov {
							g = -1
						}
						ch = append(ch, PatchChange{statName(sk), strconv.FormatFloat(ov, 'f', -1, 64), strconv.FormatFloat(nv, 'f', -1, 64), g})
					}
				}
				for sk, ov := range oi.Stats {
					if _, ok := ni.Stats[sk]; !ok {
						ch = append(ch, PatchChange{statName(sk), strconv.FormatFloat(ov, 'f', -1, 64), "rimosso", -1})
					}
				}
				if len(ch) > 0 {
					e.Groups = []PatchGroup{{"Modifiche", ch}}
					e.Verdict = verdictOf(e.Groups)
					items = append(items, e)
				}
			}
		}
	}
	sort.Slice(champs, func(i, j int) bool { return champs[i].Name < champs[j].Name })
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return champs, items, nil
}

func statName(k string) string {
	m := map[string]string{
		"FlatPhysicalDamageMod": "Attacco fisico", "FlatMagicDamageMod": "Potere magico", "FlatHPPoolMod": "Salute",
		"FlatArmorMod": "Armatura", "FlatSpellBlockMod": "Resistenza magica", "PercentAttackSpeedMod": "Velocità d'attacco",
		"FlatCritChanceMod": "Probabilità di critico", "FlatMovementSpeedMod": "Velocità movimento", "PercentMovementSpeedMod": "Velocità movimento %",
		"FlatMPPoolMod": "Mana", "PercentLifeStealMod": "Rubavita", "FlatHPRegenMod": "Rigenerazione salute",
	}
	if v, ok := m[k]; ok {
		return v
	}
	return k
}

func verdictOf(groups []PatchGroup) string {
	g, b := 0, 0
	for _, gr := range groups {
		for _, c := range gr.Changes {
			if c.Good > 0 {
				g++
			} else if c.Good < 0 {
				b++
			}
		}
	}
	switch {
	case g > 0 && b == 0:
		return "buff"
	case b > 0 && g == 0:
		return "nerf"
	}
	return "modifica"
}

func previousPatchVersion(versions []string, cur string) string {
	mm := majorMinor(cur)
	for _, v := range versions {
		if majorMinor(v) != mm && !strings.HasPrefix(v, "lolpatch") {
			return v
		}
	}
	return ""
}
