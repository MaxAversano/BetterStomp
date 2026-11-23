package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------- Motore statistiche BetterStomp ----------
// Campiona partite pubbliche classificate tramite la Riot API ufficiale (chiave
// dell'utente) e aggrega win rate, pick rate, ban rate, rune, oggetti, incantesimi,
// ordine abilità e matchup per modalità, rango e ruolo. I dati restano sul PC.

var Tiers = []string{"IRON", "BRONZE", "SILVER", "GOLD", "PLATINUM", "EMERALD", "DIAMOND", "MASTER", "GRANDMASTER", "CHALLENGER"}

func queueGroup(q int, gameMode string) string {
	switch q {
	case 420:
		return "ranked"
	case 440:
		return "flex"
	case 400, 430, 490, 480:
		return "normal"
	case 450:
		return "aram"
	case 2400:
		return "mayhem"
	case 1700, 1710:
		return "arena"
	case 900, 1900, 1010:
		return "urf"
	}
	gm := strings.ToUpper(gameMode)
	switch {
	case gm == "ARAM":
		return "aram"
	case gm == "CHERRY":
		return "arena"
	case strings.Contains(gm, "URF"):
		return "urf"
	case strings.Contains(gm, "KIWI") || strings.Contains(gm, "MAYHEM"):
		return "mayhem"
	}
	return ""
}

func positional(mode string) bool { return mode == "ranked" || mode == "flex" || mode == "normal" }

type Stat struct {
	N int `json:"n"`
	W int `json:"w"`
}

func (s *Stat) add(win bool) {
	s.N++
	if win {
		s.W++
	}
}
func (s *Stat) merge(o *Stat) { s.N += o.N; s.W += o.W }

type ChampAgg struct {
	Stat
	Items  map[int]*Stat    `json:"items"`
	Boots  map[int]*Stat    `json:"boots"`
	Start  map[string]*Stat `json:"start"`
	Runes  map[string]*Stat `json:"runes"`
	Spells map[string]*Stat `json:"spells"`
	Skills map[string]*Stat `json:"skills"`
	Builds map[string]*Stat `json:"builds"` // varianti: "stivali:ogg1,ogg2,ogg3"
	Lane   map[int]*Stat    `json:"lane"`
	Vs     map[int]*Stat    `json:"vs"`
	With   map[int]*Stat    `json:"with"`
	K      int64            `json:"k"`
	D      int64            `json:"d"`
	A      int64            `json:"a"`
	CS     int64            `json:"cs"`
	Dur    int64            `json:"dur"`
}

func newAgg() *ChampAgg {
	return &ChampAgg{Items: map[int]*Stat{}, Boots: map[int]*Stat{}, Start: map[string]*Stat{}, Runes: map[string]*Stat{},
		Spells: map[string]*Stat{}, Skills: map[string]*Stat{}, Builds: map[string]*Stat{}, Lane: map[int]*Stat{}, Vs: map[int]*Stat{}, With: map[int]*Stat{}}
}

// file salvati da versioni precedenti possono non avere alcune mappe
func fixAgg(a *ChampAgg) {
	n := newAgg()
	if a.Items == nil {
		a.Items = n.Items
	}
	if a.Boots == nil {
		a.Boots = n.Boots
	}
	if a.Start == nil {
		a.Start = n.Start
	}
	if a.Runes == nil {
		a.Runes = n.Runes
	}
	if a.Spells == nil {
		a.Spells = n.Spells
	}
	if a.Skills == nil {
		a.Skills = n.Skills
	}
	if a.Builds == nil {
		a.Builds = n.Builds
	}
	if a.Lane == nil {
		a.Lane = n.Lane
	}
	if a.Vs == nil {
		a.Vs = n.Vs
	}
	if a.With == nil {
		a.With = n.With
	}
}

func addI(m map[int]*Stat, k int, win bool) {
	s := m[k]
	if s == nil {
		s = &Stat{}
		m[k] = s
	}
	s.add(win)
}
func addS(m map[string]*Stat, k string, win bool) {
	s := m[k]
	if s == nil {
		s = &Stat{}
		m[k] = s
	}
	s.add(win)
}
func mergeI(dst, src map[int]*Stat) {
	for k, v := range src {
		d := dst[k]
		if d == nil {
			d = &Stat{}
			dst[k] = d
		}
		d.merge(v)
	}
}
func mergeS(dst, src map[string]*Stat) {
	for k, v := range src {
		d := dst[k]
		if d == nil {
			d = &Stat{}
			dst[k] = d
		}
		d.merge(v)
	}
}

func (a *ChampAgg) merge(o *ChampAgg) {
	a.Stat.merge(&o.Stat)
	mergeI(a.Items, o.Items)
	mergeI(a.Boots, o.Boots)
	mergeS(a.Start, o.Start)
	mergeS(a.Runes, o.Runes)
	mergeS(a.Spells, o.Spells)
	mergeS(a.Skills, o.Skills)
	mergeS(a.Builds, o.Builds)
	mergeI(a.Lane, o.Lane)
	mergeI(a.Vs, o.Vs)
	mergeI(a.With, o.With)
	a.K += o.K
	a.D += o.D
	a.A += o.A
	a.CS += o.CS
	a.Dur += o.Dur
}

