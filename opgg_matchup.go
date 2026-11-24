package main

// Build adattata al matchup: OP.GG (lol_get_lane_matchup_guide) dà rune, incantesimi, oggetti, stivali
// e abilità di chi gioca il tuo campione contro quell'avversario, più un consiglio per la corsia.

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type MatchupInfo struct {
	Opponent  int     `json:"opponent"`
	Games     int     `json:"games"`
	WinRate   float64 `json:"winRate"`
	Tip       string  `json:"tip,omitempty"`
	Advantage string  `json:"advantage,omitempty"` // EVEN, ME, OPPONENT…
}

type mgItems struct {
	IDs      []int   `json:"ids"`
	Play     int     `json:"play"`
	Win      int     `json:"win"`
	PickRate float64 `json:"pick_rate"`
}

type mgRune struct {
	PrimaryPage   int     `json:"primary_page_id"`
	PrimaryRunes  []int   `json:"primary_rune_ids"`
	SecondaryPage int     `json:"secondary_page_id"`
	Secondary     []int   `json:"secondary_rune_ids"`
	StatMods      []int   `json:"stat_mod_ids"`
	Play          int     `json:"play"`
	Win           int     `json:"win"`
	PickRate      float64 `json:"pick_rate"`
}

type mgSkills struct {
	Order    []string `json:"order"`
	IDs      []string `json:"ids"`
	Play     int      `json:"play"`
	Win      int      `json:"win"`
	PickRate float64  `json:"pick_rate"`
}

type matchupGuide struct {
	Spells   []mgItems  `json:"summoner_spells"`
	Core     []mgItems  `json:"core_items"`
	Boots    []mgItems  `json:"boots"`
	Starter  []mgItems  `json:"starter_items"`
	Last     []mgItems  `json:"last_items"`
	Runes    []mgRune   `json:"runes"`
	Skills   []mgSkills `json:"skills"`
	Mastery  []mgSkills `json:"skill_masteries"`
	Tip      string     `json:"opponent_champion_tip"`
	Adv      string     `json:"lane_advantage_champion"`
	at       time.Time
	notFound bool
}

var (
	mgMu    sync.Mutex
	mgCache = map[string]*matchupGuide{}
)

func opggMatchupGuide(me, opp int, pos string) (*matchupGuide, error) {
	k := fmt.Sprintf("%d|%d|%s", me, opp, pos)
	mgMu.Lock()
	if g, ok := mgCache[k]; ok && time.Since(g.at) < 12*time.Hour {
		mgMu.Unlock()
		if g.notFound {
			return nil, fmt.Errorf("nessun dato")
		}
		return g, nil
	}
	mgMu.Unlock()
	if err := opgg.ensureChamps(); err != nil {
		return nil, err
	}
	opgg.mu.Lock()
	a, b := opgg.keys[me], opgg.keys[opp]
	opgg.mu.Unlock()
	p := pos2opgg[strings.ToUpper(pos)]
	if a == "" || b == "" || p == "" {
		return nil, fmt.Errorf("matchup non valido")
	}
	txt, err := opgg.callRaw("lol_get_lane_matchup_guide", map[string]any{"position": p, "my_champion": a, "opponent_champion": b})
	if err != nil {
		return nil, err
	}
	var r struct {
		Data matchupGuide `json:"data"`
	}
	if err := json.Unmarshal([]byte(txt), &r); err != nil {
		return nil, err
	}
	g := &r.Data
	g.at = time.Now()
	g.notFound = len(g.Spells) == 0
	mgMu.Lock()
	mgCache[k] = g
	mgMu.Unlock()
	if g.notFound {
		return nil, fmt.Errorf("nessun dato")
	}
	// vale anche per i pick consigliati "contro l'avversario"
	n, w := 0, 0
	for _, s := range g.Spells {
		n += s.Play
		w += s.Win
	}
	sugMu.Lock()
	matchupWL[k] = wl{N: n, W: w, At: time.Now()}
	sugMu.Unlock()
	return g, nil
}

func mgOpt(x mgItems) Opt {
	o := Opt{IDs: x.IDs, Games: x.Play, Pick: x.PickRate}
	if x.Play > 0 {
		o.WinRate = float64(x.Win) / float64(x.Play)
	}
	var s []string
	for _, id := range x.IDs {
		s = append(s, strconv.Itoa(id))
	}
	o.Key = strings.Join(s, ",")
	return o
}

