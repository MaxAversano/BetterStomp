package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// ---------- Profilo: partite, ruoli, maestrie, premade, analisi ----------

type PPart struct {
	Puuid  string `json:"puuid"`
	Name   string `json:"name"`
	Tag    string `json:"tag"`
	Champ  int    `json:"champ"`
	Team   int    `json:"team"`
	Pos    string `json:"pos"`
	K      int    `json:"k"`
	D      int    `json:"d"`
	A      int    `json:"a"`
	CS     int    `json:"cs"`
	Gold   int    `json:"gold"`
	Dmg    int    `json:"dmg"`
	Vision int    `json:"vision"`
	Items  []int  `json:"items"`
	Spells []int  `json:"spells"`
	Level  int    `json:"level"`
	Win    bool   `json:"win"`
	Place  int    `json:"place,omitempty"`
}

type Insight struct {
	Kind string `json:"kind"` // good | bad | info
	Text string `json:"text"`
}

type PMatch struct {
	ID       string    `json:"id"`
	Queue    int       `json:"queue"`
	Mode     string    `json:"mode"`
	Start    int64     `json:"start"`
	Duration int       `json:"duration"`
	Remake   bool      `json:"remake"`
	Me       PPart     `json:"me"`
	Parts    []PPart   `json:"parts"`
	Analysis []Insight `json:"analysis"`
}

type MasteryRow struct {
	Champion int   `json:"champion"`
	Level    int   `json:"level"`
	Points   int64 `json:"points"`
}

type Premade struct {
	Name  string `json:"name"`
	Tag   string `json:"tag"`
	Games int    `json:"games"`
	Wins  int    `json:"wins"`
	Last  int64  `json:"last"`
}

type ChampRow struct {
	Champion int     `json:"champion"`
	Games    int     `json:"games"`
	Wins     int     `json:"wins"`
	KDA      float64 `json:"kda"`
	CSMin    float64 `json:"csMin"`
}

type Roles struct {
	Main    string         `json:"main"`
	Second  string         `json:"second"`
	Counts  map[string]int `json:"counts"`
	Games   int            `json:"games"`
	Needed  int            `json:"needed"`
	Enough  bool           `json:"enough"`
}

type Profile struct {
	Source   string                 `json:"source"`
	Name     string                 `json:"name"`
	Tag      string                 `json:"tag"`
	Level    int                    `json:"level"`
	Icon     int                    `json:"icon"`
	Puuid    string                 `json:"puuid"`
	Region   string                 `json:"region"`
	Ranked   map[string]RankedQueue `json:"ranked"`
	Mastery  []MasteryRow           `json:"mastery"`
	Matches  []PMatch               `json:"matches"`
	Roles    Roles                  `json:"roles"`
	Premades []Premade              `json:"premades"`
	Champs   []ChampRow             `json:"champs"`
	Today    struct {
		W int `json:"w"`
		L int `json:"l"`
	} `json:"today"`
	Error string `json:"error,omitempty"`
}

var (
	profMu    sync.Mutex
	profCache = map[string]struct {
		p  *Profile
		at time.Time
	}{}
)

func queueName(q int) string {
	m := map[int]string{420: "Classificata Solo/Duo", 440: "Classificata Flex", 400: "Normale Draft", 430: "Normale Blind", 490: "Partita Rapida",
		450: "ARAM", 2400: "ARAM Mayhem", 1700: "Arena", 1710: "Arena", 900: "URF", 1900: "URF", 1010: "URF", 830: "Coop vs IA", 840: "Coop vs IA", 850: "Coop vs IA", 0: "Personalizzata"}
	if v, ok := m[q]; ok {
		return v
	}
	return fmt.Sprintf("Coda %d", q)
}

func legacyPos(lane, role string) string {
	switch strings.ToUpper(lane) {
	case "TOP":
		return "TOP"
	case "JUNGLE":
		return "JUNGLE"
	case "MIDDLE", "MID":
		return "MIDDLE"
	case "BOTTOM", "BOT":
		if strings.Contains(strings.ToUpper(role), "SUPPORT") {
			return "UTILITY"
		}
		return "BOTTOM"
	}
	return ""
}

// ----- formato "legacy" del client (LCU match history) -----

