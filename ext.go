package main

// Fonti esterne già preparate dal server (aversano.design):
//   - LoLalytics: statistiche, build, rune, incantesimi, abilità e matchup per campione/corsia/elo
//   - onetricks.gg: partite recenti dei one-trick di ogni campione
// Il server le scarica piano in sottofondo; l'app scarica solo i file pronti (e solo le parti
// cambiate), li tiene in una copia locale e all'avvio è subito pronta.

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type llBuildPack struct {
	SO     string      `json:"so"`
	SON    int         `json:"son"`
	SOWR   float64     `json:"sowr"`
	SP     string      `json:"sp"`
	Sums   []int       `json:"sums"`
	SumsN  int         `json:"sumsn"`
	SumsWR float64     `json:"sumswr"`
	R      llRunes     `json:"r"`
	St     llSet       `json:"st"`
	Core   llSet       `json:"core"`
	I4     [][]float64 `json:"i4"`
	I5     [][]float64 `json:"i5"`
	I6     [][]float64 `json:"i6"`
}
type llRunes struct {
	Pri []int   `json:"pri"`
	Sec []int   `json:"sec"`
	Mod []int   `json:"mod"`
	N   int     `json:"n"`
	WR  float64 `json:"wr"`
}
type llSet struct {
	Set []int   `json:"set"`
	N   int     `json:"n"`
	WR  float64 `json:"wr"`
}
type llEntry struct {
	C    int    `json:"c"`
	Lane string `json:"lane"`
	T    string `json:"t"`
	P    string `json:"p"`
	At   int64  `json:"at"`
	H    struct {
		WR   float64 `json:"wr"`
		PR   float64 `json:"pr"`
		BR   float64 `json:"br"`
		N    int     `json:"n"`
		Tier string  `json:"tier"`
		Rank int     `json:"rank"`
		Of   int     `json:"of"`
		Avg  float64 `json:"avg"`
	} `json:"h"`
	Lanes map[string]float64 `json:"lanes"`
	B     *llBuildPack       `json:"b"`
	BW    *llBuildPack       `json:"bw"`
	VS    [][]float64        `json:"vs"` // cid, vsWr, n, d1
}
type otGame struct {
	N    string `json:"n"`
	T    string `json:"t"`
	Reg  string `json:"reg"`
	W    bool   `json:"w"`
	K    int    `json:"k"`
	D    int    `json:"d"`
	A    int    `json:"a"`
	It   []int  `json:"it"`
	Ord  []int  `json:"ord"`
	KS   int    `json:"ks"`
	Sub  int    `json:"sub"`
	SP   []int  `json:"sp"`
	Opp  int    `json:"opp"`
	Role string `json:"role"`
	At   int64  `json:"at"`
	P    string `json:"p"`
	ID   string `json:"id"`
}
type otEntry struct {
	C    int      `json:"c"`
	At   int64    `json:"at"`
	Role string   `json:"role"`
	G    []otGame `json:"g"`
}

var extTiers = []string{"emerald_plus", "platinum_plus", "master_plus", "all"}

var ext = struct {
	mu   sync.RWMutex
	ll   map[string]map[string]llEntry // tier -> "cid_lane" -> dati
	llAt map[string]int64
	ot   map[int]otEntry
	otAt int64
	err  string
	at   time.Time
}{ll: map[string]map[string]llEntry{}, llAt: map[string]int64{}, ot: map[int]otEntry{}}

func extPath(name string) string { return filepath.Join(statsDir(), "ext_"+name+".json") }

func extLoadLocal() {
	ext.mu.Lock()
	defer ext.mu.Unlock()
	for _, t := range extTiers {
		var c struct {
			At    int64              `json:"at"`
			Items map[string]llEntry `json:"items"`
		}
		if readJSON(extPath("ll_"+t), &c) == nil && c.Items != nil {
			ext.ll[t], ext.llAt[t] = c.Items, c.At
		}
	}
	var o struct {
		At    int64           `json:"at"`
		Items map[int]otEntry `json:"items"`
	}
	if readJSON(extPath("ot"), &o) == nil && o.Items != nil {
		ext.ot, ext.otAt = o.Items, o.At
	}
}