type Bucket struct {
	Matches int                  `json:"matches"`
	Bans    map[int]int          `json:"bans"`
	Champs  map[string]*ChampAgg `json:"champs"`
}

func newBucket() *Bucket { return &Bucket{Bans: map[int]int{}, Champs: map[string]*ChampAgg{}} }

func (b *Bucket) champ(c int, pos string) *ChampAgg {
	k := fmt.Sprintf("%d|%s", c, pos)
	a := b.Champs[k]
	if a == nil {
		a = newAgg()
		b.Champs[k] = a
	}
	return a
}

type DB struct {
	Patch   string             `json:"patch"`
	Buckets map[string]*Bucket `json:"buckets"`
	Seen    map[string]bool    `json:"seen"`
	Total   int                `json:"total"`
}

func newDB(p string) *DB {
	return &DB{Patch: p, Buckets: map[string]*Bucket{}, Seen: map[string]bool{}}
}

type Engine struct {
	mu      sync.RWMutex
	db      *DB
	prev    *DB
	pools   map[string][]string
	lastErr string
	running bool
	started time.Time
	session int
	cache   map[string]cachedBucket
	dirty   bool
}

type cachedBucket struct {
	b  *Bucket
	at time.Time
}

var engine = &Engine{pools: map[string][]string{}, cache: map[string]cachedBucket{}}

func statsDir() string { d := filepath.Join(dataDir, "stats"); os.MkdirAll(d, 0o755); return d }

func (e *Engine) loadPatch(p string) *DB {
	var db DB
	if readJSON(filepath.Join(statsDir(), "stats_"+p+".json"), &db) == nil && db.Buckets != nil {
		if db.Seen == nil {
			db.Seen = map[string]bool{}
		}
		for _, b := range db.Buckets {
			if b.Bans == nil {
				b.Bans = map[int]int{}
			}
			for _, a := range b.Champs {
				fixAgg(a)
			}
		}
		return &db
	}
	return newDB(p)
}