type lcuGame struct {
	GameID       int64  `json:"gameId"`
	QueueID      int    `json:"queueId"`
	GameCreation int64  `json:"gameCreation"`
	GameDuration int    `json:"gameDuration"`
	GameMode     string `json:"gameMode"`
	Participants []struct {
		ParticipantID int `json:"participantId"`
		TeamID        int `json:"teamId"`
		ChampionID    int `json:"championId"`
		Spell1        int `json:"spell1Id"`
		Spell2        int `json:"spell2Id"`
		Stats         struct {
			Win     bool `json:"win"`
			Kills   int  `json:"kills"`
			Deaths  int  `json:"deaths"`
			Assists int  `json:"assists"`
			Minions int  `json:"totalMinionsKilled"`
			Neutral int  `json:"neutralMinionsKilled"`
			Gold    int  `json:"goldEarned"`
			Vision  int  `json:"visionScore"`
			Damage  int  `json:"totalDamageDealtToChampions"`
			Item0   int  `json:"item0"`
			Item1   int  `json:"item1"`
			Item2   int  `json:"item2"`
			Item3   int  `json:"item3"`
			Item4   int  `json:"item4"`
			Item5   int  `json:"item5"`
			Item6   int  `json:"item6"`
			Level   int  `json:"champLevel"`
			Place   int  `json:"subteamPlacement"`
		} `json:"stats"`
		Timeline struct {
			Lane string `json:"lane"`
			Role string `json:"role"`
		} `json:"timeline"`
	} `json:"participants"`
	Identities []struct {
		ParticipantID int `json:"participantId"`
		Player        struct {
			Puuid    string `json:"puuid"`
			GameName string `json:"gameName"`
			TagLine  string `json:"tagLine"`
			Summ     string `json:"summonerName"`
		} `json:"player"`
	} `json:"participantIdentities"`
}

func convertLegacy(g lcuGame, me string) PMatch {
	m := PMatch{ID: fmt.Sprint(g.GameID), Queue: g.QueueID, Mode: g.GameMode, Start: g.GameCreation, Duration: g.GameDuration}
	if m.Duration > 100000 { // alcune versioni restituiscono millisecondi
		m.Duration /= 1000
	}
	ids := map[int]int{}
	for i, id := range g.Identities {
		ids[id.ParticipantID] = i
	}
	for _, p := range g.Participants {
		s := p.Stats
		pp := PPart{Champ: p.ChampionID, Team: p.TeamID, K: s.Kills, D: s.Deaths, A: s.Assists, CS: s.Minions + s.Neutral,
			Gold: s.Gold, Dmg: s.Damage, Vision: s.Vision, Items: []int{s.Item0, s.Item1, s.Item2, s.Item3, s.Item4, s.Item5, s.Item6},
			Spells: []int{p.Spell1, p.Spell2}, Level: s.Level, Win: s.Win, Place: s.Place, Pos: legacyPos(p.Timeline.Lane, p.Timeline.Role)}
		if i, ok := ids[p.ParticipantID]; ok {
			pl := g.Identities[i].Player
			pp.Puuid, pp.Name, pp.Tag = pl.Puuid, pl.GameName, pl.TagLine
			if pp.Name == "" {
				pp.Name = pl.Summ
			}
		}
		m.Parts = append(m.Parts, pp)
		if pp.Puuid == me || len(g.Participants) == 1 {
			m.Me = pp
		}
	}
	m.Remake = m.Duration > 0 && m.Duration < 300
	return m
}

func convertV5(mm *MMatch, me string) PMatch {
	m := PMatch{ID: mm.Metadata.MatchID, Queue: mm.Info.QueueID, Mode: mm.Info.GameMode, Start: mm.Info.Start, Duration: mm.Info.Duration}
	for _, p := range mm.Info.Participants {
		name := p.GameName
		if name == "" {
			name = p.SummName
		}
		pp := PPart{Puuid: p.Puuid, Name: name, Tag: p.Tag, Champ: p.ChampionID, Team: p.TeamID, Pos: p.TeamPos, K: p.Kills, D: p.Deaths, A: p.Assists,
			CS: p.Minions + p.Neutral, Gold: p.Gold, Dmg: p.Damage, Vision: p.Vision, Items: p.items(), Spells: []int{p.Spell1, p.Spell2},
			Level: p.Level, Win: p.Win, Place: p.Placement}
		m.Parts = append(m.Parts, pp)
		if p.Puuid == me {
			m.Me = pp
		}
		if p.EarlySurr {
			m.Remake = true
		}
	}
	return m
}

// ----- analisi automatica della partita (gratuita, a regole) -----

