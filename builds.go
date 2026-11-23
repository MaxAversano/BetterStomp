package main

import (
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
	b := engine.ChampionStats(champ, mode, tier, pos)
	if b.Position == "" {
		b.Position = pos
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