func (e *Engine) ensurePatch() {
	cur := majorMinor(static.version())
	if cur == "" {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.db != nil && e.db.Patch == cur {
		return
	}
	if e.db != nil && e.dirty {
		writeJSON(filepath.Join(statsDir(), "stats_"+e.db.Patch+".json"), e.db)
	}
	e.db = e.loadPatch(cur)
	// patch precedente come riserva finché la nuova non ha abbastanza partite
	static.mu.RLock()
	prevV := previousPatchVersion(static.Versions, static.Version)
	static.mu.RUnlock()
	if prevV != "" {
		p := e.loadPatch(majorMinor(prevV))
		if p.Total > 0 {
			e.prev = p
		}
	}
	e.cache = map[string]cachedBucket{}
}

func (e *Engine) save() {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.db != nil && e.dirty {
		writeJSON(filepath.Join(statsDir(), "stats_"+e.db.Patch+".json"), e.db)
	}
}

func (e *Engine) setErr(s string) { e.mu.Lock(); e.lastErr = s; e.mu.Unlock() }

func (e *Engine) seedPlayer(platform, tier string) (string, error) {
	e.mu.Lock()
	pool := e.pools[tier]
	if len(pool) > 0 {
		i := rand.Intn(len(pool))
		p := pool[i]
		pool[i] = pool[len(pool)-1]
		e.pools[tier] = pool[:len(pool)-1]
		e.mu.Unlock()
		return p, nil
	}
	e.mu.Unlock()
	type entry struct {
		Puuid      string `json:"puuid"`
		SummonerID string `json:"summonerId"`
	}
	var entries []entry
	switch tier {
	case "MASTER", "GRANDMASTER", "CHALLENGER":
		var l struct {
			Entries []entry `json:"entries"`
		}
		name := strings.ToLower(tier)
		if err := riot.get(fmt.Sprintf("%s/lol/league/v4/%sleagues/by-queue/RANKED_SOLO_5x5", platHost(platform), name), &l); err != nil {
			return "", err
		}
		entries = l.Entries
	default:
		divs := []string{"I", "II", "III", "IV"}
		u := fmt.Sprintf("%s/lol/league/v4/entries/RANKED_SOLO_5x5/%s/%s?page=%d", platHost(platform), tier, divs[rand.Intn(4)], 1+rand.Intn(3))
		if err := riot.get(u, &entries); err != nil {
			return "", err
		}
	}
	var ids []string
	for _, en := range entries {
		if en.Puuid != "" {
			ids = append(ids, en.Puuid)
		} else if en.SummonerID != "" && len(ids) < 15 {
			var s struct {
				Puuid string `json:"puuid"`
			}
			if riot.get(platHost(platform)+"/lol/summoner/v4/summoners/"+en.SummonerID, &s) == nil && s.Puuid != "" {
				ids = append(ids, s.Puuid)
			}
		}
	}
	rand.Shuffle(len(ids), func(i, j int) { ids[i], ids[j] = ids[j], ids[i] })
	if len(ids) > 60 {
		ids = ids[:60]
	}
	if len(ids) == 0 {
		return "", fmt.Errorf("nessun giocatore trovato in %s", tier)
	}
	e.mu.Lock()
	e.pools[tier] = ids[1:]
	e.mu.Unlock()
	return ids[0], nil
}

func engineLoop() {
	tierIdx := rand.Intn(len(Tiers))
	lastSave := time.Now()
	for {
		s := getSettings()
		if !s.EngineEnabled || strings.TrimSpace(s.RiotAPIKey) == "" || !staticReady() {
			engine.mu.Lock()
			engine.running = false
			engine.mu.Unlock()
			time.Sleep(5 * time.Second)
			continue
		}
		// durante la partita il motore rallenta per non disturbare
		if getClientState().Phase == "InProgress" {
			time.Sleep(5 * time.Second)
		}
		engine.ensurePatch()
		engine.mu.Lock()
		if !engine.running {
			engine.running = true
			engine.started = time.Now()
		}
		engine.mu.Unlock()

		tier := Tiers[tierIdx%len(Tiers)]
		tierIdx++
		puuid, err := engine.seedPlayer(s.Region, tier)
		if err != nil {
			engine.setErr(err.Error())
			if _, ok := err.(riotErr); ok && riot.getStatus() == "invalid" {
				time.Sleep(60 * time.Second)
			} else {
				time.Sleep(5 * time.Second)
			}
			continue
		}
		ids, err := riotMatchIDs(s.Region, puuid, 12, 0)
		if err != nil {
			engine.setErr(err.Error())
			continue
		}
		taken := 0
		for _, id := range ids {
			if taken >= 5 {
				break
			}
			engine.mu.RLock()
			seen := engine.db.Seen[id]
			patch := engine.db.Patch
			engine.mu.RUnlock()
			if seen {
				continue
			}
			m, err := riotMatch(s.Region, id)
			if err != nil {
				engine.setErr(err.Error())
				break
			}
			engine.mu.Lock()
			engine.db.Seen[id] = true
			engine.mu.Unlock()
			if majorMinor(m.Info.GameVersion) != patch {
				break // le partite successive sono ancora più vecchie
			}
			mode := queueGroup(m.Info.QueueID, m.Info.GameMode)
			if mode == "" || m.Info.Duration < 300 {
				continue
			}
			var tl *MTimeline
			if positional(mode) && rand.Float64() < 0.35 {
				tl, _ = riotTimeline(s.Region, id)
			}
			engine.ingest(m, mode, tier, tl)
			taken++
		}
		engine.setErr("")
		if time.Since(lastSave) > 3*time.Minute {
			engine.save()
			lastSave = time.Now()
		}
	}
}

func runeKey(p MParticipant) string {
	if len(p.Perks.Styles) < 2 {
		return ""
	}
	var perks []string
	for _, st := range p.Perks.Styles {
		for _, s := range st.Selections {
			perks = append(perks, strconv.Itoa(s.Perk))
		}
	}
	if len(perks) != 6 {
		return ""
	}
	perks = append(perks, strconv.Itoa(p.Perks.Stat.Offense), strconv.Itoa(p.Perks.Stat.Flex), strconv.Itoa(p.Perks.Stat.Defense))
	return fmt.Sprintf("%d:%d:%s", p.Perks.Styles[0].Style, p.Perks.Styles[1].Style, strings.Join(perks, ","))
}

func (e *Engine) ingest(m *MMatch, mode, tier string, tl *MTimeline) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := mode + "|" + tier
	b := e.db.Buckets[key]
	if b == nil {
		b = newBucket()
		e.db.Buckets[key] = b
	}
	b.Matches++
	e.db.Total++
	e.session++
	e.dirty = true
	for _, t := range m.Info.Teams {
		for _, bn := range t.Bans {
			if bn.ChampionID > 0 {
				b.Bans[bn.ChampionID]++
			}
		}
	}
	// dati della timeline: oggetti iniziali e ordine abilità
	starts := map[int][]int{}
	skills := map[int][]int{}
	order := map[int][]int{}
	if tl != nil {
		for _, f := range tl.Info.Frames {
			for _, ev := range f.Events {
				switch ev.Type {
				case "ITEM_PURCHASED":
					if ev.Timestamp < 65000 && ev.ItemID != 3340 && ev.ItemID != 3364 && ev.ItemID != 3363 {
						starts[ev.ParticipantID] = append(starts[ev.ParticipantID], ev.ItemID)
					}
					if isCompleted(ev.ItemID) {
						order[ev.ParticipantID] = append(order[ev.ParticipantID], ev.ItemID)
					}
				case "SKILL_LEVEL_UP":
					if ev.LevelUpType == "NORMAL" || ev.LevelUpType == "" {
						skills[ev.ParticipantID] = append(skills[ev.ParticipantID], ev.SkillSlot)
					}
				}
			}
		}
	}
	isPos := positional(mode)
	arena := mode == "arena"
	for i, p := range m.Info.Participants {
		pos := "ALL"
		if isPos && p.TeamPos != "" {
			pos = p.TeamPos
		}
		a := b.champ(p.ChampionID, pos)
		a.add(p.Win)
		a.K += int64(p.Kills)
		a.D += int64(p.Deaths)
		a.A += int64(p.Assists)
		a.CS += int64(p.Minions + p.Neutral)
		a.Dur += int64(m.Info.Duration)
		boots := 0
		var final []int
		for _, it := range p.items() {
			if isBoots(it) {
				addI(a.Boots, it, p.Win)
				boots = it
			} else if isCompleted(it) {
				addI(a.Items, it, p.Win)
				final = append(final, it)
			}
		}
		// variante di build: primi 3 oggetti completati (ordine d'acquisto dalla timeline,
		// altrimenti l'ordine degli slot, che segue quasi sempre quello d'acquisto)
		path := final
		if seq := order[i+1]; len(seq) >= 3 {
			path = nil
			seen := map[int]bool{}
			for _, it := range seq {
				if !seen[it] {
					seen[it] = true
					path = append(path, it)
				}
			}
		}
		if len(path) >= 3 {
			addS(a.Builds, fmt.Sprintf("%d:%d,%d,%d", boots, path[0], path[1], path[2]), p.Win)
		}
		if rk := runeKey(p); rk != "" {
			addS(a.Runes, rk, p.Win)
		}
		s1, s2 := p.Spell1, p.Spell2
		if s1 > s2 {
			s1, s2 = s2, s1
		}
		addS(a.Spells, fmt.Sprintf("%d,%d", s1, s2), p.Win)
		if st := starts[i+1]; len(st) > 0 {
			sort.Ints(st)
			var parts []string
			for _, x := range st {
				parts = append(parts, strconv.Itoa(x))
			}
			addS(a.Start, strings.Join(parts, ","), p.Win)
		}
		if sk := skills[i+1]; len(sk) >= 9 {
			addS(a.Skills, skillKey(sk), p.Win)
		}
		if arena {
			continue
		}
		for _, o := range m.Info.Participants {
			if o.Puuid == p.Puuid {
				continue
			}
			if o.TeamID != p.TeamID {
				addI(a.Vs, o.ChampionID, p.Win)
				if isPos && o.TeamPos != "" && o.TeamPos == p.TeamPos {
					addI(a.Lane, o.ChampionID, p.Win)
				}
			} else {
				addI(a.With, o.ChampionID, p.Win)
			}
		}
	}
	e.cache = map[string]cachedBucket{}
}