func analyze(m *PMatch) {
	me := m.Me
	mins := float64(m.Duration) / 60
	if mins < 1 || m.Remake {
		m.Analysis = []Insight{{"info", "Partita troppo corta per un'analisi (remake)."}}
		return
	}
	var out []Insight
	teamK, teamDmg, teamGold := 0, 0, 0
	var opp *PPart
	for i, p := range m.Parts {
		if p.Team == me.Team {
			teamK += p.K
			teamDmg += p.Dmg
			teamGold += p.Gold
		} else if me.Pos != "" && p.Pos == me.Pos {
			opp = &m.Parts[i]
		}
	}
	csm := float64(me.CS) / mins
	classic := m.Queue == 420 || m.Queue == 440 || m.Queue == 400 || m.Queue == 430 || m.Queue == 490
	if classic && me.Pos != "UTILITY" && me.Pos != "" {
		target := 7.0
		if me.Pos == "JUNGLE" {
			target = 5.5
		}
		switch {
		case csm >= target+0.8:
			out = append(out, Insight{"good", fmt.Sprintf("Farm ottimo: %.1f CS/min (riferimento %.1f).", csm, target)})
		case csm < target-1.2:
			out = append(out, Insight{"bad", fmt.Sprintf("Farm basso: %.1f CS/min. Punta ad almeno %.1f: ogni 15 CS persi sono circa un'uccisione d'oro.", csm, target)})
		default:
			out = append(out, Insight{"info", fmt.Sprintf("Farm nella media: %.1f CS/min.", csm)})
		}
	}
	if teamK > 0 && len(m.Parts) > 2 {
		kp := float64(me.K+me.A) / float64(teamK)
		if kp >= 0.65 {
			out = append(out, Insight{"good", fmt.Sprintf("Partecipazione alle uccisioni alta: %.0f%%.", kp*100)})
		} else if kp < 0.40 && me.Pos != "TOP" {
			out = append(out, Insight{"bad", fmt.Sprintf("Partecipazione alle uccisioni bassa (%.0f%%): unisciti di più ai combattimenti intorno agli obiettivi.", kp*100)})
		}
	}
	dpm := float64(me.D) / mins * 10
	if me.D >= 8 && dpm > 3 {
		out = append(out, Insight{"bad", fmt.Sprintf("%d morti (%.1f ogni 10 minuti): controlla dove muori, spesso è per mancanza di visione o per inseguimenti troppo lunghi.", me.D, dpm)})
	} else if me.D <= 2 && mins > 20 {
		out = append(out, Insight{"good", fmt.Sprintf("Solo %d morti: ottima gestione del rischio.", me.D)})
	}
	if classic {
		vpm := float64(me.Vision) / mins
		ref := 0.9
		if me.Pos == "UTILITY" {
			ref = 2.0
		} else if me.Pos == "JUNGLE" {
			ref = 1.2
		}
		if vpm < ref*0.6 {
			out = append(out, Insight{"bad", fmt.Sprintf("Visione scarsa: %.1f punti visione al minuto (riferimento ~%.1f). Compra Lumi di Controllo e usa il ninnolo.", vpm, ref)})
		} else if vpm > ref*1.3 {
			out = append(out, Insight{"good", fmt.Sprintf("Ottimo controllo della visione: %.1f al minuto.", vpm)})
		}
	}
	if teamDmg > 0 && len(m.Parts) > 2 {
		ds := float64(me.Dmg) / float64(teamDmg)
		gs := float64(me.Gold) / float64(maxI(1, teamGold))
		if ds > gs+0.06 {
			out = append(out, Insight{"good", fmt.Sprintf("Hai fatto il %.0f%% dei danni della squadra con il %.0f%% dell'oro: oro sfruttato molto bene.", ds*100, gs*100)})
		} else if ds < gs-0.08 && me.Pos != "UTILITY" && me.Pos != "JUNGLE" {
			out = append(out, Insight{"bad", fmt.Sprintf("Danni bassi rispetto all'oro (%.0f%% contro %.0f%%): cerca più scambi e posizionamento offensivo nei teamfight.", ds*100, gs*100)})
		}
	}
	if opp != nil {
		d := me.Gold - opp.Gold
		if d > 2000 {
			out = append(out, Insight{"good", fmt.Sprintf("Hai chiuso con %d oro in più di %s.", d, champName(opp.Champ))})
		} else if d < -2000 {
			out = append(out, Insight{"bad", fmt.Sprintf("Hai chiuso con %d oro in meno di %s: rivedi la fase di corsia.", -d, champName(opp.Champ))})
		}
	}
	if len(out) == 0 {
		out = append(out, Insight{"info", "Partita equilibrata, niente di particolare da segnalare."})
	}
	m.Analysis = out
}

