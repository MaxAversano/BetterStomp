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
	EventID     int     `json:"EventID"`
	EventName   string  `json:"EventName"`
	EventTime   float64 `json:"EventTime"`
	KillerName  string  `json:"KillerName"`
	InhibKilled string  `json:"InhibKilled"`
	DragonType  string  `json:"DragonType"`
}

type LiveData struct {
	ActivePlayer struct {
		CurrentGold  float64 `json:"currentGold"`
		SummonerName string  `json:"summonerName"`
		RiotID       string  `json:"riotId"`
		RiotName     string  `json:"riotIdGameName"`
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
			interval = 500 * time.Millisecond // in partita: due controlli al secondo
		}
		if d, _ := live.snapshot(); d != nil {
			interval = 500 * time.Millisecond
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
	Extra string `json:"extra,omitempty"` // valore di riferimento mostrato dopo " / "
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
	Key    string  `json:"key,omitempty"`   // impostazione da aggiornare quando lo trascini
	Tiles  []OTile `json:"tiles,omitempty"` // riquadri grandi (CS/min, partecipazione)
	Segs   []OLine `json:"segs,omitempty"`  // versione riassunta su una riga
	Dot    bool    `json:"dot,omitempty"`
	Icons  bool    `json:"icons,omitempty"`    // lucchetto, impostazioni, chiudi
	Locked bool    `json:"locked,omitempty"`   // bloccato con il lucchetto
	Text   bool    `json:"textOnly,omitempty"` // solo testo con contorno (minimappa)
	Card   *OCard  `json:"card,omitempty"`     // stile scheda (ritratto, KDA, CS, obiettivi)
	Teams  *OTeams `json:"teams,omitempty"`    // schermata di caricamento: due squadre affiancate
}

type OCard struct {
	Champ   string  `json:"champ"` // id Data Dragon per il ritratto
	KDA     string  `json:"kda"`
	CS      string  `json:"cs"`
	CSSub   string  `json:"csSub"` // CS/min e riferimento
	CSColor string  `json:"csColor"`
	Rows    []OLine `json:"rows"` // altre statistiche (GPM, KP, LVL)
	Objs    []OLine `json:"objs"` // timer obiettivi
}

type OTeams struct {
	Ally  []OTeamRow `json:"ally"`
	Enemy []OTeamRow `json:"enemy"`
}

type OTeamRow struct {
	Champ   string `json:"champ"` // id Data Dragon
	ChampNm string `json:"champName"`
	Name    string `json:"name"`
	Rank    string `json:"rank"`
	RankCol string `json:"rankColor"`
	WR      string `json:"wr"`
	WRCol   string `json:"wrColor"`
	Games   string `json:"games"`
	Me      bool   `json:"me"`
}

type OTile struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Sub   string `json:"sub,omitempty"`
	Color string `json:"color"`
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
	var objLines []OLine
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
		objLines = lines
	}
	live.mu.RLock()
	tabNow := live.tabHeld
	live.mu.RUnlock()
	cardOn := o.CS.On && me != nil && !o.Classic && !o.Compact && (o.BenchAlways || tabNow || preview)
	if o.CS.On && me != nil && (o.BenchAlways || tabNow || preview) {
		var objForCard []OLine
		if cardOn {
			objForCard = objLines
		}
		m.Widgets = append(m.Widgets, mainWidget(o, d, me, gt, objForCard))
	}
	if cardOn {
		objLines = nil // i timer sono già nella scheda
	}
	// notifiche obiettivi: promemoria prima dello spawn e avviso quando compare
	if o.Notify.On && !aram {
		type objT struct {
			name string
			at   float64
		}
		dn := "Drago"
		if elderPhase {
			dn = "Drago Antico"
		}
		objs := []objT{{dn, dragonNext}, {"Barone", baronNext}}
		if !heraldDone {
			objs = append(objs, objT{"Araldo", t.HeraldFirst})
		}
		if !grubsDone {
			objs = append(objs, objT{"Larve del Vuoto", t.GrubsFirst})
		}
		if !atakhanDone && t.AtakhanFirst > 0 {
			objs = append(objs, objT{"Atakhan", t.AtakhanFirst})
		}
		dur := o.NotifyDur
		if dur <= 0 {
			dur = 10
		}
		var msgs []OLine
		size := map[string]int{"small": 16, "medium": 20, "large": 26}[o.NotifySize]
		if size == 0 {
			size = 20
		}
		for _, ob := range objs {
			for _, r := range o.NotifyBefore {
				if r > 0 && gt >= ob.at-r && gt < ob.at-r+dur && gt < ob.at {
					msgs = append(msgs, OLine{Text: ob.name + " tra " + mmss(ob.at-gt), Color: colGold, Bold: true, Size: size})
					break
				}
			}
			if gt >= ob.at && gt < ob.at+dur && ob.at > 0 {
				msgs = append(msgs, OLine{Text: ob.name + " è comparso!", Color: colGood, Bold: true, Size: size})
			}
		}
		if preview && len(msgs) == 0 {
			msgs = append(msgs, OLine{Text: "Drago tra 0:58", Color: colGold, Bold: true, Size: size})
		}
		if len(msgs) > 0 {
			wd := map[string]int{"small": 240, "medium": 300, "large": 380}[o.NotifySize]
			if wd == 0 {
				wd = 300
			}
			m.Widgets = append(m.Widgets, OWidget{ID: "notify", Key: "notify", X: o.Notify.X, Y: o.Notify.Y, Anchor: "center", Title: "OBIETTIVO", Accent: colGold, Width: wd, Lines: msgs, Icons: true, Locked: o.Notify.Locked})
		}
	}
	if len(objLines) > 0 {
		m.Widgets = append(m.Widgets, OWidget{ID: "objectives", Key: "objectives", X: o.Objectives.X, Y: o.Objectives.Y, Title: "OBIETTIVI", Accent: colAccent, Width: 170, Lines: objLines, Icons: true, Locked: o.Objectives.Locked})
	}

	if preview && baronBuffEnd == 0 {
		baronBuffEnd = gt + 95 // esempio per l'anteprima
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
			m.Widgets = append(m.Widgets, OWidget{ID: "buffs", Key: "buffs", Icons: true, Locked: o.Buffs.Locked, X: o.Buffs.X, Y: o.Buffs.Y, Anchor: "center", Title: "BUFF IN SCADENZA", Accent: colGold, Width: 230, Lines: lines})
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
		m.Widgets = append(m.Widgets, OWidget{ID: "camps", Key: "camps", Icons: true, Locked: o.Camps.Locked, X: o.Camps.X, Y: o.Camps.Y, Title: "GIUNGLA", Accent: colGood, Width: 190, Lines: lines})
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
			m.Widgets = append(m.Widgets, OWidget{ID: id, X: x, Y: y, Pixel: true, Pill: true, Text: true, Accent: c, Lines: []OLine{{Text: mmssShort(at - gt), Color: c, Bold: true, Size: 11}}})
		}
		for _, c := range campDefs {
			col := "#F5B947" // campi della giungla in arancione, come su Blitz
			allyX, allyY := c.X, c.Y
			enX, enY := mapSize-c.X, mapSize-c.Y
			if myTeam == "CHAOS" {
				allyX, allyY, enX, enY = enX, enY, allyX, allyY
			}
			addPill("mm-a-"+c.Key, allyX, allyY, campAt("ally", c), col)
			if v, ok := camps["enemy:"+c.Key]; ok {
				addPill("mm-e-"+c.Key, enX, enY, v, col)
			}
		}
		addPill("mm-dragon", 9866, 4414, dragonNext, colText)
		addPill("mm-baron", 5007, 10471, baronNext, colText)
		// inibitori distrutti: tornano dopo 5 minuti
		for _, ev := range d.Events.Events {
			if ev.EventName == "InhibKilled" {
				if pos, ok := inhibPos(ev.InhibKilled); ok {
					addPill("mm-inhib-"+ev.InhibKilled, pos[0], pos[1], ev.EventTime+300, colText)
				}
			}
		}
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
		m.Widgets = append(m.Widgets, OWidget{ID: "relics", Key: "camps", Icons: true, Locked: o.Camps.Locked, X: o.Camps.X, Y: o.Camps.Y, Title: "VITE ARAM", Accent: colGood, Width: 170, Lines: lines})
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
			m.Widgets = append(m.Widgets, OWidget{ID: "deaths", Key: "deaths", Icons: true, Locked: o.Deaths.Locked, X: o.Deaths.X, Y: o.Deaths.Y, Title: "RESPAWN", Accent: colBad, Width: 170, Lines: lines})
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
		m.Widgets = append(m.Widgets, OWidget{ID: "itemdiff", Key: "itemDiff", Icons: true, Locked: o.ItemDiff.Locked, X: o.ItemDiff.X, Y: o.ItemDiff.Y, Anchor: "center", Title: "VALORE OGGETTI", Accent: colGold, Width: 220, Lines: lines})
	}

	if preview && o.Loading.On && len(getClientState().Lobby) == 0 {
		m.Widgets = append(m.Widgets, loadingWidget([]LobbyPlayer{
			{Name: "Tu", Champion: 103, Team: 1, Tier: "EMERALD", Division: "II", Wins: 61, Losses: 52},
			{Name: "Alleato", Champion: 64, Team: 1, Tier: "PLATINUM", Division: "I", Wins: 40, Losses: 38},
			{Name: "Avversario", Champion: 134, Team: 2, Tier: "EMERALD", Division: "III", Wins: 55, Losses: 50},
			{Name: "Nemico", Champion: 122, Team: 2, Tier: "DIAMOND", Division: "IV", Wins: 90, Losses: 81},
		}, o))
	}
	// statistiche squadre (Ctrl+T)
	if showLoad && !preview {
		lobby := getClientState().Lobby
		if len(lobby) == 0 {
			lobby = lobbyFromLive(d)
		}
		if len(lobby) > 0 {
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

var tierCol = map[string]string{"IRON": "#8C8C8C", "BRONZE": "#C0814F", "SILVER": "#AEB8C4", "GOLD": "#E6B450", "PLATINUM": "#3FC5B4",
	"EMERALD": "#2FD07A", "DIAMOND": "#6FA8FF", "MASTER": "#C56BFF", "GRANDMASTER": "#FF6464", "CHALLENGER": "#FFD86B"}

// schermata di caricamento: le due squadre affiancate, con rango e win rate della stagione
func loadingWidget(lobby []LobbyPlayer, o OverlaySettings) OWidget {
	t := &OTeams{}
	me := ""
	if s := getClientState().Summoner; s != nil {
		me = s.Puuid
	}
	for _, p := range lobby {
		r := OTeamRow{Name: p.Name, Rank: "Senza rango", RankCol: colMuted, WRCol: colMuted, Me: p.Puuid != "" && p.Puuid == me}
		if p.Champion > 0 {
			r.ChampNm = champName(p.Champion)
			static.mu.RLock()
			r.Champ = static.Champs[p.Champion].ID
			static.mu.RUnlock()
		}
		if p.Tier != "" && p.Tier != "NONE" {
			r.Rank = rankShort(p.Tier, p.Division)
			if p.Tier != "MASTER" && p.Tier != "GRANDMASTER" && p.Tier != "CHALLENGER" || p.LP > 0 {
				r.Rank += fmt.Sprintf(" · %d LP", p.LP)
			}
			r.RankCol = tierCol[p.Tier]
		}
		if g := p.Wins + p.Losses; g > 0 {
			wr := float64(p.Wins) / float64(g)
			r.WR = fmt.Sprintf("%d%%", int(wr*100+0.5))
			r.Games = fmt.Sprintf("%d partite", g)
			switch {
			case wr >= 0.55:
				r.WRCol = colGood
			case wr < 0.47:
				r.WRCol = colBad
			default:
				r.WRCol = colText
			}
		}
		if p.Team == 1 {
			t.Ally = append(t.Ally, r)
		} else {
			t.Enemy = append(t.Enemy, r)
		}
	}
	return OWidget{ID: "loading", Key: "loading", Icons: true, Locked: o.Loading.Locked, X: o.Loading.X, Y: o.Loading.Y, Anchor: "center", Title: "SQUADRE  ·  Ctrl+T", Accent: colAccent, Width: 760, Teams: t}
}

// lobby ricavata dalla partita in corso quando il client non la fornisce (niente rango)
func lobbyFromLive(d *LiveData) []LobbyPlayer {
	var me *LivePlayer
	for i := range d.AllPlayers {
		if samePlayer(d.AllPlayers[i], d.ActivePlayer.RiotID, d.ActivePlayer.RiotName) || samePlayer(d.AllPlayers[i], d.ActivePlayer.SummonerName, "") {
			me = &d.AllPlayers[i]
		}
	}
	var out []LobbyPlayer
	for _, p := range d.AllPlayers {
		team := 2
		if me != nil && p.Team == me.Team {
			team = 1
		}
		name := p.RiotName
		if name == "" {
			name = strings.SplitN(p.RiotID, "#", 2)[0]
		}
		out = append(out, LobbyPlayer{Name: name, Champion: champByName(champDDID(p.ChampionName)), Team: team})
	}
	return out
}

// "Lee Sin" / "game_character_displayname_LeeSin" → "LeeSin"
func champDDID(name string) string {
	if id := champByName(name); id > 0 {
		static.mu.RLock()
		defer static.mu.RUnlock()
		return static.Champs[id].ID
	}
	return strings.ReplaceAll(name, " ", "")
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

// ---------- riquadro principale: CS, partecipazione e timer obiettivi ----------

var (
	benchMu  sync.Mutex
	benchKey string
	benchVal Bench
	benchAt  time.Time
)

func benchFor(o OverlaySettings, me *LivePlayer) (Bench, string) {
	tier := o.CSGoalTier
	if tier == "" || tier == "auto" {
		tier = strings.TrimSuffix(autoTier(), "+")
	}
	key := me.ChampionName + "|" + me.Position + "|" + tier
	benchMu.Lock()
	defer benchMu.Unlock()
	if key != benchKey || time.Since(benchAt) > time.Minute {
		benchVal = engine.Benchmarks(champByName(me.ChampionName), me.Position, tier)
		benchKey, benchAt = key, time.Now()
	}
	return benchVal, tier
}

// livello atteso al minuto t (curva tipica di un giocatore di corsia)
func expectedLevel(mins float64) float64 {
	l := 1 + 0.78*mins
	if mins > 10 {
		l = 8.8 + 0.45*(mins-10)
	}
	if l > 18 {
		l = 18
	}
	return l
}

func fmt1(v float64) string {
	if v >= 100 {
		return fmt.Sprintf("%.0f", v)
	}
	s := fmt.Sprintf("%.1f", v)
	return strings.TrimSuffix(s, ".0")
}

func cmpCol(you, ref float64) string {
	if ref <= 0 {
		return colText
	}
	switch {
	case you >= ref:
		return colGood
	case you >= ref*0.85:
		return colText
	}
	return colBad
}

// riquadro "benchmark" in stile Blitz: il tuo valore / la media dell'elo scelto
func mainWidget(o OverlaySettings, d *LiveData, me *LivePlayer, gt float64, obj []OLine) OWidget {
	w := OWidget{ID: "main", Key: "cs", X: o.CS.X, Y: o.CS.Y, Title: "BETTERSTOMP", Accent: colCyan, Width: 170, Icons: true, Locked: o.CS.Locked}
	mins := gt / 60
	if mins < 0.01 {
		mins = 0.01
	}
	teamK, gold := 0, 0
	for _, p := range d.AllPlayers {
		if p.Team == me.Team {
			teamK += p.Scores.Kills
		}
	}
	for _, it := range me.Items {
		if info, ok := itemInfo(it.ItemID); ok {
			gold += info.Gold.Total * maxI(1, it.Count)
		}
	}
	earned := float64(gold) + d.ActivePlayer.CurrentGold - 500
	if earned < 0 {
		earned = 0
	}
	cs := effectiveCS(me)
	csm := float64(cs) / mins
	gpm := earned / mins
	kp := 0.0
	if teamK > 0 {
		kp = float64(me.Scores.Kills+me.Scores.Assists) * 100 / float64(teamK)
	}
	kda := float64(me.Scores.Kills+me.Scores.Assists) / float64(maxI(1, me.Scores.Deaths))

	var bCSM, bGPM, bKP, bKDA, bLVL float64
	hasBench := o.CSGoal != "off"
	if hasBench {
		b, _ := benchFor(o, me)
		ramp := 0.0
		if gt > 65 {
			ramp = (gt - 65) / gt // i minion arrivano dopo circa 1:05
		}
		bCSM = b.CSM * ramp
		bGPM = b.GPM * math.Min(1, 0.25+0.75*math.Min(1, gt/1200))
		bKP = b.KP * 100 * (0.35 + 0.65*math.Min(1, gt/900))
		bKDA = b.KDA * math.Min(1, gt/900)
		bLVL = expectedLevel(mins)
	}
	row := func(label string, you, ref float64, show bool) OLine {
		l := OLine{Text: label, Right: fmt1(you), Color: colText, Bold: true}
		if show {
			l.Extra = fmt1(ref)
			l.Color = cmpCol(you, ref)
		}
		return l
	}
	if o.BenchCSM {
		w.Lines = append(w.Lines, row("CSM", csm, bCSM, hasBench))
		if o.CSGoalValue > 0 {
			w.Lines = append(w.Lines, row("OBIETTIVO", csm, o.CSGoalValue, true))
		}
		w.Lines = append(w.Lines, OLine{Text: "CS", Right: fmt.Sprint(cs), Color: colText, Bold: true})
	}
	if o.BenchGPM {
		w.Lines = append(w.Lines, row("GPM", gpm, bGPM, hasBench))
	}
	if o.ShowKP {
		w.Lines = append(w.Lines, row("KP", kp, bKP, hasBench))
	}
	if o.BenchKDA {
		w.Lines = append(w.Lines, row("KDA", kda, bKDA, hasBench))
	}
	if o.BenchLVL {
		w.Lines = append(w.Lines, row("LVL", float64(me.Level), math.Floor(bLVL), hasBench))
	}
	if !o.Compact && !o.Classic {
		c := &OCard{Champ: champDDID(me.ChampionName), KDA: fmt.Sprintf("%d / %d / %d", me.Scores.Kills, me.Scores.Deaths, me.Scores.Assists), CS: fmt.Sprint(cs), CSColor: colText, Objs: obj}
		c.CSSub = fmt1(csm) + "/min"
		ref := 0.0
		if o.CSGoalValue > 0 {
			ref = o.CSGoalValue
		} else if hasBench {
			ref = bCSM
		}
		if ref > 0 {
			c.CSSub += "  ·  rif. " + fmt1(ref)
			c.CSColor = cmpCol(csm, ref)
		}
		for _, l := range w.Lines {
			if l.Text == "GPM" || l.Text == "KP" || l.Text == "LVL" {
				c.Rows = append(c.Rows, l)
			}
		}
		w.Card, w.Lines, w.Title, w.Width = c, nil, "", 200
		return w
	}
	if o.Compact {
		w.Title, w.Icons = "", false
		for _, l := range w.Lines {
			w.Segs = append(w.Segs, OLine{Text: l.Text, Color: colMuted, Bold: true}, OLine{Text: l.Right, Color: l.Color, Bold: true})
			if l.Extra != "" {
				w.Segs = append(w.Segs, OLine{Text: "/ " + l.Extra, Color: colMuted})
			}
		}
		w.Lines = nil
	}
	return w
}

func mmssShort(sec float64) string {
	if sec < 60 {
		return fmt.Sprintf("%d", int(math.Ceil(sec)))
	}
	return mmss(sec)
}

// "Barracks_T1_L1": T1 = lato blu, T2 = lato rosso; L = top, C = mid, R = bot
func inhibPos(name string) ([2]float64, bool) {
	blue := map[string][2]float64{"L": {1171, 3571}, "C": {3203, 3208}, "R": {3452, 1236}}
	red := map[string][2]float64{"L": {11261, 13676}, "C": {11598, 11667}, "R": {13604, 11316}}
	p := strings.Split(name, "_")
	if len(p) < 3 || len(p[2]) < 1 {
		return [2]float64{}, false
	}
	lane := p[2][:1]
	if p[1] == "T1" {
		v, ok := blue[lane]
		return v, ok
	}
	v, ok := red[lane]
	return v, ok
}

// CS reali: il numero letto dallo schermo, se coerente con l'API (che arrotonda alle decine)
func effectiveCS(me *LivePlayer) int {
	api := me.Scores.CreepScore
	if v, ok := ocrCS(); ok && v >= api && v < api+10 {
		return v
	}
	return api
}