// ordine abilità: "Q>E>W|QEW" (priorità di massimizzazione | prime tre)
func skillKey(seq []int) string {
	letters := map[int]string{1: "Q", 2: "W", 3: "E", 4: "R"}
	first := ""
	for _, s := range seq {
		if len(first) >= 3 {
			break
		}
		first += letters[s]
	}
	count := map[int]int{}
	reach := map[int]int{}
	limit := len(seq)
	if limit > 13 {
		limit = 13
	}
	for i := 0; i < limit; i++ {
		s := seq[i]
		if s == 4 {
			continue
		}
		count[s]++
		reach[s] = i
	}
	order := []int{1, 2, 3}
	sort.Slice(order, func(i, j int) bool {
		a, b := order[i], order[j]
		if count[a] != count[b] {
			return count[a] > count[b]
		}
		return reach[a] < reach[b]
	})
	return letters[order[0]] + ">" + letters[order[1]] + ">" + letters[order[2]] + "|" + first
}

// ---------- Interrogazioni ----------

func tierSet(tf string) []string {
	tf = strings.ToUpper(strings.TrimSpace(tf))
	if tf == "" || tf == "ALL" || tf == "AUTO" {
		return Tiers
	}
	if strings.HasSuffix(tf, "+") {
		base := strings.TrimSuffix(tf, "+")
		for i, t := range Tiers {
			if t == base {
				return Tiers[i:]
			}
		}
	}
	for _, t := range Tiers {
		if t == tf {
			return []string{t}
		}
	}
	return Tiers
}

func (e *Engine) merged(db *DB, mode string, tiers []string) *Bucket {
	if db == nil {
		return newBucket()
	}
	ck := db.Patch + "|" + mode + "|" + strings.Join(tiers, ",")
	if c, ok := e.cache[ck]; ok && time.Since(c.at) < 90*time.Second {
		return c.b
	}
	out := newBucket()
	for _, t := range tiers {
		b := db.Buckets[mode+"|"+t]
		if b == nil {
			continue
		}
		out.Matches += b.Matches
		for k, v := range b.Bans {
			out.Bans[k] += v
		}
		for k, a := range b.Champs {
			d := out.Champs[k]
			if d == nil {
				d = newAgg()
				out.Champs[k] = d
			}
			d.merge(a)
		}
	}
	e.cache[ck] = cachedBucket{out, time.Now()}
	return out
}

func smooth(w, n int, prior float64, k float64) float64 {
	return (float64(w) + prior*k) / (float64(n) + k)
}

