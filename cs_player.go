package main

// Champion select: scheda di un giocatore (stile League of Graphs) con maestrie,
// ultime partite e punti forti / deboli calcolati dalla sua cronologia.

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

type csGame struct {
	Champ  int    `json:"champion"`
	Win    bool   `json:"win"`
	K      int    `json:"k"`
	D      int    `json:"d"`
	A      int    `json:"a"`
	CS     int    `json:"cs"`
	Gold   int    `json:"gold"`
	Vision int    `json:"vision"`
	Damage int    `json:"damage"`
	Dur    int    `json:"dur"` // secondi
	When   int64  `json:"when"`
	Queue  int    `json:"queue"`
	Role   string `json:"role"` // TOP JUNGLE MIDDLE BOTTOM UTILITY ("" fuori dalla Landa)
	Items  []int  `json:"items"`
}

func csGameFrom(g lcuGame) csGame {
	p := g.Participants[0]
	st := p.Stats
	dur := g.GameDuration
	if dur > 100000 {
		dur /= 1000
	}
	x := csGame{Champ: p.ChampionID, Win: st.Win, K: st.Kills, D: st.Deaths, A: st.Assists, CS: st.Minions + st.Neutral,
		Gold: st.Gold, Vision: st.Vision, Damage: st.Damage, Dur: dur, When: g.GameCreation, Queue: g.QueueID}
	for _, it := range []int{st.Item0, st.Item1, st.Item2, st.Item3, st.Item4, st.Item5} {
		if it > 0 {
			x.Items = append(x.Items, it)
		}
	}
	if srQueue(g.QueueID, g.GameMode) {
		lane, role := strings.ToUpper(p.Timeline.Lane), strings.ToUpper(p.Timeline.Role)
		mins := math.Max(1, float64(dur)/60)
		switch {
		case strings.Contains(role, "SUPPORT"):
			x.Role = "UTILITY"
		case lane == "JUNGLE" || float64(st.Neutral)/mins > 3.5:
			x.Role = "JUNGLE"
		case lane == "TOP":
			x.Role = "TOP"
		case lane == "MIDDLE" || lane == "MID":
			x.Role = "MIDDLE"
		case lane == "BOTTOM" || lane == "BOT":
			x.Role = "BOTTOM"
		}
		if x.Role != "JUNGLE" && float64(x.CS)/mins < 2.2 {
			x.Role = "UTILITY"
		}
		if x.Role == "" {
			x.Role = "MIDDLE"
		}
	}
	return x
}

// partite sulla Landa (classificate, normali, rapide): le uniche buone per i confronti
func srQueue(q int, mode string) bool {
	switch q {
	case 400, 420, 430, 440, 480, 490, 700:
		return true
	}
	return q == 0 && mode == "CLASSIC"
}

type CSMastery struct {
	Champ  int   `json:"champion"`
	Level  int   `json:"level"`
	Points int   `json:"points"`
	Last   int64 `json:"last"`
}

type CSPoint struct {
	Text   string `json:"text"`
	Detail string `json:"detail"`
}

type CSPlayerSheet struct {
	Name      string      `json:"name"`
	Tag       string      `json:"tag"`
	Level     int         `json:"level"`
	Tier      string      `json:"tier"`
	Division  string      `json:"division"`
	LP        int         `json:"lp"`
	Wins      int         `json:"wins"`
	Losses    int         `json:"losses"`
	Champ     int         `json:"champion"`
	ChampM    *CSMastery  `json:"champMastery"`
	Masteries []CSMastery `json:"masteries"`
	MainRole  string      `json:"mainRole"`
	RoleShare float64     `json:"roleShare"`
	Games     []csGame    `json:"games"`
	Strong    []CSPoint   `json:"strong"`
	Weak      []CSPoint   `json:"weak"`
	Stats     struct {
		N, Wins          int
		CSM, KDA, Deaths float64
		VisM, DmgM, GPM  float64
	} `json:"stats"`
}

var (
	masteryMu    sync.Mutex
	masteryCache = map[string]masteryEntry{}
	masteryWay   = "" // quale endpoint ha funzionato (per il log)
)

type masteryEntry struct {
	at   time.Time
	list []CSMastery
}