// mette davanti a tutto la build di chi gioca questo matchup
func applyMatchup(b BuildResult, opp int) BuildResult {
	if opp <= 0 || b.Champion == 0 || b.Position == "" || b.Mode != "ranked" && b.Mode != "flex" && b.Mode != "" {
		return b
	}
	g, err := opggMatchupGuide(b.Champion, opp, b.Position)
	if err != nil {
		return b
	}
	info := &MatchupInfo{Opponent: opp, Tip: strings.TrimSpace(g.Tip), Advantage: g.Adv}
	for _, s := range g.Spells {
		info.Games += s.Play
		info.WinRate += float64(s.Win)
	}
	if info.Games > 0 {
		info.WinRate /= float64(info.Games)
	}
	b.Matchup = info
	iw := map[int]float64{} // copia: la mappa originale è nella cache delle build
	for k, v := range b.ItemWR {
		iw[k] = v
	}
	b.ItemWR = iw
	if info.Games < 80 { // troppo poche partite per fidarsi della build
		return b
	}
	name := champName(opp)
	min := int(math.Max(25, float64(info.Games)*0.02))
	// incantesimi, oggetti iniziali, stivali
	var spells, starts, boots []Opt
	for _, x := range g.Spells {
		if x.Play >= min && len(x.IDs) == 2 {
			spells = append(spells, mgOpt(x))
		}
	}
	for _, x := range g.Starter {
		if x.Play >= min {
			starts = append(starts, mgOpt(x))
		}
	}
	for _, x := range g.Boots {
		if x.Play >= min && len(x.IDs) == 1 {
			boots = append(boots, mgOpt(x))
		}
	}
	if len(spells) > 0 {
		b.Spells, b.SpellsWR = spells[0].IDs, spells[0].WinRate
		b.SpellSets = mergeOptsList(spells, b.SpellSets, 4)
	}
	if len(starts) > 0 {
		seen := map[int]bool{}
		var u []int
		for _, id := range starts[0].IDs {
			if !seen[id] {
				u = append(u, id)
				seen[id] = true
			}
		}
		b.Starting = u
		b.StartSets = mergeOptsList(starts, b.StartSets, 4)
	}
	bootID := 0
	if len(boots) > 0 {
		bootID = boots[0].IDs[0]
		b.BootsList = mergeOptsList(boots, b.BootsList, 5)
	}
	// rune: pagine intere giocate in questo matchup
	var pages []*RunePage
	var ks []Opt
	for _, x := range g.Runes {
		perks := append(append(append([]int{}, x.PrimaryRunes...), x.Secondary...), x.StatMods...)
		if len(perks) != 9 || x.Play < min {
			continue
		}
		rp := &RunePage{Primary: x.PrimaryPage, Secondary: x.SecondaryPage, Perks: perks, Games: x.Play, Source: "vs " + name}
		rp.WinRate = float64(x.Win) / float64(x.Play)
		pages = append(pages, rp)
		ks = append(ks, Opt{Key: strconv.Itoa(perks[0]), IDs: []int{perks[0]}, Games: x.Play, WinRate: rp.WinRate, Pick: math.Max(0.01, x.PickRate)})
	}
	if len(pages) > 0 {
		b.Runes = pages[0]
		rest := []*RunePage{}
		for _, p := range b.RunePages {
			dup := false
			for _, q := range pages {
				if p != nil && fmt.Sprint(p.Perks) == fmt.Sprint(q.Perks) {
					dup = true
				}
			}
			if !dup {
				rest = append(rest, p)
			}
		}
		b.RunePages = append(pages, rest...)
		b.RuneKeystones = mergeOptsList(ks, b.RuneKeystones, 6)
	}
	// abilità
	if len(g.Skills) > 0 && len(g.Skills[0].Order) > 0 && g.Skills[0].Play >= min {
		s := g.Skills[0]
		b.SkillSeq, b.SkillSeqN = strings.Join(s.Order, ""), s.Play
		b.SkillSeqWR = float64(s.Win) / float64(s.Play)
		if len(b.SkillSeq) >= 3 {
			b.SkillFirst = b.SkillSeq[:3]
		}
	}
	if len(g.Mastery) > 0 && len(g.Mastery[0].IDs) > 0 {
		b.Skills = strings.Join(g.Mastery[0].IDs, ">")
	}
	// build: i core più giocati contro questo avversario diventano le prime schede
	var last []int
	sort.SliceStable(g.Last, func(i, j int) bool { return g.Last[i].Play > g.Last[j].Play })
	for _, x := range g.Last {
		if len(x.IDs) == 1 && x.IDs[0] != bootID && x.Play >= min {
			last = append(last, x.IDs[0])
			o := mgOpt(x)
			b.ItemWR[x.IDs[0]] = o.WinRate
		}
	}
	var vs []BuildVariant
	for i, x := range g.Core {
		if i >= 3 || x.Play < min || len(x.IDs) == 0 {
			continue
		}
		o := mgOpt(x)
		in := map[int]bool{}
		for _, id := range x.IDs {
			in[id] = true
			if _, ok := b.ItemWR[id]; !ok {
				b.ItemWR[id] = o.WinRate
			}
		}
		var sit []int
		for _, id := range last {
			if !in[id] {
				sit = append(sit, id)
			}
		}
		if len(sit) > 6 {
			sit = sit[:6]
		}
		label := "vs " + name
		if i > 0 {
			label = fmt.Sprintf("vs %s · %d", name, i+1)
		}
		v := BuildVariant{Label: label, Path: x.IDs, Boots: bootID, Games: x.Play, WinRate: o.WinRate, Share: math.Min(1, x.PickRate),
			Core: x.IDs, Situational: sit, Runes: b.Runes}
		if b.Runes != nil {
			v.Keystone = b.Runes.Perks[0]
		}
		vs = append(vs, v)
	}
	if len(vs) > 0 {
		b.Core, b.Situational = vs[0].Core, vs[0].Situational
		var rest []BuildVariant
		for _, v := range b.Variants {
			if len(rest)+len(vs) >= 7 {
				break
			}
			rest = append(rest, v)
		}
		b.Variants = append(vs, rest...)
	}
	return b
}

func mergeOptsList(a, b []Opt, max int) []Opt {
	out := append([]Opt{}, a...)
	for _, x := range b {
		dup := false
		for _, y := range out {
			if x.Key == y.Key {
				dup = true
			}
		}
		if !dup && len(out) < max {
			out = append(out, x)
		}
	}
	if len(out) > max {
		out = out[:max]
	}
	return out
}