type TierRow struct {
	Champion int     `json:"champion"`
	Position string  `json:"position"`
	Games    int     `json:"games"`
	WinRate  float64 `json:"winRate"`
	PickRate float64 `json:"pickRate"`
	BanRate  float64 `json:"banRate"`
	Score    float64 `json:"score"`
	Tier     string  `json:"tier"`
}

func splitKey(k string) (int, string) {
	p := strings.SplitN(k, "|", 2)
	c, _ := strconv.Atoi(p[0])
	if len(p) < 2 {
		return c, "ALL"
	}
	return c, p[1]
}

func (e *Engine) TierList(mode, tf, pos string) ([]TierRow, int, string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	db := e.db
	b := e.merged(db, mode, tierSet(tf))
	patch := ""
	if db != nil {
		patch = db.Patch
	}
	if b.Matches < 200 && e.prev != nil {
		pb := e.merged(e.prev, mode, tierSet(tf))
		if pb.Matches > b.Matches {
			b, patch = pb, e.prev.Patch
		}
	}
	if b.Matches == 0 {
		return nil, 0, patch
	}
	minGames := int(math.Max(8, float64(b.Matches)*0.004))
	var rows []TierRow
	for k, a := range b.Champs {
		c, p := splitKey(k)
		if pos != "" && pos != "ALL" && p != pos {
			continue
		}
		if a.N < minGames {
			continue
		}
		wr := smooth(a.W, a.N, 0.5, 30)
		pr := float64(a.N) / float64(b.Matches)
		br := float64(b.Bans[c]) / float64(b.Matches)
		score := (wr-0.5)*220 + math.Min(pr, 0.2)*35 + math.Min(br, 0.4)*8
		rows = append(rows, TierRow{c, p, a.N, float64(a.W) / float64(a.N), pr, br, score, ""})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Score > rows[j].Score })
	n := float64(len(rows))
	for i := range rows {
		q := float64(i) / n
		switch {
		case q < 0.05:
			rows[i].Tier = "S+"
		case q < 0.17:
			rows[i].Tier = "S"
		case q < 0.40:
			rows[i].Tier = "A"
		case q < 0.70:
			rows[i].Tier = "B"
		case q < 0.90:
			rows[i].Tier = "C"
		default:
			rows[i].Tier = "D"
		}
	}
	return rows, b.Matches, patch
}

type RunePage struct {
	Primary   int     `json:"primary"`
	Secondary int     `json:"secondary"`
	Perks     []int   `json:"perks"`
	Games     int     `json:"games"`
	WinRate   float64 `json:"winRate"`
	Source    string  `json:"source"`
	Spells    []int   `json:"spells,omitempty"`
}

func parseRuneKey(k string) *RunePage {
	p := strings.Split(k, ":")
	if len(p) != 3 {
		return nil
	}
	r := &RunePage{}
	r.Primary, _ = strconv.Atoi(p[0])
	r.Secondary, _ = strconv.Atoi(p[1])
	for _, x := range strings.Split(p[2], ",") {
		v, _ := strconv.Atoi(x)
		r.Perks = append(r.Perks, v)
	}
	if len(r.Perks) != 9 {
		return nil
	}
	return r
}

type Pair struct {
	Champion int     `json:"champion"`
	Games    int     `json:"games"`
	WinRate  float64 `json:"winRate"`
	Delta    float64 `json:"delta"`
}

type PosShare struct {
	Position string  `json:"position"`
	Games    int     `json:"games"`
	Share    float64 `json:"share"`
}

type BuildVariant struct {
	Label       string  `json:"label"`
	Path        []int   `json:"path"`
	Boots       int     `json:"boots"`
	Games       int     `json:"games"`
	WinRate     float64 `json:"winRate"`
	Share       float64 `json:"share"`
	Core        []int   `json:"core"`
	Situational []int   `json:"situational"`
}

type BuildResult struct {
	Champion    int             `json:"champion"`
	Position    string          `json:"position"`
	Positions   []PosShare      `json:"positions"`
	Mode        string          `json:"mode"`
	Tier        string          `json:"tier"`
	Patch       string          `json:"patch"`
	Games       int             `json:"games"`
	Matches     int             `json:"matches"`
	WinRate     float64         `json:"winRate"`
	PickRate    float64         `json:"pickRate"`
	BanRate     float64         `json:"banRate"`
	TierLabel   string          `json:"tierLabel"`
	KDA         [3]float64      `json:"kda"`
	CSMin       float64         `json:"csMin"`
	Runes       *RunePage       `json:"runes"`
	RunesAlt    *RunePage       `json:"runesAlt"`
	Spells      []int           `json:"spells"`
	SpellsWR    float64         `json:"spellsWR"`
	Starting    []int           `json:"starting"`
	Core        []int           `json:"core"`
	Situational []int           `json:"situational"`
	ItemWR      map[int]float64 `json:"itemWR"`
	Skills      string          `json:"skills"`
	SkillFirst  string          `json:"skillFirst"`
	Best        []Pair          `json:"best"`
	Worst       []Pair          `json:"worst"`
	Synergy     []Pair          `json:"synergy"`
	Variants    []BuildVariant  `json:"variants"`
	Variant     int             `json:"variant"`
	Source      string          `json:"source"`
	Personal    bool            `json:"personal"`
	Widened     bool            `json:"widened"`
}

