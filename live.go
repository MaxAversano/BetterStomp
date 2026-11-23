package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ---------- Live Client Data API (partita in corso, https://127.0.0.1:2999) ----------

type LivePlayer struct {
	ChampionName string  `json:"championName"`
	RiotID       string  `json:"riotId"`
	RiotName     string  `json:"riotIdGameName"`
	SummonerName string  `json:"summonerName"`
	Team         string  `json:"team"`
	Position     string  `json:"position"`
	Level        int     `json:"level"`
	IsDead       bool    `json:"isDead"`
	RespawnTimer float64 `json:"respawnTimer"`
	Scores       struct {
		Kills      int `json:"kills"`
		Deaths     int `json:"deaths"`
		Assists    int `json:"assists"`
		CreepScore int `json:"creepScore"`
	} `json:"scores"`
	Items []struct {
		ItemID int `json:"itemID"`
		Count  int `json:"count"`
		Price  int `json:"price"`
	} `json:"items"`
}

type LiveEvent struct {
	EventID    int     `json:"EventID"`
	EventName  string  `json:"EventName"`
	EventTime  float64 `json:"EventTime"`
	KillerName string  `json:"KillerName"`
	DragonType string  `json:"DragonType"`
}

type LiveData struct {
	ActivePlayer struct {
		SummonerName string `json:"summonerName"`
		RiotID       string `json:"riotId"`
		RiotName     string `json:"riotIdGameName"`
	} `json:"activePlayer"`
	AllPlayers []LivePlayer `json:"allPlayers"`
	Events     struct {
		Events []LiveEvent `json:"Events"`
	} `json:"events"`
	GameData struct {
		GameMode  string  `json:"gameMode"`
		GameTime  float64 `json:"gameTime"`
		MapNumber int     `json:"mapNumber"`
	} `json:"gameData"`
}

var liveClient = &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}}

type LiveState struct {
	mu       sync.RWMutex
	data     *LiveData
	at       time.Time
	camps    map[string]float64 // prossimo respawn (tempo di gioco) segnato a mano
	relics   map[int]float64
	gameKey  string
	showLoad bool // Ctrl+T
	tabHeld  bool
}

var live = &LiveState{camps: map[string]float64{}, relics: map[int]float64{}}

func liveLoop() {
	for {
		phase := getClientState().Phase
		interval := 3 * time.Second
		if phase == "InProgress" || !lcu.isConnected() {
			interval = time.Second
		}
		resp, err := liveClient.Get("https://127.0.0.1:2999/liveclientdata/allgamedata")
		if err == nil {
			b, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			var d LiveData
			if resp.StatusCode == 200 && json.Unmarshal(b, &d) == nil && len(d.AllPlayers) > 0 {
				live.mu.Lock()
				key := ""
				for _, p := range d.AllPlayers {
					key += p.ChampionName
				}
				if key != live.gameKey || d.GameData.GameTime < 1 && live.data != nil && live.data.GameData.GameTime > 30 {
					live.gameKey = key
					live.camps = map[string]float64{}
					live.relics = map[int]float64{}
				}
				live.data = &d
				live.at = time.Now()
				live.mu.Unlock()
			} else {
				live.clearIfStale()
			}
		} else {
			live.clearIfStale()
		}
		time.Sleep(interval)
	}
}

func (l *LiveState) clearIfStale() {
	l.mu.Lock()
	if time.Since(l.at) > 6*time.Second {
		l.data = nil
	}
	l.mu.Unlock()
}

func (l *LiveState) snapshot() (*LiveData, float64) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.data == nil {
		return nil, 0
	}
	// tempo di gioco interpolato tra un aggiornamento e l'altro
	gt := l.data.GameData.GameTime + time.Since(l.at).Seconds()
	return l.data, gt
}

// segna un campo della giungla come appena ripulito (hotkey)
func (l *LiveState) markCamp(side string, idx int) {
	d, gt := l.snapshot()
	if d == nil {
		return
	}
	s := getSettings().Overlay.Timers
	if d.GameData.MapNumber == 12 {
		l.mu.Lock()
		if idx >= 1 && idx <= 4 {
			l.relics[idx] = gt + s.RelicRespawn
		}
		l.mu.Unlock()
		return
	}
	if idx < 1 || idx > len(campDefs) {
		return
	}
	c := campDefs[idx-1]
	resp := s.CampRespawn
	if c.Buff {
		resp = s.BuffRespawn
	}
	key := side + ":" + c.Key
	l.mu.Lock()
	if v, ok := l.camps[key]; ok && v > gt+resp-3 {
		delete(l.camps, key) // doppia pressione = annulla
	} else {
		l.camps[key] = gt + resp
	}
	l.mu.Unlock()
}

