package main

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ---------- Client locale di League (LCU) ----------

type LCU struct {
	mu        sync.RWMutex
	port      string
	auth      string
	connected bool
	client    *http.Client
}

var lcu = &LCU{client: &http.Client{
	Timeout:   8 * time.Second,
	Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
}}

func (l *LCU) isConnected() bool { l.mu.RLock(); defer l.mu.RUnlock(); return l.connected }

func parseLockfile(b []byte) (port, pass string, ok bool) {
	p := strings.Split(strings.TrimSpace(string(b)), ":")
	if len(p) >= 4 {
		return p[2], p[3], true
	}
	return "", "", false
}

func candidateLeagueDirs() []string {
	var dirs []string
	if s := getSettings(); s.LeaguePath != "" {
		dirs = append(dirs, s.LeaguePath)
	}
	pd := os.Getenv("PROGRAMDATA")
	if pd == "" {
		pd = `C:\ProgramData`
	}
	var inst struct {
		Associated map[string]string `json:"associated_client"`
	}
	if readJSON(filepath.Join(pd, "Riot Games", "RiotClientInstalls.json"), &inst) == nil {
		for k := range inst.Associated {
			dirs = append(dirs, filepath.FromSlash(k))
		}
	}
	for _, d := range []string{"C", "D", "E", "F", "G"} {
		dirs = append(dirs, d+`:\Riot Games\League of Legends`)
	}
	return dirs
}

var reAppPort = regexp.MustCompile(`--app-port=(\d+)`)
var reToken = regexp.MustCompile(`--remoting-auth-token=([\w-]+)`)

func (l *LCU) discover() bool {
	port, pass := "", ""
	for _, d := range candidateLeagueDirs() {
		if b, err := os.ReadFile(filepath.Join(d, "lockfile")); err == nil {
			if p, pw, ok := parseLockfile(b); ok {
				port, pass = p, pw
				break
			}
		}
	}
	if port == "" {
		// ultima spiaggia: riga di comando del processo LeagueClientUx
		out := processCommandLine("LeagueClientUx.exe")
		if m := reAppPort.FindStringSubmatch(out); m != nil {
			if t := reToken.FindStringSubmatch(out); t != nil {
				port, pass = m[1], t[1]
			}
		}
	}
	if port == "" {
		l.mu.Lock()
		l.connected = false
		l.mu.Unlock()
		return false
	}
	l.mu.Lock()
	l.port = port
	l.auth = "Basic " + base64.StdEncoding.EncodeToString([]byte("riot:"+pass))
	l.mu.Unlock()
	_, code, err := l.req("GET", "/lol-summoner/v1/current-summoner", nil)
	ok := err == nil && code == 200
	l.mu.Lock()
	l.connected = ok
	l.mu.Unlock()
	return ok
}