func computeDerived(p *Profile) {
	// ruoli: servono almeno 20 partite su Landa (Solo/Duo e normali contano di più della Flex)
	p.Roles = Roles{Counts: map[string]int{}, Needed: 20}
	weighted := map[string]float64{}
	champs := map[int]*ChampRow{}
	kdaSum := map[int][3]int{}
	csSum := map[int]float64{}
	mates := map[string]*Premade{}
	start := time.Now()
	day := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location()).UnixMilli()
	for i := range p.Matches {
		m := &p.Matches[i]
		analyze(m)
		if m.Remake {
			continue
		}
		if m.Queue == 420 && m.Start >= day {
			if m.Me.Win {
				p.Today.W++
			} else {
				p.Today.L++
			}
		}
		if (m.Queue == 420 || m.Queue == 440 || m.Queue == 400 || m.Queue == 430 || m.Queue == 490) && m.Me.Pos != "" {
			p.Roles.Counts[m.Me.Pos]++
			p.Roles.Games++
			w := 1.0
			if m.Queue == 440 {
				w = 0.5
			}
			weighted[m.Me.Pos] += w
		}
		c := champs[m.Me.Champ]
		if c == nil {
			c = &ChampRow{Champion: m.Me.Champ}
			champs[m.Me.Champ] = c
		}
		c.Games++
		if m.Me.Win {
			c.Wins++
		}
		k := kdaSum[m.Me.Champ]
		kdaSum[m.Me.Champ] = [3]int{k[0] + m.Me.K, k[1] + m.Me.D, k[2] + m.Me.A}
		if m.Duration > 0 {
			csSum[m.Me.Champ] += float64(m.Me.CS) / (float64(m.Duration) / 60)
		}
		for _, o := range m.Parts {
			if o.Team != m.Me.Team || o.Puuid == "" || o.Puuid == p.Puuid || len(m.Parts) < 2 {
				continue
			}
			pm := mates[o.Puuid]
			if pm == nil {
				pm = &Premade{Name: o.Name, Tag: o.Tag}
				mates[o.Puuid] = pm
			}
			pm.Games++
			if m.Me.Win {
				pm.Wins++
			}
			if m.Start > pm.Last {
				pm.Last = m.Start
			}
		}
	}
	type kv struct {
		k string
		v float64
	}
	var rs []kv
	for k, v := range weighted {
		rs = append(rs, kv{k, v})
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i].v > rs[j].v })
	p.Roles.Enough = p.Roles.Games >= p.Roles.Needed
	if p.Roles.Enough && len(rs) > 0 {
		p.Roles.Main = rs[0].k
		if len(rs) > 1 && rs[1].v >= rs[0].v*0.25 {
			p.Roles.Second = rs[1].k
		}
	}
	for id, c := range champs {
		k := kdaSum[id]
		c.KDA = float64(k[0]+k[2]) / float64(maxI(1, k[1]))
		c.CSMin = csSum[id] / float64(c.Games)
		p.Champs = append(p.Champs, *c)
	}
	sort.Slice(p.Champs, func(i, j int) bool { return p.Champs[i].Games > p.Champs[j].Games })
	// premade: compagni ritrovati in almeno 2 partite
	for _, pm := range mates {
		if pm.Games >= 2 {
			p.Premades = append(p.Premades, *pm)
		}
	}
	sort.Slice(p.Premades, func(i, j int) bool { return p.Premades[i].Games > p.Premades[j].Games })
	if len(p.Premades) > 12 {
		p.Premades = p.Premades[:12]
	}
}