type campDef struct {
	Key, Name string
	Buff      bool
	// posizione sulla mappa (lato blu); il lato rosso è speculare
	X, Y float64
}

var campDefs = []campDef{
	{"blue", "Blu", true, 3830, 7930},
	{"gromp", "Gromp", false, 2150, 8400},
	{"wolves", "Lupi", false, 3800, 6500},
	{"raptors", "Pappagalli", false, 6950, 5450},
	{"red", "Rosso", true, 7770, 4050},
	{"krugs", "Sassi", false, 8400, 2700},
}

const mapSize = 14870.0

// ---------- Modello overlay (usato sia dall'overlay nativo sia dall'anteprima) ----------

type OLine struct {
	Text  string `json:"text"`
	Right string `json:"right,omitempty"`
	Color string `json:"color"`
	Bold  bool   `json:"bold,omitempty"`
	Size  int    `json:"size,omitempty"`
}

type OWidget struct {
	ID     string  `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Anchor string  `json:"anchor,omitempty"` // "center" per centrare orizzontalmente
	Title  string  `json:"title,omitempty"`
	Accent string  `json:"accent"`
	Width  int     `json:"width"`
	Lines  []OLine `json:"lines"`
	Pill   bool    `json:"pill,omitempty"`  // etichetta piccola sulla minimappa
	Pixel  bool    `json:"pixel,omitempty"` // X/Y in pixel dal basso-destra (minimappa)
}

type OverlayModel struct {
	Active  bool      `json:"active"`
	Mode    string    `json:"mode"`
	Time    float64   `json:"time"`
	Widgets []OWidget `json:"widgets"`
}

func mmss(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	s := int(math.Ceil(sec))
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

const (
	colText   = "#EEF0FF"
	colMuted  = "#9AA3C7"
	colGood   = "#3EE6A0"
	colBad    = "#FF5C7A"
	colGold   = "#F5C451"
	colAccent = "#8B6CFF"
	colCyan   = "#38D6FF"
)

func samePlayer(p LivePlayer, id, name string) bool {
	return (id != "" && (p.RiotID == id || p.SummonerName == id)) || (name != "" && (p.RiotName == name || p.SummonerName == name))
}

func buildOverlayModel(preview bool) OverlayModel {
	s := getSettings()
	o := s.Overlay
	d, gt := live.snapshot()
	m := OverlayModel{}
	if d == nil {
		if preview {
			d, gt = demoLive(), 754
		} else {
			// schermata di caricamento: statistiche delle squadre
			cs := getClientState()
			if o.Loading.On && cs.Phase == "InProgress" && len(cs.Lobby) > 0 {
				m.Active = true
				m.Widgets = append(m.Widgets, loadingWidget(cs.Lobby, o))
			}
			return m
		}
	}
	m.Active, m.Time = true, gt
	m.Mode = d.GameData.GameMode
	aram := d.GameData.MapNumber == 12

	// giocatore attivo e squadra
	var me *LivePlayer
	for i := range d.AllPlayers {
		if samePlayer(d.AllPlayers[i], d.ActivePlayer.RiotID, d.ActivePlayer.RiotName) || samePlayer(d.AllPlayers[i], d.ActivePlayer.SummonerName, "") {
			me = &d.AllPlayers[i]
		}
	}
	myTeam := "ORDER"
	if me != nil {
		myTeam = me.Team
	}
	teamOf := map[string]string{}
	for _, p := range d.AllPlayers {
		teamOf[p.RiotID] = p.Team
		teamOf[p.SummonerName] = p.Team
		teamOf[p.RiotName] = p.Team
	}
	killerTeam := func(name string) string {
		if t, ok := teamOf[name]; ok {
			return t
		}
		if i := strings.Index(name, "#"); i > 0 {
			if t, ok := teamOf[name[:i]]; ok {
				return t
			}
		}
		return ""
	}

	// CS in tempo reale
	if o.CS.On && me != nil && !aram {
		csmin := 0.0
		if gt > 60 {
			csmin = float64(me.Scores.CreepScore) / (gt / 60)
		}
		m.Widgets = append(m.Widgets, OWidget{ID: "cs", X: o.CS.X, Y: o.CS.Y, Accent: colCyan, Width: 150, Lines: []OLine{
			{Text: "CS", Right: fmt.Sprintf("%d", me.Scores.CreepScore), Color: colText, Bold: true, Size: 18},
			{Text: "al minuto", Right: fmt.Sprintf("%.1f", csmin), Color: colMuted},
		}})
	}

	t := o.Timers
	// obiettivi epici dagli eventi di gioco
	dragonNext, baronNext, heraldDone, grubsDone, atakhanDone := t.DragonFirst, t.BaronFirst, false, false, false
	drakes := map[string]int{}
	elderPhase := false
	var baronBuffEnd, elderBuffEnd float64
	grubs := 0
	for _, ev := range d.Events.Events {
		switch {
		case ev.EventName == "DragonKill":
			kt := killerTeam(ev.KillerName)
			if strings.EqualFold(ev.DragonType, "Elder") {
				dragonNext = ev.EventTime + t.ElderRespawn
				if kt == myTeam {
					elderBuffEnd = ev.EventTime + t.ElderBuff
				}
			} else {
				drakes[kt]++
				if drakes[kt] >= 4 {
					elderPhase = true
					dragonNext = ev.EventTime + t.ElderRespawn
				} else {
					dragonNext = ev.EventTime + t.DragonRespawn
				}
			}
		case ev.EventName == "BaronKill":
			baronNext = ev.EventTime + t.BaronRespawn
			if killerTeam(ev.KillerName) == myTeam {
				baronBuffEnd = ev.EventTime + t.BaronBuff
			}
		case ev.EventName == "HeraldKill":
			heraldDone = true
		case ev.EventName == "HordeKill":
			grubs++
			if grubs >= 3 {
				grubsDone = true
			}
		case strings.Contains(ev.EventName, "Atakhan"):
			atakhanDone = true
		}
	}
	timerLine := func(name string, at float64, col string) OLine {
		if gt >= at {
			return OLine{Text: name, Right: "VIVO", Color: colGood, Bold: true}
		}
		c := colText
		if at-gt <= 60 {
			c = colGold
		}
		return OLine{Text: name, Right: mmss(at - gt), Color: c}
	}
	if o.Objectives.On && !aram {
		var lines []OLine
		dn := "Drago"
		if elderPhase {
			dn = "Drago Antico"
		}
		lines = append(lines, timerLine(dn, dragonNext, colCyan))
		if !grubsDone && gt < t.HeraldFirst {
			lines = append(lines, timerLine("Larve del Vuoto", t.GrubsFirst, colAccent))
		}
		if !heraldDone && gt < t.HeraldFirst+300 {
			lines = append(lines, timerLine("Araldo", t.HeraldFirst, colAccent))
		}
		if !atakhanDone && t.AtakhanFirst > 0 && gt < t.AtakhanFirst+300 && gt > t.HeraldFirst-120 {
			lines = append(lines, timerLine("Atakhan", t.AtakhanFirst, colBad))
		}
		lines = append(lines, timerLine("Barone", baronNext, colAccent))
		m.Widgets = append(m.Widgets, OWidget{ID: "objectives", X: o.Objectives.X, Y: o.Objectives.Y, Title: "OBIETTIVI", Accent: colAccent, Width: 190, Lines: lines})
	}

	// scadenza buff Barone / Drago Antico
	if o.Buffs.On {
		var lines []OLine
		if baronBuffEnd > gt {
			c := colAccent
			if baronBuffEnd-gt < 30 {
				c = colBad
			}
			lines = append(lines, OLine{Text: "Buff Barone", Right: mmss(baronBuffEnd - gt), Color: c, Bold: true})
		}
		if elderBuffEnd > gt {
			c := colGold
			if elderBuffEnd-gt < 30 {
				c = colBad
			}
			lines = append(lines, OLine{Text: "Buff Drago Antico", Right: mmss(elderBuffEnd - gt), Color: c, Bold: true})
		}
		if len(lines) > 0 {
			m.Widgets = append(m.Widgets, OWidget{ID: "buffs", X: o.Buffs.X, Y: o.Buffs.Y, Anchor: "center", Title: "BUFF IN SCADENZA", Accent: colGold, Width: 230, Lines: lines})
		}
	}

	// campi della giungla: primo spawn automatico, respawn segnati con hotkey
	live.mu.RLock()
	camps := map[string]float64{}
	for k, v := range live.camps {
		camps[k] = v
	}
	relics := map[int]float64{}
	for k, v := range live.relics {
		relics[k] = v
	}
	showLoad, tabHeld := live.showLoad, live.tabHeld
	live.mu.RUnlock()

	campAt := func(side string, c campDef) float64 {
		if v, ok := camps[side+":"+c.Key]; ok {
			return v
		}
		if c.Buff {
			return t.BuffFirst
		}
		return t.CampFirst
	}
	if !aram && o.Camps.On {
		var lines []OLine
		for i, c := range campDefs {
			lines = append(lines, timerLine(fmt.Sprintf("%d · %s", i+1, c.Name), campAt("ally", c), colText))
		}
		var enemy []OLine
		for i, c := range campDefs {
			if v, ok := camps["enemy:"+c.Key]; ok && v > gt {
				enemy = append(enemy, timerLine(fmt.Sprintf("%d · %s", i+1, c.Name), v, colBad))
			}
		}
		if len(enemy) > 0 {
			lines = append(lines, OLine{Text: "Lato nemico", Color: colBad, Bold: true})
			lines = append(lines, enemy...)
		}
		m.Widgets = append(m.Widgets, OWidget{ID: "camps", X: o.Camps.X, Y: o.Camps.Y, Title: "GIUNGLA", Accent: colGood, Width: 190, Lines: lines})
	}

	// timer sulla minimappa, direttamente sopra i campi
	if !aram && o.Minimap {
		size := float64(o.MinimapSize)
		place := func(mx, my float64) (float64, float64) {
			// pixel dal bordo destro / inferiore dello schermo
			px := float64(o.MinimapRight) + size - mx/mapSize*size
			py := float64(o.MinimapBottom) + my/mapSize*size
			return px, py
		}
		addPill := func(id string, mx, my, at float64, col string) {
			if gt >= at {
				return
			}
			x, y := place(mx, my)
			c := col
			if at-gt <= 30 {
				c = colGold
			}
			m.Widgets = append(m.Widgets, OWidget{ID: id, X: x, Y: y, Pixel: true, Pill: true, Accent: c, Lines: []OLine{{Text: mmss(at - gt), Color: c, Bold: true, Size: 11}}})
		}
		for _, c := range campDefs {
			allyX, allyY := c.X, c.Y
			enX, enY := mapSize-c.X, mapSize-c.Y
			if myTeam == "CHAOS" {
				allyX, allyY, enX, enY = enX, enY, allyX, allyY
			}
			addPill("mm-a-"+c.Key, allyX, allyY, campAt("ally", c), colText)
			if v, ok := camps["enemy:"+c.Key]; ok {
				addPill("mm-e-"+c.Key, enX, enY, v, colBad)
			}
		}
		addPill("mm-dragon", 9866, 4414, dragonNext, colCyan)
		addPill("mm-baron", 5007, 10471, baronNext, colAccent)
	}

	// ARAM: timer delle vite (reliquie salute, segnate con hotkey) e respawn dei giocatori
	if aram && o.Deaths.On {
		var lines []OLine
		for i := 1; i <= 4; i++ {
			at, ok := relics[i]
			if !ok {
				at = t.RelicFirst
			}
			lines = append(lines, timerLine(fmt.Sprintf("Vita %d", i), at, colGood))
		}
		m.Widgets = append(m.Widgets, OWidget{ID: "relics", X: o.Camps.X, Y: o.Camps.Y, Title: "VITE ARAM", Accent: colGood, Width: 170, Lines: lines})
	}
	if o.Deaths.On {
		var lines []OLine
		for _, p := range d.AllPlayers {
			if p.IsDead && p.RespawnTimer > 0 {
				c := colGood
				if p.Team != myTeam {
					c = colBad
				}
				lines = append(lines, OLine{Text: p.ChampionName, Right: fmt.Sprintf("%.0fs", p.RespawnTimer), Color: c})
			}
		}
		if len(lines) > 0 {
			m.Widgets = append(m.Widgets, OWidget{ID: "deaths", X: o.Deaths.X, Y: o.Deaths.Y, Title: "RESPAWN", Accent: colBad, Width: 170, Lines: lines})
		}
	}

	// valore oggetti: si vede tenendo premuto TAB
	if o.ItemDiff.On && (tabHeld || preview) {
		val := func(p LivePlayer) int {
			t := 0
			for _, it := range p.Items {
				if info, ok := itemInfo(it.ItemID); ok {
					t += info.Gold.Total * maxI(1, it.Count)
				} else {
					t += it.Price * maxI(1, it.Count)
				}
			}
			return t
		}
		ally, enemy := 0, 0
		var opp *LivePlayer
		for i, p := range d.AllPlayers {
			if p.Team == myTeam {
				ally += val(p)
			} else {
				enemy += val(p)
				if me != nil && p.Position != "" && p.Position == me.Position {
					opp = &d.AllPlayers[i]
				}
			}
		}
		lines := []OLine{diffLine("Squadra", ally-enemy)}
		if me != nil && opp != nil {
			lines = append(lines, diffLine("vs "+opp.ChampionName, val(*me)-val(*opp)))
		}
		m.Widgets = append(m.Widgets, OWidget{ID: "itemdiff", X: o.ItemDiff.X, Y: o.ItemDiff.Y, Anchor: "center", Title: "VALORE OGGETTI", Accent: colGold, Width: 220, Lines: lines})
	}

	// statistiche squadre (Ctrl+T)
	if showLoad && o.Loading.On {
		if lobby := getClientState().Lobby; len(lobby) > 0 {
			m.Widgets = append(m.Widgets, loadingWidget(lobby, o))
		}
	}
	return m
}

func boolF(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

func maxI(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func diffLine(label string, v int) OLine {
	c, sign := colGood, "+"
	if v < 0 {
		c, sign = colBad, "-"
	}
	if v == 0 {
		c, sign = colMuted, "±"
	}
	av := v
	if av < 0 {
		av = -av
	}
	return OLine{Text: label, Right: fmt.Sprintf("%s%.1fk", sign, float64(av)/1000), Color: c, Bold: true}
}

var tierIT = map[string]string{"IRON": "Ferro", "BRONZE": "Bronzo", "SILVER": "Argento", "GOLD": "Oro", "PLATINUM": "Platino",
	"EMERALD": "Smeraldo", "DIAMOND": "Diamante", "MASTER": "Maestro", "GRANDMASTER": "Gran Maestro", "CHALLENGER": "Challenger"}

func rankShort(tier, div string) string {
	if tier == "" || tier == "NONE" || tier == "UNRANKED" {
		return "Non class."
	}
	n := tierIT[tier]
	if tier == "MASTER" || tier == "GRANDMASTER" || tier == "CHALLENGER" {
		return n
	}
	return n + " " + div
}

func loadingWidget(lobby []LobbyPlayer, o OverlaySettings) OWidget {
	var lines []OLine
	for _, team := range []int{1, 2} {
		if team == 1 {
			lines = append(lines, OLine{Text: "LA TUA SQUADRA", Color: colGood, Bold: true})
		} else {
			lines = append(lines, OLine{Text: "SQUADRA NEMICA", Color: colBad, Bold: true})
		}
		for _, p := range lobby {
			if p.Team != team {
				continue
			}
			wr := ""
			if g := p.Wins + p.Losses; g > 0 {
				wr = fmt.Sprintf(" · %d%% (%d)", 100*p.Wins/g, g)
			}
			name := p.Name
			if c := champName(p.Champion); p.Champion > 0 {
				name = c + "  " + name
			}
			lines = append(lines, OLine{Text: name, Right: rankShort(p.Tier, p.Division) + wr, Color: colText})
		}
	}
	return OWidget{ID: "loading", X: o.Loading.X, Y: o.Loading.Y, Anchor: "center", Title: "STATISTICHE SQUADRE  ·  Ctrl+T", Accent: colAccent, Width: 520, Lines: lines}
}

func demoLive() *LiveData {
	d := &LiveData{}
	d.ActivePlayer.RiotID = "Tu#EUW"
	d.GameData.GameTime = 754
	d.GameData.MapNumber = 11
	d.GameData.GameMode = "CLASSIC"
	mk := func(ch, id, team, pos string, cs int, items ...int) LivePlayer {
		p := LivePlayer{ChampionName: ch, RiotID: id, Team: team, Position: pos, Level: 11}
		p.Scores.CreepScore = cs
		for _, it := range items {
			p.Items = append(p.Items, struct {
				ItemID int `json:"itemID"`
				Count  int `json:"count"`
				Price  int `json:"price"`
			}{it, 1, 0})
		}
		return p
	}
	d.AllPlayers = []LivePlayer{
		mk("Ahri", "Tu#EUW", "ORDER", "MIDDLE", 104, 6655, 3020, 1052),
		mk("Lee Sin", "A#1", "ORDER", "JUNGLE", 80, 6692, 3047),
		mk("Syndra", "B#1", "CHAOS", "MIDDLE", 96, 6655, 3020),
		mk("Darius", "C#1", "CHAOS", "TOP", 110, 6631),
	}
	d.AllPlayers[3].IsDead = true
	d.AllPlayers[3].RespawnTimer = 21
	d.Events.Events = []LiveEvent{{EventName: "DragonKill", EventTime: 420, KillerName: "A#1", DragonType: "Fire"}}
	return d
}