func (l *LCU) req(method, path string, body any) ([]byte, int, error) {
	l.mu.RLock()
	port, auth := l.port, l.auth
	l.mu.RUnlock()
	if port == "" {
		return nil, 0, fmt.Errorf("client non connesso")
	}
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	r, err := http.NewRequest(method, "https://127.0.0.1:"+port+path, rd)
	if err != nil {
		return nil, 0, err
	}
	r.Header.Set("Authorization", auth)
	r.Header.Set("Accept", "application/json")
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	resp, err := l.client.Do(r)
	if err != nil {
		l.mu.Lock()
		l.connected = false
		l.mu.Unlock()
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return b, resp.StatusCode, err
}

func (l *LCU) getJSON(path string, v any) error {
	b, code, err := l.req("GET", path, nil)
	if err != nil {
		return err
	}
	if code != 200 {
		return fmt.Errorf("LCU %s: HTTP %d", path, code)
	}
	return json.Unmarshal(b, v)
}

// ---------- Stato condiviso del client ----------

type Summoner struct {
	GameName      string `json:"gameName"`
	TagLine       string `json:"tagLine"`
	Level         int    `json:"summonerLevel"`
	ProfileIconID int    `json:"profileIconId"`
	Puuid         string `json:"puuid"`
	SummonerID    int64  `json:"summonerId"`
}

type RankedQueue struct {
	QueueType    string `json:"queueType"`
	Tier         string `json:"tier"`
	Division     string `json:"division"`
	LeaguePoints int    `json:"leaguePoints"`
	Wins         int    `json:"wins"`
	Losses       int    `json:"losses"`
}

type ClientState struct {
	Connected bool                   `json:"connected"`
	Phase     string                 `json:"phase"`
	Summoner  *Summoner              `json:"summoner"`
	Ranked    map[string]RankedQueue `json:"ranked"`
	QueueID   int                    `json:"queueId"`
	ChampSel  *ChampSelect           `json:"champSelect"`
	Lobby     []LobbyPlayer          `json:"lobby"`
	Imported  string                 `json:"imported"`
}

type CSPlayer struct {
	CellID     int    `json:"cellId"`
	ChampionID int    `json:"championId"`
	Intent     int    `json:"championPickIntent"`
	Position   string `json:"assignedPosition"`
	Puuid      string `json:"puuid"`
	GameName   string `json:"gameName"`
	Spell1     int    `json:"spell1Id"`
	Spell2     int    `json:"spell2Id"`
}

type ChampSelect struct {
	MyTeam    []CSPlayer `json:"myTeam"`
	TheirTeam []CSPlayer `json:"theirTeam"`
	LocalCell int        `json:"localPlayerCellId"`
	Bans      struct {
		Mine   []int `json:"myTeamBans"`
		Theirs []int `json:"theirTeamBans"`
	} `json:"bans"`
	Actions [][]struct {
		Actor     int    `json:"actorCellId"`
		Champion  int    `json:"championId"`
		Completed bool   `json:"completed"`
		Type      string `json:"type"`
	} `json:"actions"`
	Timer struct {
		Phase    string  `json:"phase"`
		TimeLeft float64 `json:"adjustedTimeLeftInPhase"`
	} `json:"timer"`
	// calcolati
	MyChampion int    `json:"myChampion"`
	MyPosition string `json:"myPosition"`
	Locked     bool   `json:"locked"`
}

type LobbyPlayer struct {
	Puuid    string `json:"puuid"`
	Name     string `json:"name"`
	Champion int    `json:"champion"`
	Team     int    `json:"team"` // 1 = alleati, 2 = nemici
	Tier     string `json:"tier"`
	Division string `json:"division"`
	LP       int    `json:"lp"`
	Wins     int    `json:"wins"`
	Losses   int    `json:"losses"`
}

var (
	stateMu      sync.RWMutex
	clientState  = ClientState{Ranked: map[string]RankedQueue{}}
	lastImported string
	rankCache    = map[string]RankedQueue{}
)

func getClientState() ClientState {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return clientState
}

func fetchRanked(path string) map[string]RankedQueue {
	var r struct {
		QueueMap map[string]RankedQueue `json:"queueMap"`
		Queues   []RankedQueue          `json:"queues"`
	}
	out := map[string]RankedQueue{}
	if lcu.getJSON(path, &r) != nil {
		return out
	}
	for k, v := range r.QueueMap {
		v.QueueType = k
		out[k] = v
	}
	for _, q := range r.Queues {
		if _, ok := out[q.QueueType]; !ok && q.QueueType != "" {
			out[q.QueueType] = q
		}
	}
	return out
}

func lcuLoop() {
	lastProfile := time.Time{}
	for {
		if !lcu.isConnected() {
			if !lcu.discover() {
				stateMu.Lock()
				clientState.Connected = false
				clientState.Phase = "None"
				clientState.ChampSel = nil
				stateMu.Unlock()
				time.Sleep(4 * time.Second)
				continue
			}
		}
		var phase string
		if err := lcu.getJSON("/lol-gameflow/v1/gameflow-phase", &phase); err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		stateMu.Lock()
		clientState.Connected = true
		clientState.Phase = phase
		stateMu.Unlock()

		if time.Since(lastProfile) > 30*time.Second || getClientState().Summoner == nil {
			var s Summoner
			if lcu.getJSON("/lol-summoner/v1/current-summoner", &s) == nil {
				rk := fetchRanked("/lol-ranked/v1/current-ranked-stats")
				stateMu.Lock()
				clientState.Summoner = &s
				clientState.Ranked = rk
				stateMu.Unlock()
				lastProfile = time.Now()
			}
		}

		var gs struct {
			GameData struct {
				Queue struct {
					ID int `json:"id"`
				} `json:"queue"`
				TeamOne []struct {
					Puuid    string `json:"puuid"`
					Champion int    `json:"championId"`
					Name     string `json:"gameName"`
					Summ     string `json:"summonerName"`
				} `json:"teamOne"`
				TeamTwo []struct {
					Puuid    string `json:"puuid"`
					Champion int    `json:"championId"`
					Name     string `json:"gameName"`
					Summ     string `json:"summonerName"`
				} `json:"teamTwo"`
			} `json:"gameData"`
		}
		if phase == "ChampSelect" || phase == "InProgress" || phase == "Lobby" || phase == "Matchmaking" || phase == "ReadyCheck" {
			if lcu.getJSON("/lol-gameflow/v1/session", &gs) == nil {
				stateMu.Lock()
				clientState.QueueID = gs.GameData.Queue.ID
				stateMu.Unlock()
			}
		}

		switch phase {
		case "ChampSelect":
			var cs ChampSelect
			if lcu.getJSON("/lol-champ-select/v1/session", &cs) == nil {
				finishChampSelect(&cs)
				stateMu.Lock()
				clientState.ChampSel = &cs
				stateMu.Unlock()
				maybeAutoImport(&cs)
			}
			time.Sleep(1 * time.Second)
			continue
		case "InProgress":
			// giocatori della partita per le statistiche della schermata di caricamento
			if len(getClientState().Lobby) == 0 && (len(gs.GameData.TeamOne) > 0 || len(gs.GameData.TeamTwo) > 0) {
				me := ""
				if s := getClientState().Summoner; s != nil {
					me = s.Puuid
				}
				myTeam := 1
				for _, p := range gs.GameData.TeamTwo {
					if p.Puuid == me {
						myTeam = 2
					}
				}
				var lobby []LobbyPlayer
				add := func(team int, puuid, name, summ string, champ int) {
					rel := 2
					if team == myTeam {
						rel = 1
					}
					if name == "" {
						name = summ
					}
					lp := LobbyPlayer{Puuid: puuid, Name: name, Champion: champ, Team: rel}
					if puuid != "" {
						rq, ok := rankCache[puuid]
						if !ok {
							rq = fetchRanked("/lol-ranked/v1/ranked-stats/" + puuid)["RANKED_SOLO_5x5"]
							rankCache[puuid] = rq
						}
						lp.Tier, lp.Division, lp.LP, lp.Wins, lp.Losses = rq.Tier, rq.Division, rq.LeaguePoints, rq.Wins, rq.Losses
					}
					lobby = append(lobby, lp)
				}
				for _, p := range gs.GameData.TeamOne {
					add(1, p.Puuid, p.Name, p.Summ, p.Champion)
				}
				for _, p := range gs.GameData.TeamTwo {
					add(2, p.Puuid, p.Name, p.Summ, p.Champion)
				}
				stateMu.Lock()
				clientState.Lobby = lobby
				stateMu.Unlock()
			}
		default:
			stateMu.Lock()
			clientState.ChampSel = nil
			if phase != "InProgress" && phase != "GameStart" {
				clientState.Lobby = nil
			}
			stateMu.Unlock()
			if phase == "EndOfGame" || phase == "Lobby" {
				if phase == "EndOfGame" {
					profMu.Lock()
					delete(profCache, "me")
					profileVer.Add(1) // la partita appena finita compare subito nel profilo
					profMu.Unlock()
				}
				lastImported = ""
				stateMu.Lock()
				clientState.Imported = ""
				stateMu.Unlock()
			}
		}
		time.Sleep(2 * time.Second)
	}
}

func finishChampSelect(cs *ChampSelect) {
	for _, p := range cs.MyTeam {
		if p.CellID == cs.LocalCell {
			cs.MyChampion = p.ChampionID
			if cs.MyChampion == 0 {
				cs.MyChampion = p.Intent
			}
			cs.MyPosition = strings.ToUpper(p.Position)
		}
	}
	for _, grp := range cs.Actions {
		for _, a := range grp {
			if a.Actor == cs.LocalCell && a.Type == "pick" && a.Completed && a.Champion != 0 {
				cs.Locked = true
				cs.MyChampion = a.Champion
			}
		}
	}
}

func lcuPosToRiot(p string) string {
	switch strings.ToUpper(p) {
	case "TOP":
		return "TOP"
	case "JUNGLE":
		return "JUNGLE"
	case "MIDDLE", "MID":
		return "MIDDLE"
	case "BOTTOM", "ADC":
		return "BOTTOM"
	case "UTILITY", "SUPPORT":
		return "UTILITY"
	}
	return ""
}

func maybeAutoImport(cs *ChampSelect) {
	if !cs.Locked || cs.MyChampion == 0 {
		return
	}
	key := fmt.Sprintf("%d|%s", cs.MyChampion, cs.MyPosition)
	if key == lastImported {
		return
	}
	lastImported = key
	s := getSettings()
	if !s.AutoRunes && !s.AutoSpells && !s.AutoItems {
		return
	}
	mode := queueGroup(getClientState().QueueID, "")
	res := importBuild(cs.MyChampion, lcuPosToRiot(cs.MyPosition), mode, s.AutoRunes, s.AutoSpells, s.AutoItems, 0)
	stateMu.Lock()
	clientState.Imported = res
	stateMu.Unlock()
}

// ---------- Import di rune, incantesimi e set di oggetti ----------

func importBuild(champ int, pos, mode string, runes, spells, items bool, variant int) string {
	if !lcu.isConnected() {
		return "Client di League non connesso"
	}
	return importResolved(champ, pos, resolveBuild(champ, pos, mode, "", true, variant), runes, spells, items)
}

// importa una build già pronta (es. quella di un pro scelta in champion select)
func importResolved(champ int, pos string, b BuildResult, runes, spells, items bool) string {
	if !lcu.isConnected() {
		return "Client di League non connesso"
	}
	var done []string
	var errs []string
	if runes {
		if b.Runes != nil {
			if err := importRunes(champ, b.Runes); err != nil {
				errs = append(errs, "rune: "+err.Error())
			} else {
				done = append(done, "rune")
			}
		} else {
			errs = append(errs, "nessuna pagina di rune disponibile")
		}
	}
	if spells && len(b.Spells) == 2 {
		s1, s2 := b.Spells[0], b.Spells[1]
		// posizione del Flash scelta nelle impostazioni (D = primo slot, F = secondo)
		fk := getSettings().FlashKey
		if (fk == "F" && s1 == 4) || (fk == "D" && s2 == 4) {
			s1, s2 = s2, s1
		}
		_, code, err := lcu.req("PATCH", "/lol-champ-select/v1/session/my-selection", map[string]int{"spell1Id": s1, "spell2Id": s2})
		if err != nil || code >= 300 {
			errs = append(errs, "incantesimi")
		} else {
			done = append(done, "incantesimi")
		}
	}
	if items {
		if err := importItemSet(champ, pos, b); err != nil {
			errs = append(errs, "oggetti: "+err.Error())
		} else {
			done = append(done, "oggetti")
		}
	}
	name := champName(champ)
	msg := ""
	if len(done) > 0 {
		msg = "Importati per " + name + ": " + strings.Join(done, ", ")
	}
	if len(errs) > 0 {
		if msg != "" {
			msg += " · "
		}
		msg += "Problemi: " + strings.Join(errs, ", ")
	}
	return msg
}

func importRunes(champ int, r *RunePage) error {
	var pages []struct {
		ID        int64  `json:"id"`
		Name      string `json:"name"`
		Deletable bool   `json:"isDeletable"`
		Current   bool   `json:"current"`
	}
	lcu.getJSON("/lol-perks/v1/pages", &pages)
	for _, p := range pages {
		if strings.HasPrefix(p.Name, "BetterStomp") && p.Deletable {
			lcu.req("DELETE", fmt.Sprintf("/lol-perks/v1/pages/%d", p.ID), nil)
		}
	}
	page := map[string]any{
		"name":            "BetterStomp " + champName(champ),
		"primaryStyleId":  r.Primary,
		"subStyleId":      r.Secondary,
		"selectedPerkIds": r.Perks,
		"current":         true,
	}
	_, code, err := lcu.req("POST", "/lol-perks/v1/pages", page)
	if err == nil && code >= 400 {
		// limite di pagine raggiunto: sostituisco la pagina corrente (se modificabile)
		var cur struct {
			ID        int64 `json:"id"`
			Deletable bool  `json:"isDeletable"`
		}
		if lcu.getJSON("/lol-perks/v1/currentpage", &cur) == nil && cur.Deletable {
			lcu.req("DELETE", fmt.Sprintf("/lol-perks/v1/pages/%d", cur.ID), nil)
			_, code, err = lcu.req("POST", "/lol-perks/v1/pages", page)
		}
	}
	if err != nil {
		return err
	}
	if code >= 400 {
		return fmt.Errorf("HTTP %d", code)
	}
	return nil
}

func importItemSet(champ int, pos string, b BuildResult) error {
	s := getClientState().Summoner
	if s == nil {
		return fmt.Errorf("evocatore sconosciuto")
	}
	path := fmt.Sprintf("/lol-item-sets/v1/item-sets/%d/sets", s.SummonerID)
	var cur map[string]any
	if err := lcu.getJSON(path, &cur); err != nil {
		return err
	}
	sets, _ := cur["itemSets"].([]any)
	var keep []any
	for _, it := range sets {
		m, _ := it.(map[string]any)
		if t, _ := m["title"].(string); strings.HasPrefix(t, "BetterStomp") {
			continue
		}
		keep = append(keep, it)
	}
	block := func(title string, ids []int, max int) map[string]any {
		var items []map[string]any
		for i, id := range ids {
			if i >= max {
				break
			}
			items = append(items, map[string]any{"id": fmt.Sprint(id), "count": 1})
		}
		if items == nil {
			items = []map[string]any{}
		}
		return map[string]any{"type": title, "items": items}
	}
	blocks := []any{
		block("Inizio partita", b.Starting, 5),
		block("Build principale", b.Core, 10),
		block("Situazionali", b.Situational, 10),
	}
	set := map[string]any{
		"title":               "BetterStomp " + champName(champ) + " " + posLabel(pos),
		"associatedChampions": []int{champ},
		"associatedMaps":      []int{},
		"blocks":              blocks,
		"map":                 "any",
		"mode":                "any",
		"type":                "custom",
		"sortrank":            0,
		"startedFrom":         "blank",
		"uid":                 fmt.Sprintf("betterstomp-%d-%d", champ, time.Now().Unix()),
		"preferredItemSlots":  []any{},
	}
	keep = append([]any{set}, keep...)
	cur["itemSets"] = keep
	_, code, err := lcu.req("PUT", path, cur)
	if err != nil {
		return err
	}
	if code >= 400 {
		return fmt.Errorf("HTTP %d", code)
	}
	return nil
}

func posLabel(p string) string {
	switch p {
	case "TOP":
		return "Top"
	case "JUNGLE":
		return "Jungle"
	case "MIDDLE":
		return "Mid"
	case "BOTTOM":
		return "ADC"
	case "UTILITY":
		return "Support"
	}
	return ""
}

// Pagine di rune consigliate da Riot (dal client), usate quando non ci sono statistiche
func lcuRecommendedRunes(champ int, pos string) *RunePage {
	if !lcu.isConnected() {
		return nil
	}
	p := map[string]string{"TOP": "top", "JUNGLE": "jungle", "MIDDLE": "middle", "BOTTOM": "bottom", "UTILITY": "utility"}[pos]
	if p == "" {
		p = "none"
	}
	var pages []struct {
		Perks []struct {
			ID int `json:"id"`
		} `json:"perks"`
		Primary   int   `json:"primaryPerkStyleId"`
		Secondary int   `json:"secondaryPerkStyleId"`
		Spells    []int `json:"summonerSpellIds"`
	}
	if lcu.getJSON(fmt.Sprintf("/lol-perks/v1/recommended-pages/champion/%d/position/%s/map/11", champ, p), &pages) != nil || len(pages) == 0 {
		return nil
	}
	pg := pages[0]
	r := &RunePage{Primary: pg.Primary, Secondary: pg.Secondary, Source: "Consigliate da Riot", Spells: pg.Spells}
	for _, x := range pg.Perks {
		r.Perks = append(r.Perks, x.ID)
	}
	if len(r.Perks) < 9 || r.Primary == 0 {
		return nil
	}
	return r
}