func topS(m map[string]*Stat, min int) (string, *Stat) {
	best, bk := (*Stat)(nil), ""
	for k, v := range m {
		if v.N < min {
			continue
		}
		if best == nil || v.N > best.N || (v.N == best.N && v.W > best.W) {
			best, bk = v, k
		}
	}
	return bk, best
}

func sortedItems(m map[int]*Stat) []int {
	var ids []int
	for k := range m {
		ids = append(ids, k)
	}
	sort.Slice(ids, func(i, j int) bool {
		if m[ids[i]].N != m[ids[j]].N {
			return m[ids[i]].N > m[ids[j]].N
		}
		return ids[i] < ids[j]
	})
	return ids
}

func (e *Engine) ChampionStats(champ int, mode, tf, pos string) BuildResult {
	e.mu.Lock()
	defer e.mu.Unlock()
	r := BuildResult{Champion: champ, Mode: mode, Tier: tf, ItemWR: map[int]float64{}}
	if e.db == nil {
		return r
	}
	try := func(db *DB, tiers []string) (*Bucket, *ChampAgg, string) {
		b := e.merged(db, mode, tiers)
		// ruoli giocati
		var shares []PosShare
		total := 0
		for k, a := range b.Champs {
			c, p := splitKey(k)
			if c == champ {
				shares = append(shares, PosShare{p, a.N, 0})
				total += a.N
			}
		}
		sort.Slice(shares, func(i, j int) bool { return shares[i].Games > shares[j].Games })
		for i := range shares {
			shares[i].Share = float64(shares[i].Games) / math.Max(1, float64(total))
		}
		r.Positions = shares
		p := pos
		if !positional(mode) {
			p = "ALL"
		} else if p == "" || p == "ALL" {
			if len(shares) > 0 {
				p = shares[0].Position
			}
		}
		return b, b.Champs[fmt.Sprintf("%d|%s", champ, p)], p
	}
	b, a, p := try(e.db, tierSet(tf))
	r.Patch = e.db.Patch
	if a == nil || a.N < 15 {
		if b2, a2, p2 := try(e.db, Tiers); a2 != nil && (a == nil || a2.N > a.N) {
			b, a, p, r.Widened = b2, a2, p2, true
		}
	}
	if (a == nil || a.N < 15) && e.prev != nil {
		if b2, a2, p2 := try(e.prev, tierSet(tf)); a2 != nil && (a == nil || a2.N > a.N) {
			b, a, p, r.Patch = b2, a2, p2, e.prev.Patch
		}
	}
	r.Position = p
	if a == nil || a.N == 0 {
		return r
	}
	r.Source = "stats"
	r.Games, r.Matches = a.N, b.Matches
	r.WinRate = float64(a.W) / float64(a.N)
	r.PickRate = float64(a.N) / math.Max(1, float64(b.Matches))
	r.BanRate = float64(b.Bans[champ]) / math.Max(1, float64(b.Matches))
	n := float64(a.N)
	r.KDA = [3]float64{float64(a.K) / n, float64(a.D) / n, float64(a.A) / n}
	if a.Dur > 0 {
		r.CSMin = float64(a.CS) / (float64(a.Dur) / 60)
	}
	min := int(math.Max(2, n*0.02))
	// rune: le due pagine più giocate
	type kv struct {
		k string
		s *Stat
	}
	var rp []kv
	for k, v := range a.Runes {
		rp = append(rp, kv{k, v})
	}
	sort.Slice(rp, func(i, j int) bool { return rp[i].s.N > rp[j].s.N })
	for i, x := range rp {
		if i > 1 || x.s.N < min {
			break
		}
		pg := parseRuneKey(x.k)
		if pg == nil {
			continue
		}
		pg.Games, pg.WinRate, pg.Source = x.s.N, float64(x.s.W)/float64(x.s.N), "Statistiche BetterStomp"
		if r.Runes == nil {
			r.Runes = pg
		} else {
			r.RunesAlt = pg
		}
	}
	if k, s := topS(a.Spells, 1); s != nil {
		for _, x := range strings.Split(k, ",") {
			v, _ := strconv.Atoi(x)
			r.Spells = append(r.Spells, v)
		}
		r.SpellsWR = float64(s.W) / float64(s.N)
	}
	if k, s := topS(a.Start, 1); s != nil {
		seen := map[int]bool{}
		for _, x := range strings.Split(k, ",") {
			v, _ := strconv.Atoi(x)
			if !seen[v] {
				r.Starting = append(r.Starting, v)
				seen[v] = true
			}
		}
	}
	boots := sortedItems(a.Boots)
	items := sortedItems(a.Items)
	for _, id := range items {
		r.ItemWR[id] = float64(a.Items[id].W) / float64(a.Items[id].N)
	}
	for _, id := range boots {
		r.ItemWR[id] = float64(a.Boots[id].W) / float64(a.Boots[id].N)
	}
	if len(boots) > 0 {
		r.Core = append(r.Core, boots[0])
	}
	for _, id := range items {
		if len(r.Core) >= 10 {
			r.Situational = append(r.Situational, id)
		} else {
			r.Core = append(r.Core, id)
		}
		if len(r.Situational) >= 9 {
			break
		}
	}
	if len(boots) > 1 && len(r.Situational) < 10 {
		r.Situational = append(r.Situational, boots[1])
	}
	// 3-4 build alternative con il loro win rate
	var bk []kv
	tot := 0
	for k, v := range a.Builds {
		bk = append(bk, kv{k, v})
		tot += v.N
	}
	sort.Slice(bk, func(i, j int) bool { return bk[i].s.N > bk[j].s.N })
	minB := int(math.Max(2, float64(tot)*0.03))
	usedLead := map[int]bool{}
	for _, x := range bk {
		if len(r.Variants) >= 4 || x.s.N < minB {
			break
		}
		parts := strings.SplitN(x.k, ":", 2)
		if len(parts) != 2 {
			continue
		}
		bt, _ := strconv.Atoi(parts[0])
		var path []int
		for _, y := range strings.Split(parts[1], ",") {
			v, _ := strconv.Atoi(y)
			path = append(path, v)
		}
		// varianti davvero diverse: oggetto iniziale diverso o almeno due oggetti diversi
		dup := false
		for _, ov := range r.Variants {
			same := 0
			for _, a1 := range ov.Path {
				for _, b1 := range path {
					if a1 == b1 {
						same++
					}
				}
			}
			if same >= 2 && ov.Path[0] == path[0] {
				dup = true
			}
		}
		if dup {
			continue
		}
		v := BuildVariant{Path: path, Boots: bt, Games: x.s.N, WinRate: float64(x.s.W) / float64(x.s.N), Share: float64(x.s.N) / float64(tot)}
		if it, ok := itemInfo(path[0]); ok {
			v.Label = it.Name
		}
		if usedLead[path[0]] && len(path) > 1 {
			if it, ok := itemInfo(path[1]); ok {
				v.Label += " + " + it.Name
			}
		}
		usedLead[path[0]] = true
		in := map[int]bool{}
		if bt > 0 {
			v.Core = append(v.Core, bt)
			in[bt] = true
		}
		for _, id := range path {
			v.Core = append(v.Core, id)
			in[id] = true
		}
		for _, id := range items {
			if in[id] {
				continue
			}
			if len(v.Core) < 10 {
				v.Core = append(v.Core, id)
			} else if len(v.Situational) < 9 {
				v.Situational = append(v.Situational, id)
			}
			in[id] = true
		}
		for _, id := range boots {
			if !in[id] && len(v.Situational) < 10 {
				v.Situational = append(v.Situational, id)
				break
			}
		}
		r.Variants = append(r.Variants, v)
	}
	if len(r.Variants) > 0 {
		r.Core, r.Situational = r.Variants[0].Core, r.Variants[0].Situational
	}
	if k, s := topS(a.Skills, 1); s != nil {
		parts := strings.SplitN(k, "|", 2)
		r.Skills = parts[0]
		if len(parts) > 1 {
			r.SkillFirst = parts[1]
		}
	}
	base := r.WinRate
	pairs := func(m map[int]*Stat, minG int) []Pair {
		var out []Pair
		for c, s := range m {
			if s.N < minG {
				continue
			}
			wr := smooth(s.W, s.N, base, 8)
			out = append(out, Pair{c, s.N, float64(s.W) / float64(s.N), wr - base})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Delta > out[j].Delta })
		return out
	}
	lane := pairs(a.Lane, 3)
	if !positional(mode) {
		lane = pairs(a.Vs, 4)
	}
	for i := 0; i < len(lane) && i < 6; i++ {
		if lane[i].Delta > 0 {
			r.Best = append(r.Best, lane[i])
		}
	}
	for i := len(lane) - 1; i >= 0 && len(r.Worst) < 6; i-- {
		if lane[i].Delta < 0 {
			r.Worst = append(r.Worst, lane[i])
		}
	}
	syn := pairs(a.With, 4)
	for i := 0; i < len(syn) && i < 6; i++ {
		if syn[i].Delta > 0 {
			r.Synergy = append(r.Synergy, syn[i])
		}
	}
	// etichetta di tier coerente con la tier list
	rows, _, _ := e.tierListLocked(b)
	for _, row := range rows {
		if row.Champion == champ && row.Position == p {
			r.TierLabel = row.Tier
		}
	}
	return r
}

