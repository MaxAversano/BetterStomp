package main

import (
	"errors"
	"path/filepath"
	"strconv"
	"sync"
)

// ---------- Build personali (slot 5 - 10 - 10) ----------

type PersonalBuild struct {
	Starting    []int     `json:"starting"`
	Core        []int     `json:"core"`
	Situational []int     `json:"situational"`
	Runes       *RunePage `json:"runes,omitempty"`
	Spells      []int     `json:"spells,omitempty"`
}

var (
	buildsMu sync.RWMutex
	personal = map[string]PersonalBuild{}
)

func buildsPath() string { return filepath.Join(dataDir, "builds.json") }

func loadPersonal() {
	buildsMu.Lock()
	defer buildsMu.Unlock()
	readJSON(buildsPath(), &personal)
	if personal == nil {
		personal = map[string]PersonalBuild{}
	}
}

func getPersonal(champ int) (PersonalBuild, bool) {
	buildsMu.RLock()
	defer buildsMu.RUnlock()
	b, ok := personal[strconv.Itoa(champ)]
	return b, ok
}

func clip(a []int, n int) []int {
	var out []int
	for _, x := range a {
		if x > 0 {
			out = append(out, x)
		}
	}
	if len(out) > n {
		out = out[:n]
	}
	return out
}

func setPersonal(champ int, b PersonalBuild) error {
	b.Starting, b.Core, b.Situational = clip(b.Starting, 5), clip(b.Core, 10), clip(b.Situational, 10)
	buildsMu.Lock()
	if len(b.Starting)+len(b.Core)+len(b.Situational) == 0 && b.Runes == nil {
		delete(personal, strconv.Itoa(champ))
	} else {
		personal[strconv.Itoa(champ)] = b
	}
	cp := map[string]PersonalBuild{}
	for k, v := range personal {
		cp[k] = v
	}
	buildsMu.Unlock()
	return writeJSON(buildsPath(), cp)
}

// build finale: statistiche + build personale + rune consigliate dal client
func resolveBuild(champ int, pos, mode, tier string, usePersonal bool, variant int) BuildResult {
	if mode == "" {
		mode = "ranked"
	}
	if tier == "" || tier == "auto" {
		tier = autoTier()
	}
	b, err := BuildResult{}, errors.New("")
	var otp []ProGame
	otpPending := false
	if !getSettings().OpggOff {
		otp, _ = opggOTPCached(champ) // le partite mancanti arrivano in sottofondo
		otpPending = len(otp) == 0
	}
	if !getSettings().OpggOff {
		b, err = opggBuild(champ, pos, mode, tier) // dati OP.GG, il nostro motore resta di riserva
	}
	if err != nil {
		b = engine.ChampionStats(champ, mode, tier, pos)
	} else {
		b = mergeDetected(b, engine.ChampionStats(champ, mode, tier, b.Position))
	}
	if b.Position == "" {
		b.Position = pos
	}
	b.OtpPending = otpPending
	if len(otp) > 0 {
		var mine []ProGame
		for _, g := range otp {
			if b.Position == "" || g.Pos == "" || g.Pos == b.Position {
				mine = append(mine, g)
			}
		}
		if len(mine) == 0 {
			mine = otp
		}
		if len(mine) > 12 {
			mine = mine[:12]
		}
		b.ProGames = append(fillOTPRunes(mine, b.RunePages), b.ProGames...)
	}
	if variant > 0 && variant < len(b.Variants) {
		b.Variant = variant
		b.Core, b.Situational = b.Variants[variant].Core, b.Variants[variant].Situational
	}
	if variant >= 0 && variant < len(b.Variants) && b.Variants[variant].Runes != nil {
		b.Runes = b.Variants[variant].Runes // ogni build ha la sua pagina di rune
	}
	if pb, ok := getPersonal(champ); ok && usePersonal {
		pref := getSettings().PreferPersonal
		if pref || len(b.Core) == 0 {
			if len(pb.Starting)+len(pb.Core)+len(pb.Situational) > 0 {
				b.Starting, b.Core, b.Situational = pb.Starting, pb.Core, pb.Situational
				b.Personal = true
			}
		}
		if pb.Runes != nil && (pref || b.Runes == nil) {
			b.Runes = pb.Runes
		}
		if len(pb.Spells) == 2 && (pref || len(b.Spells) == 0) {
			b.Spells = pb.Spells
		}
	}
	if b.Runes == nil {
		if r := lcuRecommendedRunes(champ, b.Position); r != nil {
			b.Runes = r
			if len(b.Spells) == 0 && len(r.Spells) == 2 {
				b.Spells = r.Spells
			}
		}
	}
	return b
}