// maestrie: prima dal client (gratis), poi dalla Riot API se c'è la chiave
func fetchMastery(puuid string, sumID int64) []CSMastery {
	masteryMu.Lock()
	if e, ok := masteryCache[puuid]; ok && time.Since(e.at) < 15*time.Minute {
		masteryMu.Unlock()
		return e.list
	}
	masteryMu.Unlock()
	type raw struct {
		Champ  int   `json:"championId"`
		Level  int   `json:"championLevel"`
		Points int   `json:"championPoints"`
		Last   int64 `json:"lastPlayTime"`
	}
	var list []raw
	way := ""
	try := func(name string, get func() error) {
		if len(list) > 0 {
			return
		}
		list = nil
		if get() == nil && len(list) > 0 {
			way = name
		}
	}
	try("lcu puuid", func() error { return lcu.getJSON("/lol-champion-mastery/v1/"+puuid+"/champion-mastery", &list) })
	if sumID > 0 {
		try("lcu collections", func() error {
			return lcu.getJSON(fmt.Sprintf("/lol-collections/v1/inventories/%d/champion-mastery", sumID), &list)
		})
	}
	if strings.TrimSpace(getSettings().RiotAPIKey) != "" {
		try("riot api", func() error {
			return riot.get(platHost(getSettings().Region)+"/lol/champion-mastery/v4/champion-masteries/by-puuid/"+puuid, &list)
		})
	}
	if way != masteryWay {
		masteryWay = way
		log.Printf("champion select: maestrie lette da %q", way)
	}
	out := make([]CSMastery, 0, len(list))
	for _, r := range list {
		out = append(out, CSMastery{r.Champ, r.Level, r.Points, r.Last})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Points > out[j].Points })
	masteryMu.Lock()
	masteryCache[puuid] = masteryEntry{time.Now(), out}
	masteryMu.Unlock()
	return out
}

func fmtK(n int) string {
	switch {
	case n >= 1000000:
		return strings.Replace(fmt.Sprintf("%.1fM", float64(n)/1e6), ".", ",", 1)
	case n >= 1000:
		return fmt.Sprintf("%dk", n/1000)
	}
	return fmt.Sprint(n)
}

func f1it(v float64) string { return strings.Replace(fmt.Sprintf("%.1f", v), ".", ",", 1) }