// versione senza lock della tier list su un bucket già unito
func (e *Engine) tierListLocked(b *Bucket) ([]TierRow, int, string) {
	if b.Matches == 0 {
		return nil, 0, ""
	}
	minGames := int(math.Max(8, float64(b.Matches)*0.004))
	var rows []TierRow
	for k, a := range b.Champs {
		c, p := splitKey(k)
		if a.N < minGames {
			continue
		}
		wr := smooth(a.W, a.N, 0.5, 30)
		pr := float64(a.N) / float64(b.Matches)
		br := float64(b.Bans[c]) / float64(b.Matches)
		rows = append(rows, TierRow{c, p, a.N, float64(a.W) / float64(a.N), pr, br, (wr-0.5)*220 + math.Min(pr, 0.2)*35 + math.Min(br, 0.4)*8, ""})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Score > rows[j].Score })
	n := float64(len(rows))
	for i := range rows {
		q := float64(i) / n
		rows[i].Tier = map[bool]string{true: "S+"}[q < 0.05]
		switch {
		case q < 0.05:
		case q < 0.17:
			rows[i].Tier = "S"
		case q < 0.40:
			rows[i].Tier = "A"
		case q < 0.70:
			rows[i].Tier = "B"
		case q < 0.90:
			rows[i].Tier = "C"
		default:
			rows[i].Tier = "D"
		}
	}
	return rows, b.Matches, ""
}