// rango predefinito: quello dell'utente (dal client), altrimenti Smeraldo+
func autoTier() string {
	s := getSettings()
	if s.DefaultTier != "" && s.DefaultTier != "auto" {
		return s.DefaultTier
	}
	if rq, ok := getClientState().Ranked["RANKED_SOLO_5x5"]; ok && rq.Tier != "" && rq.Tier != "NONE" {
		return rq.Tier
	}
	return "EMERALD+"
}

// OP.GG come base, le partite rilevate dal nostro server aggiungono le alternative
// (altre build, pagine di rune, stivali, oggetti, incantesimi) e i dettagli che OP.GG non dà
func mergeDetected(o, e BuildResult) BuildResult {
	r := o
	sameInts := func(a, b []int, n int) bool {
		if len(a) < n || len(b) < n {
			n = min(len(a), len(b))
			if len(a) != len(b) {
				return false
			}
		}
		for i := 0; i < n; i++ {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	r.Variants = append([]BuildVariant{}, o.Variants...)
	for _, v := range e.Variants {
		dup := false
		for _, x := range r.Variants {
			if sameInts(x.Path, v.Path, 3) {
				dup = true
			}
		}
		if !dup && len(r.Variants) < 6 {
			if v.Label == "" {
				v.Label = "Rilevata"
			}
			r.Variants = append(r.Variants, v)
		}
	}
	r.RunePages = append([]*RunePage{}, o.RunePages...)
	for _, p := range e.RunePages {
		dup := false
		for _, x := range r.RunePages {
			if sameInts(x.Perks, p.Perks, 9) {
				dup = true
			}
		}
		if !dup {
			r.RunePages = append(r.RunePages, p)
		}
	}
	mergeOpts := func(a, b []Opt, max int) []Opt {
		out := append([]Opt{}, a...)
		for _, x := range b {
			dup := false
			for _, y := range out {
				if (len(x.IDs) > 0 && sameInts(x.IDs, y.IDs, len(x.IDs))) || (len(x.IDs) == 0 && x.Key == y.Key) {
					dup = true
				}
			}
			if !dup && len(out) < max {
				out = append(out, x)
			}
		}
		return out
	}
	r.RuneKeystones = mergeOpts(o.RuneKeystones, e.RuneKeystones, 6)
	r.StartSets = mergeOpts(o.StartSets, e.StartSets, 4)
	r.SpellSets = mergeOpts(o.SpellSets, e.SpellSets, 4)
	r.BootsList = mergeOpts(o.BootsList, e.BootsList, 5)
	r.ItemsList = mergeOpts(o.ItemsList, e.ItemsList, 16)
	r.SkillOrders = mergeOpts(o.SkillOrders, e.SkillOrders, 3)
	r.ItemWR = map[int]float64{}
	for k, v := range e.ItemWR {
		r.ItemWR[k] = v
	}
	for k, v := range o.ItemWR {
		r.ItemWR[k] = v
	}
	mergePairs := func(a, b []Pair) []Pair {
		out := append([]Pair{}, a...)
		for _, x := range b {
			dup := false
			for _, y := range out {
				if y.Champion == x.Champion {
					dup = true
				}
			}
			if !dup && x.Games >= 20 && len(out) < 8 {
				out = append(out, x)
			}
		}
		return out
	}
	r.Best, r.Worst = mergePairs(o.Best, e.Best), mergePairs(o.Worst, e.Worst)
	if len(r.Synergy) == 0 {
		r.Synergy = e.Synergy
	}
	r.ProGames, r.KDA, r.CSMin = e.ProGames, e.KDA, e.CSMin
	if r.SkillSeq == "" {
		r.SkillSeq, r.SkillSeqN, r.SkillSeqWR = e.SkillSeq, e.SkillSeqN, e.SkillSeqWR
	}
	if len(e.Variants) > 0 || len(e.RunePages) > 0 {
		r.Source = "OP.GG+"
	}
	return r
}