func playerSheet(puuid string, champ int, pos string) CSPlayerSheet {
	d := csPlayerData(puuid)
	sh := CSPlayerSheet{Name: d.name, Tag: d.tag, Level: d.level, Tier: d.rank.Tier, Division: d.rank.Division, LP: d.rank.LeaguePoints,
		Wins: d.rank.Wins, Losses: d.rank.Losses, Champ: champ, Games: d.games}
	ms := fetchMastery(puuid, d.sumID)
	for i := range ms {
		if ms[i].Champ == champ && champ > 0 {
			m := ms[i]
			sh.ChampM = &m
		}
	}
	if len(ms) > 6 {
		sh.Masteries = ms[:6]
	} else {
		sh.Masteries = ms
	}
	tier := d.rank.Tier
	if tier == "" || tier == "NONE" {
		tier = "GOLD"
	}
	strong := func(t, det string) { sh.Strong = append(sh.Strong, CSPoint{t, det}) }
	weak := func(t, det string) { sh.Weak = append(sh.Weak, CSPoint{t, det}) }

	// solo partite sulla Landa per i confronti
	var sr []csGame
	roles := map[string]int{}
	for _, g := range d.games {
		if g.Role != "" && g.Dur >= 600 {
			sr = append(sr, g)
			roles[g.Role]++
		}
	}
	for r, n := range roles {
		if n > roles[sh.MainRole] || n == roles[sh.MainRole] && r < sh.MainRole {
			sh.MainRole = r
		}
	}
	if len(sr) > 0 {
		sh.RoleShare = float64(roles[sh.MainRole]) / float64(len(sr))
	}
	st := &sh.Stats
	st.N = len(sr)
	if len(sr) >= 5 {
		var mins, csMins, csBench, gpmBench, kdaBench, sK, sD, sA, cs, gold, dmg, cMins, vis, vMins, svis, sMins float64
		benchCache := map[string]Bench{}
		for _, g := range sr {
			m := float64(g.Dur) / 60
			b, ok := benchCache[g.Role]
			if !ok {
				b = engine.Benchmarks(0, g.Role, tier)
				benchCache[g.Role] = b
			}
			mins += m
			sK, sD, sA = sK+float64(g.K), sD+float64(g.D), sA+float64(g.A)
			kdaBench += b.KDA
			if g.Win {
				st.Wins++
			}
			if g.Role == "UTILITY" {
				svis += float64(g.Vision)
				sMins += m
				continue
			}
			cs += float64(g.CS)
			csMins += m
			csBench += b.CSM * m
			gold += float64(g.Gold)
			gpmBench += b.GPM * m
			dmg += float64(g.Damage)
			cMins += m
			vis += float64(g.Vision)
			vMins += m
		}
		n := float64(len(sr))
		st.KDA = (sK + sA) / math.Max(1, sD)
		st.Deaths = sD / n
		kdaRef := kdaBench / n
		if csMins >= 30 {
			st.CSM, st.GPM, st.DmgM, st.VisM = cs/csMins, gold/cMins, dmg/cMins, vis/vMins
			ref := csBench / csMins
			switch r := st.CSM / ref; {
			case r >= 1.1:
				strong("Farma bene", fmt.Sprintf("%s CS/min (media del suo elo %s)", f1it(st.CSM), f1it(ref)))
			case r <= 0.85:
				weak("Farma poco", fmt.Sprintf("%s CS/min (media del suo elo %s)", f1it(st.CSM), f1it(ref)))
			}
			gref := gpmBench / csMins
			if st.GPM >= gref*1.1 {
				strong("Accumula tanto oro", fmt.Sprintf("%.0f oro/min", st.GPM))
			} else if st.GPM <= gref*0.88 {
				weak("Poco oro", fmt.Sprintf("%.0f oro/min (media %.0f)", st.GPM, gref))
			}
			if st.DmgM >= 900 {
				strong("Tanti danni ai campioni", fmt.Sprintf("%.0f danni/min", st.DmgM))
			} else if st.DmgM <= 450 {
				weak("Pochi danni ai campioni", fmt.Sprintf("%.0f danni/min", st.DmgM))
			}
			if st.VisM >= 1.1 {
				strong("Buona visione", fmt.Sprintf("%s punti visione/min", f1it(st.VisM)))
			} else if st.VisM <= 0.55 {
				weak("Poca visione", fmt.Sprintf("%s punti visione/min", f1it(st.VisM)))
			}
		}
		if sMins >= 30 {
			v := svis / sMins
			if v >= 2.2 {
				strong("Visione da support ottima", fmt.Sprintf("%s punti visione/min", f1it(v)))
			} else if v <= 1.4 {
				weak("Visione da support scarsa", fmt.Sprintf("%s punti visione/min", f1it(v)))
			}
			if csMins < 30 {
				st.VisM = v
			}
		}
		if st.Deaths <= 4 {
			strong("Muore poco", fmt.Sprintf("%s morti a partita", f1it(st.Deaths)))
		} else if st.Deaths >= 7 {
			weak("Muore spesso", fmt.Sprintf("%s morti a partita", f1it(st.Deaths)))
		}
		if st.KDA >= kdaRef*1.25 {
			strong("KDA alto", fmt.Sprintf("%s (media %s)", f1it(st.KDA), f1it(kdaRef)))
		} else if st.KDA <= kdaRef*0.75 {
			weak("KDA basso", fmt.Sprintf("%s (media %s)", f1it(st.KDA), f1it(kdaRef)))
		}
		if len(sr) >= 8 {
			wr := float64(st.Wins) / n
			if wr >= 0.65 {
				strong("In forma", fmt.Sprintf("%d vittorie nelle ultime %d", st.Wins, len(sr)))
			} else if wr <= 0.35 {
				weak("Periodo no", fmt.Sprintf("%d vittorie nelle ultime %d", st.Wins, len(sr)))
			}
		}
		distinct := map[int]bool{}
		for _, g := range sr {
			distinct[g.Champ] = true
		}
		if len(sr) >= 10 && len(distinct) <= 3 {
			strong("Pool di campioni ristretto", fmt.Sprintf("%d campioni nelle ultime %d partite", len(distinct), len(sr)))
		} else if len(sr) >= 12 && float64(len(distinct)) >= n*0.7 {
			weak("Cambia spesso campione", fmt.Sprintf("%d campioni diversi su %d partite", len(distinct), len(sr)))
		}
		if pos != "" && len(sr) >= 8 && roles[pos] == 0 && sh.MainRole != pos {
			weak("Fuori ruolo", "di solito gioca "+roleIT(sh.MainRole))
		} else if pos != "" && sh.MainRole == pos && sh.RoleShare >= 0.7 {
			strong("Nel suo ruolo", fmt.Sprintf("%.0f%% delle partite in %s", sh.RoleShare*100, roleIT(pos)))
		}
	}
	if g := sh.Wins + sh.Losses; g >= 30 {
		wr := float64(sh.Wins) / float64(g)
		if wr >= 0.55 {
			strong("Stagione positiva", fmt.Sprintf("%.0f%% in Solo/Duo su %d partite", wr*100, g))
		} else if wr <= 0.46 {
			weak("Stagione in salita", fmt.Sprintf("%.0f%% in Solo/Duo su %d partite", wr*100, g))
		}
	}
	if champ > 0 {
		cg, cw := 0, 0
		for _, g := range d.games {
			if g.Champ == champ {
				cg++
				if g.Win {
					cw++
				}
			}
		}
		name := champName(champ)
		pts := 0
		if sh.ChampM != nil {
			pts = sh.ChampM.Points
		}
		switch {
		case pts >= 200000:
			strong("Esperto di "+name, fmtK(pts)+" punti maestria")
		case len(ms) > 0 && pts < 15000:
			weak("Poca esperienza con "+name, fmtK(pts)+" punti maestria")
		}
		if cg >= 3 {
			det := fmt.Sprintf("%dV %dS nelle ultime partite", cw, cg-cw)
			if float64(cw)/float64(cg) >= 0.6 {
				strong("Vince con "+name, det)
			} else if float64(cw)/float64(cg) <= 0.34 {
				weak("Perde con "+name, det)
			}
		}
	}
	return sh
}

func roleIT(r string) string {
	return map[string]string{"TOP": "Top", "JUNGLE": "Jungle", "MIDDLE": "Mid", "BOTTOM": "ADC", "UTILITY": "Support"}[r]
}