type SuggestReq struct {
	Mode     string `json:"mode"`
	Tier     string `json:"tier"`
	Position string `json:"position"`
	Kind     string `json:"kind"` // matchup | team
	Allies   []int  `json:"allies"`
	Enemies  []int  `json:"enemies"`
	Opponent int    `json:"opponent"`
	Exclude  []int  `json:"exclude"`
}

type Suggestion struct {
	Champion int     `json:"champion"`
	Score    float64 `json:"score"`
	WinRate  float64 `json:"winRate"`
	Games    int     `json:"games"`
	Detail   string  `json:"detail"`
}

func (e *Engine) Suggest(q SuggestReq) []Suggestion {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.db == nil {
		return nil
	}
	db := e.db
	b := e.merged(db, q.Mode, tierSet(q.Tier))
	if b.Matches < 300 && e.prev != nil {
		if pb := e.merged(e.prev, q.Mode, tierSet(q.Tier)); pb.Matches > b.Matches {
			b = pb
		}
	}
	ex := map[int]bool{}
	for _, c := range append(append(append([]int{}, q.Exclude...), q.Allies...), q.Enemies...) {
		ex[c] = true
	}
	pos := q.Position
	if !positional(q.Mode) {
		pos = "ALL"
	}
	var out []Suggestion
	for k, a := range b.Champs {
		c, p := splitKey(k)
		if ex[c] || a.N < 10 || (pos != "" && pos != "ALL" && p != pos) {
			continue
		}
		base := smooth(a.W, a.N, 0.5, 30)
		s := Suggestion{Champion: c, Games: a.N, WinRate: float64(a.W) / float64(a.N)}
		if q.Kind == "matchup" && q.Opponent > 0 {
			src := a.Lane
			if !positional(q.Mode) {
				src = a.Vs
			}
			st := src[q.Opponent]
			if st == nil || st.N < 2 {
				continue
			}
			wr := smooth(st.W, st.N, base, 10)
			s.Score = wr
			s.Detail = fmt.Sprintf("%.1f%% contro %s su %d partite", 100*float64(st.W)/float64(st.N), champName(q.Opponent), st.N)
		} else {
			delta, cnt := 0.0, 0.0
			for _, en := range q.Enemies {
				if st := a.Vs[en]; st != nil && st.N >= 2 {
					delta += smooth(st.W, st.N, base, 10) - base
					cnt++
				}
			}
			for _, al := range q.Allies {
				if st := a.With[al]; st != nil && st.N >= 2 {
					delta += 0.6 * (smooth(st.W, st.N, base, 10) - base)
					cnt += 0.6
				}
			}
			if cnt > 0 {
				delta /= cnt
			}
			s.Score = base + delta
			s.Detail = fmt.Sprintf("Base %.1f%% · effetto composizione %+.1f%%", base*100, delta*100)
		}
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > 12 {
		out = out[:12]
	}
	return out
}

type EngineStatus struct {
	Enabled  bool   `json:"enabled"`
	HasKey   bool   `json:"hasKey"`
	Running  bool   `json:"running"`
	Patch    string `json:"patch"`
	Total    int    `json:"total"`
	Session  int    `json:"session"`
	PerHour  int    `json:"perHour"`
	Error    string `json:"error"`
	KeyState string `json:"keyState"`
	Prev     int    `json:"prev"`
}

func (e *Engine) Status() EngineStatus {
	s := getSettings()
	e.mu.RLock()
	defer e.mu.RUnlock()
	st := EngineStatus{Enabled: s.EngineEnabled, HasKey: strings.TrimSpace(s.RiotAPIKey) != "", Running: e.running,
		Session: e.session, Error: e.lastErr, KeyState: riot.getStatus()}
	if e.db != nil {
		st.Patch, st.Total = e.db.Patch, e.db.Total
	}
	if e.prev != nil {
		st.Prev = e.prev.Total
	}
	if e.running && time.Since(e.started) > time.Minute {
		st.PerHour = int(float64(e.session) / time.Since(e.started).Hours())
	}
	return st
}
