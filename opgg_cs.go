package main

// Pick consigliati in champion select, divisi in tre: contro l'avversario di corsia (matchup OP.GG),
// per la composizione (sinergie OP.GG con gli alleati già scelti) e i più forti del ruolo (tier list).

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type CSAlly struct {
	Champ int    `json:"champ"`
	Pos   string `json:"pos"`
}

type CSSuggestReq struct {
	Pos      string   `json:"pos"`
	Tier     string   `json:"tier"`
	Opponent int      `json:"opponent"`
	Allies   []CSAlly `json:"allies"`
	Exclude  []int    `json:"exclude"`
}

type CSPick struct {
	Champion int     `json:"champion"`
	WinRate  float64 `json:"winRate"`
	Games    int     `json:"games"`
	Score    float64 `json:"score"`
	Tier     string  `json:"tier,omitempty"`
	Detail   string  `json:"detail"`
	With     []int   `json:"with,omitempty"` // alleati con cui funziona bene
}

type CSSuggest struct {
	Matchup []CSPick `json:"matchup"`
	Comp    []CSPick `json:"comp"`
	Meta    []CSPick `json:"meta"`
}

func snakeName(n string) string {
	n = strings.ToUpper(n)
	var b strings.Builder
	for _, r := range n {
		switch {
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		case r == ' ' || r == '&':
			if s := b.String(); s != "" && !strings.HasSuffix(s, "_") {
				b.WriteRune('_')
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

type wl struct {
	N, W int
	At   time.Time
}

var (
	sugMu      sync.Mutex
	matchupWL = map[string]wl{}
	synergyWL = map[string]map[int]wl{}
	synergyAt = map[string]time.Time{}
)

// partite e vittorie di "me" contro "opp" in corsia (tutti i ranghi)
func opggMatchup(me, opp int, pos string) (wl, bool) {
	k := fmt.Sprintf("%d|%d|%s", me, opp, pos)
	sugMu.Lock()
	if v, ok := matchupWL[k]; ok && time.Since(v.At) < 12*time.Hour {
		sugMu.Unlock()
		return v, true
	}
	sugMu.Unlock()
	opgg.mu.Lock()
	a, b := opgg.keys[me], opgg.keys[opp]
	opgg.mu.Unlock()
	if a == "" || b == "" {
		return wl{}, false
	}
	v, err := opgg.callRaw("lol_get_lane_matchup_guide", map[string]any{"position": pos2opgg[pos], "my_champion": a, "opponent_champion": b})
	if err != nil {
		return wl{}, false
	}
	var r struct {
		Data struct {
			Spells []struct {
				Play int `json:"play"`
				Win  int `json:"win"`
			} `json:"summoner_spells"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(v), &r) != nil {
		return wl{}, false
	}
	res := wl{At: time.Now()}
	for _, s := range r.Data.Spells {
		res.N += s.Play
		res.W += s.Win
	}
	sugMu.Lock()
	matchupWL[k] = res
	sugMu.Unlock()
	return res, true
}

// campioni del mio ruolo che vanno bene con un alleato (i 10 più giocati insieme)
func opggSynergy(ally int, allyPos, myPos string) map[int]wl {
	k := fmt.Sprintf("%d|%s|%s", ally, allyPos, myPos)
	sugMu.Lock()
	if m, ok := synergyWL[k]; ok && time.Since(synergyAt[k]) < 12*time.Hour {
		sugMu.Unlock()
		return m
	}
	sugMu.Unlock()
	opgg.mu.Lock()
	names := []string{opgg.snake[ally], opgg.keys[ally]}
	opgg.mu.Unlock()
	out := map[int]wl{}
	for _, n := range names {
		if n == "" {
			continue
		}
		v, err := opgg.call("lol_get_champion_synergies", map[string]any{"champion": n, "my_position": pos2opgg[allyPos],
			"synergy_position": pos2opgg[myPos], "desired_output_fields": []string{"data.synergies[].{synergy_champion_id,play,win}"}})
		if err != nil {
			continue
		}
		for _, x := range ogl(v, "data", "synergies") {
			out[ogi(x, "synergy_champion_id")] = wl{N: ogi(x, "play"), W: ogi(x, "win")}
		}
		break
	}
	sugMu.Lock()
	synergyWL[k], synergyAt[k] = out, time.Now()
	sugMu.Unlock()
	return out
}

func csSuggest(q CSSuggestReq) CSSuggest {
	var res CSSuggest
	pos := strings.ToUpper(q.Pos)
	if pos == "MID" {
		pos = "MIDDLE"
	}
	if _, ok := pos2opgg[pos]; !ok {
		return res
	}
	if err := opgg.ensureChamps(); err != nil {
		return res
	}
	ex := map[int]bool{}
	for _, c := range q.Exclude {
		ex[c] = true
	}
	rows, _, err := opggTierList("ranked", q.Tier, pos)
	if err != nil {
		return res
	}
	base := map[int]TierRow{}
	var cands []TierRow
	for _, r := range rows {
		base[r.Champion] = r
		if !ex[r.Champion] {
			cands = append(cands, r)
		}
	}
	for i, r := range cands {
		if i >= 8 {
			break
		}
		res.Meta = append(res.Meta, CSPick{Champion: r.Champion, WinRate: r.WinRate, Games: r.Games, Score: r.Score, Tier: r.Tier,
			Detail: fmt.Sprintf("Tier %s · %.1f%% · pick %.1f%%", r.Tier, r.WinRate*100, r.PickRate*100)})
	}
	if len(cands) > 30 {
		cands = cands[:30]
	}
	var wg sync.WaitGroup
	// contro l'avversario di corsia
	if q.Opponent > 0 {
		var mu sync.Mutex
		sem := make(chan struct{}, 8)
		for _, c := range cands {
			c := c
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				m, ok := opggMatchup(c.Champion, q.Opponent, pos)
				if !ok || m.N < 150 {
					return
				}
				wr := float64(m.W) / float64(m.N)
				sm := (float64(m.W) + 0.5*300) / (float64(m.N) + 300) // poche partite: verso il 50%
				mu.Lock()
				res.Matchup = append(res.Matchup, CSPick{Champion: c.Champion, WinRate: wr, Games: m.N, Score: sm, Tier: c.Tier,
					Detail: fmt.Sprintf("%.1f%% contro %s · %s partite", wr*100, champName(q.Opponent), fmtThousands(m.N))})
				mu.Unlock()
			}()
		}
	}
	// per la composizione: sinergie con gli alleati già scelti
	syn := map[int][]wl{}
	withs := map[int][]int{}
	var smu sync.Mutex
	nAllies := 0
	for _, a := range q.Allies {
		ap := strings.ToUpper(a.Pos)
		if a.Champ == 0 || ap == pos {
			continue
		}
		if _, ok := pos2opgg[ap]; !ok {
			ap = opgg.mainPos(a.Champ)
		}
		if ap == "" || ap == pos {
			continue
		}
		nAllies++
		a, ap := a, ap
		wg.Add(1)
		go func() {
			defer wg.Done()
			for c, v := range opggSynergy(a.Champ, ap, pos) {
				if v.N < 200 {
					continue
				}
				smu.Lock()
				syn[c] = append(syn[c], v)
				if float64(v.W)/float64(v.N) >= 0.5 {
					withs[c] = append(withs[c], a.Champ)
				}
				smu.Unlock()
			}
		}()
	}
	wg.Wait()
	sort.Slice(res.Matchup, func(i, j int) bool { return res.Matchup[i].Score > res.Matchup[j].Score })
	if len(res.Matchup) > 8 {
		res.Matchup = res.Matchup[:8]
	}
	// punteggio composizione: forza nel ruolo + quanto rende con questi alleati
	seen := map[int]bool{}
	add := func(c int) {
		if seen[c] || ex[c] {
			return
		}
		seen[c] = true
		b, ok := base[c]
		if !ok {
			return
		}
		delta, n := 0.0, 0
		tw, tn := 0, 0
		for _, v := range syn[c] {
			delta += (float64(v.W)+b.WinRate*200)/(float64(v.N)+200) - b.WinRate
			tw += v.W
			tn += v.N
			n++
		}
		if n > 0 {
			delta /= float64(n)
		}
		p := CSPick{Champion: c, WinRate: b.WinRate, Games: b.Games, Tier: b.Tier, With: withs[c],
			Score: b.WinRate + 2*delta + 0.004*float64(n)}
		if tn > 0 {
			p.Detail = fmt.Sprintf("%.1f%% con i tuoi alleati · %s partite insieme", 100*float64(tw)/float64(tn), fmtThousands(tn))
		} else {
			p.Detail = fmt.Sprintf("Tier %s · %.1f%% nel ruolo", b.Tier, b.WinRate*100)
		}
		res.Comp = append(res.Comp, p)
	}
	for c := range syn {
		add(c)
	}
	for _, c := range cands {
		add(c.Champion)
	}
	sort.Slice(res.Comp, func(i, j int) bool { return res.Comp[i].Score > res.Comp[j].Score })
	if len(res.Comp) > 14 { // il ritocco per i danni lo fa la pagina, che conosce i campioni
		res.Comp = res.Comp[:14]
	}
	_ = nAllies
	return res
}

func fmtThousands(n int) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "." + s[i:]
	}
	return s
}