func extLoop() {
	extLoadLocal()
	first := true
	for {
		if !first {
			time.Sleep(10 * time.Minute)
		}
		first = false
		s := getSettings()
		if s.RemoteURL == "" || s.RemoteToken == "" {
			continue
		}
		base := strings.TrimRight(s.RemoteURL, "/")
		if !strings.HasSuffix(base, ".php") {
			base += "/data.php"
		}
		q := "?k=" + url.QueryEscape(s.RemoteToken)
		var errs []string
		for _, t := range extTiers {
			ext.mu.RLock()
			since := ext.llAt[t]
			ext.mu.RUnlock()
			b, err := httpGetBytes(fmt.Sprintf("%s%s&ext=ll&tier=%s&since=%d", base, q, t, since))
			if err != nil {
				errs = append(errs, err.Error())
				continue
			}
			var r struct {
				At    int64     `json:"at"`
				Items []llEntry `json:"items"`
			}
			if json.Unmarshal(b, &r) != nil {
				continue
			}
			ext.mu.Lock()
			m := ext.ll[t]
			if m == nil {
				m = map[string]llEntry{}
				ext.ll[t] = m
			}
			for _, e := range r.Items {
				m[fmt.Sprintf("%d_%s", e.C, e.Lane)] = e
			}
			if r.At > ext.llAt[t] {
				ext.llAt[t] = r.At
			}
			snap := struct {
				At    int64              `json:"at"`
				Items map[string]llEntry `json:"items"`
			}{ext.llAt[t], m}
			ext.mu.Unlock()
			if len(r.Items) > 0 {
				writeJSON(extPath("ll_"+t), snap)
			}
		}
		ext.mu.RLock()
		since := ext.otAt
		ext.mu.RUnlock()
		if b, err := httpGetBytes(fmt.Sprintf("%s%s&ext=ot&since=%d", base, q, since)); err == nil {
			var r struct {
				At    int64     `json:"at"`
				Items []otEntry `json:"items"`
			}
			if json.Unmarshal(b, &r) == nil {
				ext.mu.Lock()
				for _, e := range r.Items {
					ext.ot[e.C] = e
				}
				if r.At > ext.otAt {
					ext.otAt = r.At
				}
				snap := struct {
					At    int64           `json:"at"`
					Items map[int]otEntry `json:"items"`
				}{ext.otAt, ext.ot}
				ext.mu.Unlock()
				if len(r.Items) > 0 {
					writeJSON(extPath("ot"), snap)
				}
			}
		} else {
			errs = append(errs, err.Error())
		}
		ext.mu.Lock()
		ext.at = time.Now()
		ext.err = strings.Join(errs, "; ")
		n := 0
		for _, m := range ext.ll {
			n += len(m)
		}
		no := len(ext.ot)
		ext.mu.Unlock()
		log.Printf("fonti esterne: %d build LoLalytics, %d campioni onetricks.gg", n, no)
	}
}

// elo dell'app -> elo preparato dal server
func llTier(tf string) string {
	t := strings.ToUpper(strings.TrimSpace(tf))
	switch t {
	case "", "ALL", "AUTO", "IRON", "BRONZE", "SILVER", "GOLD", "IRON+", "BRONZE+", "SILVER+", "GOLD+":
		return "all"
	case "PLATINUM", "PLATINUM+":
		return "platinum_plus"
	case "MASTER", "MASTER+", "GRANDMASTER", "GRANDMASTER+", "CHALLENGER":
		return "master_plus"
	}
	return "emerald_plus" // EMERALD, EMERALD+, DIAMOND, DIAMOND+
}

var pos2ll = map[string]string{"TOP": "top", "JUNGLE": "jungle", "MIDDLE": "middle", "MID": "middle", "BOTTOM": "bottom", "ADC": "bottom", "UTILITY": "support", "SUPPORT": "support"}
var ll2pos = map[string]string{"top": "TOP", "jungle": "JUNGLE", "middle": "MIDDLE", "bottom": "BOTTOM", "support": "UTILITY"}

func llFind(champ int, pos, tier string) (llEntry, bool) {
	ext.mu.RLock()
	defer ext.mu.RUnlock()
	m := ext.ll[llTier(tier)]
	if len(m) == 0 {
		m = ext.ll["emerald_plus"]
	}
	if l, ok := pos2ll[strings.ToUpper(pos)]; ok {
		if e, ok := m[fmt.Sprintf("%d_%s", champ, l)]; ok {
			return e, true
		}
	}
	// ruolo non indicato (o senza dati): la corsia più giocata fra quelle disponibili
	var best llEntry
	found := false
	for _, l := range []string{"top", "jungle", "middle", "bottom", "support"} {
		if e, ok := m[fmt.Sprintf("%d_%s", champ, l)]; ok && (!found || e.Lanes[l] > best.Lanes[best.Lane]) {
			best, found = e, true
		}
	}
	return best, found && pos == ""
}