func ownProfile(force bool) *Profile {
	profMu.Lock()
	if c, ok := profCache["me"]; ok && !force && time.Since(c.at) < 90*time.Second {
		profMu.Unlock()
		return c.p
	}
	profMu.Unlock()
	cs := getClientState()
	p := &Profile{Source: "client", Ranked: cs.Ranked}
	if !cs.Connected || cs.Summoner == nil {
		p.Error = "Apri il client di League of Legends per vedere il tuo profilo."
		return p
	}
	s := cs.Summoner
	p.Name, p.Tag, p.Level, p.Icon, p.Puuid = s.GameName, s.TagLine, s.Level, s.ProfileIconID, s.Puuid
	var mast []struct {
		Champion int   `json:"championId"`
		Level    int   `json:"championLevel"`
		Points   int64 `json:"championPoints"`
	}
	if lcu.getJSON("/lol-champion-mastery/v1/local-player/champion-mastery", &mast) == nil {
		for _, x := range mast {
			p.Mastery = append(p.Mastery, MasteryRow{x.Champion, x.Level, x.Points})
		}
		sort.Slice(p.Mastery, func(i, j int) bool { return p.Mastery[i].Points > p.Mastery[j].Points })
	}
	var hist struct {
		Games struct {
			Games []lcuGame `json:"games"`
		} `json:"games"`
	}
	lcu.getJSON(fmt.Sprintf("/lol-match-history/v1/products/lol/%s/matches?begIndex=0&endIndex=60", s.Puuid), &hist)
	for i, g := range hist.Games.Games {
		if i < 25 {
			var full lcuGame
			if lcu.getJSON(fmt.Sprintf("/lol-match-history/v1/games/%d", g.GameID), &full) == nil && len(full.Participants) > 1 {
				g = full
			}
		}
		p.Matches = append(p.Matches, convertLegacy(g, s.Puuid))
	}
	sort.Slice(p.Matches, func(i, j int) bool { return p.Matches[i].Start > p.Matches[j].Start })
	computeDerived(p)
	profMu.Lock()
	profCache["me"] = struct {
		p  *Profile
		at time.Time
	}{p, time.Now()}
	profMu.Unlock()
	return p
}

func searchProfile(riotID, region string) *Profile {
	if region == "" {
		region = getSettings().Region
	}
	ck := strings.ToLower(riotID + "@" + region)
	profMu.Lock()
	if c, ok := profCache[ck]; ok && time.Since(c.at) < 3*time.Minute {
		profMu.Unlock()
		return c.p
	}
	profMu.Unlock()
	p := &Profile{Source: "riot", Region: region, Ranked: map[string]RankedQueue{}}
	parts := strings.SplitN(riotID, "#", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		p.Error = "Scrivi il Riot ID nel formato Nome#TAG"
		return p
	}
	acc, err := riotAccountByID(region, strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
	if err != nil {
		if strings.Contains(err.Error(), "non trovato") {
			p.Error = "Giocatore non trovato in questa regione."
		} else {
			p.Error = err.Error()
		}
		return p
	}
	p.Name, p.Tag, p.Puuid = acc.GameName, acc.TagLine, acc.Puuid
	var sm struct {
		Level int `json:"summonerLevel"`
		Icon  int `json:"profileIconId"`
	}
	riot.get(platHost(region)+"/lol/summoner/v4/summoners/by-puuid/"+acc.Puuid, &sm)
	p.Level, p.Icon = sm.Level, sm.Icon
	var entries []struct {
		Queue  string `json:"queueType"`
		Tier   string `json:"tier"`
		Rank   string `json:"rank"`
		LP     int    `json:"leaguePoints"`
		Wins   int    `json:"wins"`
		Losses int    `json:"losses"`
	}
	riot.get(platHost(region)+"/lol/league/v4/entries/by-puuid/"+acc.Puuid, &entries)
	for _, e := range entries {
		p.Ranked[e.Queue] = RankedQueue{e.Queue, e.Tier, e.Rank, e.LP, e.Wins, e.Losses}
	}
	var mast []struct {
		Champion int   `json:"championId"`
		Level    int   `json:"championLevel"`
		Points   int64 `json:"championPoints"`
	}
	riot.get(platHost(region)+"/lol/champion-mastery/v4/champion-masteries/by-puuid/"+acc.Puuid+"/top?count=12", &mast)
	for _, x := range mast {
		p.Mastery = append(p.Mastery, MasteryRow{x.Champion, x.Level, x.Points})
	}
	ids, _ := riotMatchIDs(region, acc.Puuid, 20, 0)
	for _, id := range ids {
		if mm, err := riotMatch(region, id); err == nil {
			p.Matches = append(p.Matches, convertV5(mm, acc.Puuid))
		}
	}
	computeDerived(p)
	profMu.Lock()
	profCache[ck] = struct {
		p  *Profile
		at time.Time
	}{p, time.Now()}
	profMu.Unlock()
	return p
}