// stile (albero) di una runa, dai dati di Riot
var perkStyleMap struct {
	sync.Once
	m map[int]int
}

func perkStyle(perk int) int {
	perkStyleMap.Do(func() {
		perkStyleMap.m = map[int]int{}
		b, err := ddFile(static.version(), "it_IT", "runesReforged.json")
		if err != nil {
			return
		}
		var st []struct {
			ID    int `json:"id"`
			Slots []struct {
				Runes []struct {
					ID int `json:"id"`
				} `json:"runes"`
			} `json:"slots"`
		}
		if json.Unmarshal(b, &st) == nil {
			for _, s := range st {
				for _, sl := range s.Slots {
					for _, r := range sl.Runes {
						perkStyleMap.m[r.ID] = s.ID
					}
				}
			}
		}
	})
	if s, ok := perkStyleMap.m[perk]; ok {
		return s
	}
	if perk >= 8000 && perk < 8500 {
		return perk / 100 * 100
	}
	return 0
}

func llSkillSeq(id string) string {
	var b strings.Builder
	for _, c := range id {
		switch c {
		case '1':
			b.WriteByte('Q')
		case '2':
			b.WriteByte('W')
		case '3':
			b.WriteByte('E')
		case '4':
			b.WriteByte('R')
		}
	}
	return b.String()
}

func llRunePage(r llRunes) *RunePage {
	if len(r.Pri) < 4 || len(r.Sec) < 2 {
		return nil
	}
	perks := append(append(append([]int{}, r.Pri...), r.Sec...), r.Mod...)
	return &RunePage{Primary: perkStyle(r.Pri[0]), Secondary: perkStyle(r.Sec[0]), Perks: perks, Games: r.N, WinRate: r.WR / 100, Source: "LoLalytics"}
}

// build "variante" da una build di LoLalytics: oggetti principali + le scelte per 4°, 5° e 6° oggetto
func llVariant(b *llBuildPack, label string) BuildVariant {
	v := BuildVariant{Label: label, Path: b.Core.Set, Games: b.Core.N, WinRate: b.Core.WR / 100, Runes: llRunePage(b.R)}
	seen := map[int]bool{}
	for _, id := range b.Core.Set {
		seen[id] = true
		v.Core = append(v.Core, id)
	}
	for _, l := range [][][]float64{b.I4, b.I5, b.I6} {
		for i, o := range l {
			id := int(o[0])
			if seen[id] {
				continue
			}
			seen[id] = true
			if i == 0 {
				v.Core = append(v.Core, id)
			} else {
				v.Situational = append(v.Situational, id)
			}
		}
	}
	if len(b.R.Pri) > 0 {
		v.Keystone = b.R.Pri[0]
	}
	return v
}

// build completa da LoLalytics (statistiche su centinaia di migliaia di partite)
func llBuild(champ int, pos, mode, tier string) (BuildResult, bool) {
	if mode != "" && mode != "ranked" && mode != "flex" {
		return BuildResult{}, false
	}
	e, ok := llFind(champ, pos, tier)
	if !ok || e.B == nil {
		return BuildResult{}, false
	}
	b := BuildResult{Champion: champ, Position: ll2pos[e.Lane], Mode: mode, Tier: tier, Patch: e.P, Source: "LoLalytics",
		Games: e.H.N, Matches: e.H.N, WinRate: e.H.WR / 100, PickRate: e.H.PR / 100, BanRate: e.H.BR / 100, TierLabel: e.H.Tier,
		ItemWR: map[int]float64{}}
	tot := 0.0
	for _, v := range e.Lanes {
		tot += v
	}
	for l, v := range e.Lanes {
		if p := ll2pos[l]; p != "" && v > 0 && tot > 0 {
			b.Positions = append(b.Positions, PosShare{Position: p, Share: v / tot, Games: int(float64(e.H.N) * v / 100)})
		}
	}
	sort.Slice(b.Positions, func(i, j int) bool { return b.Positions[i].Share > b.Positions[j].Share })
	p := e.B
	b.Runes = llRunePage(p.R)
	if b.Runes != nil {
		b.RunePages = []*RunePage{b.Runes}
		b.RuneKeystones = []Opt{{Key: fmt.Sprint(p.R.Pri[0]), IDs: []int{p.R.Pri[0]}, Games: p.R.N, WinRate: p.R.WR / 100, Pick: float64(p.R.N) / float64(max(1, e.H.N))}}
	}
	b.Spells, b.SpellsWR = p.Sums, p.SumsWR/100
	if len(p.Sums) == 2 {
		b.SpellSets = []Opt{{Key: fmt.Sprintf("%d_%d", p.Sums[0], p.Sums[1]), IDs: p.Sums, Games: p.SumsN, WinRate: p.SumsWR / 100, Pick: float64(p.SumsN) / float64(max(1, e.H.N))}}
	}
	b.Starting = p.St.Set
	if len(p.St.Set) > 0 {
		b.StartSets = []Opt{{Key: "ll", IDs: p.St.Set, Games: p.St.N, WinRate: p.St.WR / 100, Pick: float64(p.St.N) / float64(max(1, e.H.N))}}
	}
	b.SkillSeq, b.SkillSeqWR, b.SkillSeqN = llSkillSeq(p.SO), p.SOWR/100, p.SON
	if p.SP != "" {
		b.Skills = strings.Join(strings.Split(p.SP, ""), ">")
	}
	if len(b.SkillSeq) >= 3 {
		b.SkillFirst = b.SkillSeq[:3]
		b.SkillOrders = []Opt{{Key: b.SkillSeq, Games: p.SON, WinRate: p.SOWR / 100}}
	}
	v := llVariant(p, "LoLalytics · più giocata")
	b.Core, b.Situational, b.BuildGames = v.Core, v.Situational, p.Core.N
	b.Variants = []BuildVariant{v}
	if e.BW != nil && len(e.BW.Core.Set) > 0 && fmt.Sprint(e.BW.Core.Set) != fmt.Sprint(p.Core.Set) {
		b.Variants = append(b.Variants, llVariant(e.BW, "LoLalytics · più vincente"))
	}
	for _, l := range [][][]float64{p.I4, p.I5, p.I6} {
		for _, o := range l {
			if len(o) >= 3 {
				b.ItemWR[int(o[0])] = o[2] / 100
			}
		}
	}
	// matchup: chi batti e chi ti batte in corsia
	var pairs []Pair
	for _, x := range e.VS {
		if len(x) >= 4 && x[2] >= 50 {
			pairs = append(pairs, Pair{Champion: int(x[0]), WinRate: x[1] / 100, Games: int(x[2]), Delta: x[3] / 100})
		}
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].WinRate > pairs[j].WinRate })
	for i := 0; i < len(pairs) && len(b.Best) < 8; i++ {
		if pairs[i].WinRate > 0.5 {
			b.Best = append(b.Best, pairs[i])
		}
	}
	for i := len(pairs) - 1; i >= 0 && len(b.Worst) < 8; i-- {
		if pairs[i].WinRate < 0.5 {
			b.Worst = append(b.Worst, pairs[i])
		}
	}
	return b, true
}

// partite recenti dei one-trick (onetricks.gg) come "pro build"
func otGames(champ int, pos string) []ProGame {
	ext.mu.RLock()
	e, ok := ext.ot[champ]
	ext.mu.RUnlock()
	if !ok {
		return nil
	}
	roles := map[string]string{"top": "TOP", "jungle": "JUNGLE", "mid": "MIDDLE", "adc": "BOTTOM", "bot": "BOTTOM", "support": "UTILITY", "sup": "UTILITY"}
	var out []ProGame
	for _, g := range e.G {
		p := roles[strings.ToLower(g.Role)]
		if pos != "" && p != "" && p != pos {
			continue
		}
		name, tag := g.N, ""
		if i := strings.Index(g.N, "#"); i >= 0 {
			name, tag = strings.TrimSpace(g.N[:i]), strings.TrimSpace(g.N[i+1:])
		}
		items := g.Ord
		if len(items) == 0 {
			items = g.It
		}
		pg := ProGame{Name: name, Tag: tag, Tier: strings.ToUpper(g.T), Region: strings.ToLower(strings.TrimRight(g.Reg, "0123456789")), Champ: champ,
			Pos: p, Opp: g.Opp, Win: g.W, Items: items, Spells: g.SP, Mode: "ranked", When: g.At, Match: g.ID, Keystone: g.KS, K: g.K, D: g.D, A: g.A}
		if g.KS > 0 {
			pg.Runes = fmt.Sprintf("%d:%d", perkStyle(g.KS), g.Sub)
		}
		out = append(out, pg)
	}
	return out
}

func extStatus() (int, int, string, int64) {
	ext.mu.RLock()
	defer ext.mu.RUnlock()
	n := 0
	for _, m := range ext.ll {
		n += len(m)
	}
	at := int64(0)
	if !ext.at.IsZero() {
		at = ext.at.Unix()
	}
	return n, len(ext.ot), ext.err, at
}
