package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Durate (in secondi) dei timer di gioco. Modificabili dalle impostazioni overlay
// perché Riot le cambia spesso tra una stagione e l'altra.
type Timers struct {
	DragonFirst   float64 `json:"dragonFirst"`
	DragonRespawn float64 `json:"dragonRespawn"`
	ElderRespawn  float64 `json:"elderRespawn"`
	GrubsFirst    float64 `json:"grubsFirst"`
	HeraldFirst   float64 `json:"heraldFirst"`
	AtakhanFirst  float64 `json:"atakhanFirst"`
	BaronFirst    float64 `json:"baronFirst"`
	BaronRespawn  float64 `json:"baronRespawn"`
	CampFirst     float64 `json:"campFirst"`
	BuffFirst     float64 `json:"buffFirst"`
	CampRespawn   float64 `json:"campRespawn"`
	BuffRespawn   float64 `json:"buffRespawn"`
	BaronBuff     float64 `json:"baronBuff"`
	ElderBuff     float64 `json:"elderBuff"`
	RelicFirst    float64 `json:"relicFirst"`
	RelicRespawn  float64 `json:"relicRespawn"`
}

type Widget struct {
	On     bool    `json:"on"`
	X      float64 `json:"x"` // 0..1 frazione dello schermo
	Y      float64 `json:"y"`
	Locked bool    `json:"locked"` // lucchetto: non si sposta tenendo TAB
	Size   float64 `json:"size"`   // dimensione del riquadro (1 = normale), regolabile in partita
}

type OverlaySettings struct {
	Enabled       bool      `json:"enabled"`
	Scale         float64   `json:"scale"`
	CS            Widget    `json:"cs"`
	Objectives    Widget    `json:"objectives"`
	Camps         Widget    `json:"camps"`
	Buffs         Widget    `json:"buffs"`
	Deaths        Widget    `json:"deaths"`
	ItemDiff      Widget    `json:"itemDiff"`
	Loading       Widget    `json:"loading"`
	Minimap       bool      `json:"minimap"`
	MinimapSize   int       `json:"minimapSize"`   // lato della minimappa in pixel
	MinimapRight  int       `json:"minimapRight"`  // distanza dal bordo destro
	MinimapBottom int       `json:"minimapBottom"` // distanza dal bordo inferiore
	Timers        Timers    `json:"timers"`
	Compact       bool      `json:"compact"`     // versione riassunta su una riga
	Classic       bool      `json:"classic"`     // riquadro a righe invece della scheda con ritratto
	ShowKP        bool      `json:"showKP"`      // partecipazione alle uccisioni
	CSGoal        string    `json:"csGoal"`      // elo di riferimento: on (elo) | off
	CSGoalValue   float64   `json:"csGoalValue"` // obiettivo fisso in CS/min (0 = nessuno)
	CSGoalTier    string    `json:"csGoalTier"`  // elo di riferimento se elo
	PillScale     float64   `json:"pillScale"`   // dimensione timer minimappa
	EditMode      bool      `json:"editMode"`    // trascina gli elementi direttamente sullo schermo
	Layout        int       `json:"layout"`      // versione del layout predefinito
	OCR           bool      `json:"ocr"`         // legge i CS dallo schermo (l'API li aggiorna ogni 10)
	BenchAlways   bool      `json:"benchAlways"`
	BenchKDA      bool      `json:"benchKDA"`
	BenchCSM      bool      `json:"benchCSM"`
	BenchGPM      bool      `json:"benchGPM"`
	BenchLVL      bool      `json:"benchLVL"`
	Notify        Widget    `json:"notify"`       // notifiche obiettivi
	NotifySize    string    `json:"notifySize"`   // small | medium | large
	NotifyDur     float64   `json:"notifyDur"`    // secondi a schermo
	NotifyBefore  []float64 `json:"notifyBefore"` // promemoria: secondi prima dello spawn
}

type ProPlayer struct {
	Label  string `json:"label"`  // nome da mostrare (es. "Faker")
	RiotID string `json:"riotId"` // Nome#TAG dell'account
	Region string `json:"region"`
}

type StreamSettings struct {
	GoalTier     string `json:"goalTier"`
	GoalDivision string `json:"goalDivision"`
	ShowGoal     bool   `json:"showGoal"`
	ShowSession  bool   `json:"showSession"`
	ShowWinrate  bool   `json:"showWinrate"`
	Corner       string `json:"corner"`
	Style        string `json:"style"`
	Queue        string `json:"queue"`
}

type Settings struct {
	Theme          string          `json:"theme"`
	Region         string          `json:"region"`
	RiotAPIKey     string          `json:"riotApiKey"`
	ProPlayers     []ProPlayer     `json:"proPlayers"`
	RemoteURL      string          `json:"remoteUrl"`     // server statistiche (es. https://www.aversano.design/bs-data/)
	NativeFrame    bool            `json:"nativeFrame"`   // usa la barra di Windows invece di quella dell'app
	Autostart      bool            `json:"autostart"`     // parte con Windows, nascosto nella barra delle applicazioni
	Notifications  bool            `json:"notifications"` // notifiche di Windows (partita trovata, build importata…)
	NotifSet       bool            `json:"notifSet"`
	RemoteToken    string          `json:"remoteToken"` // codice segreto del server
	EngineEnabled  bool            `json:"engineEnabled"`
	AutoRunes      bool            `json:"autoRunes"`
	AutoSpells     bool            `json:"autoSpells"`
	AutoItems      bool            `json:"autoItems"`
	PreferPersonal bool            `json:"preferPersonal"`
	FlashKey       string          `json:"flashKey"`
	DefaultMode    string          `json:"defaultMode"`
	DefaultTier    string          `json:"defaultTier"`
	LeaguePath     string          `json:"leaguePath"`
	Premium        bool            `json:"premium"` // non più usato: tutto è gratuito
	Animations     bool            `json:"animations"`
	OpggOff        bool            `json:"opggOff"` // non usare i dati OP.GG (solo motore/server)
	Overlay        OverlaySettings `json:"overlay"`
	Stream         StreamSettings  `json:"stream"`
}

func defaultSettings() Settings {
	return Settings{
		Theme: "dark", Region: "euw1", AutoRunes: true, AutoSpells: true, AutoItems: true,
		FlashKey: "F", DefaultMode: "ranked", DefaultTier: "auto", EngineEnabled: true, Animations: true,
		Overlay: OverlaySettings{
			Enabled: true, Scale: 1,
			CS:         Widget{On: true, X: 0.9165, Y: 0.125},
			Objectives: Widget{On: true, X: 0.9165, Y: 0.27},
			ShowKP:     true, CSGoal: "elo", CSGoalValue: 0, CSGoalTier: "MASTER", PillScale: 0.8,
			OCR: true, BenchAlways: true, BenchKDA: true, BenchCSM: true, BenchGPM: true, BenchLVL: true,
			Notify: Widget{On: true, X: 0.5, Y: 0.36}, NotifySize: "medium", NotifyDur: 10, NotifyBefore: []float64{60},
			Camps:    Widget{On: false, X: 0.9165, Y: 0.40},
			Buffs:    Widget{On: true, X: 0.5, Y: 0.12},
			Deaths:   Widget{On: true, X: 0.012, Y: 0.40},
			ItemDiff: Widget{On: true, X: 0.5, Y: 0.19},
			Loading:  Widget{On: true, X: 0.5, Y: 0.28},
			Minimap:  true, MinimapSize: 226, MinimapRight: 7, MinimapBottom: 13,
			Timers: Timers{
				DragonFirst: 300, DragonRespawn: 300, ElderRespawn: 360,
				GrubsFirst: 480, HeraldFirst: 900, AtakhanFirst: 1200,
				BaronFirst: 1500, BaronRespawn: 360,
				CampFirst: 90, BuffFirst: 90, CampRespawn: 135, BuffRespawn: 300,
				BaronBuff: 180, ElderBuff: 150, RelicFirst: 60, RelicRespawn: 40,
			},
		},
		Stream: StreamSettings{GoalTier: "DIAMOND", GoalDivision: "IV", ShowGoal: true, ShowSession: true,
			ShowWinrate: true, Corner: "top-left", Style: "glass", Queue: "RANKED_SOLO_5x5"},
	}
}

var (
	dataDir    string
	settingsMu sync.RWMutex
	settings   Settings
)

func initDataDir() {
	exe, err := os.Executable()
	dir := "."
	if err == nil {
		dir = filepath.Dir(exe)
	}
	dataDir = filepath.Join(dir, "BetterStomp-data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		// cartella non scrivibile (es. Programmi): ripiego su AppData
		if la := os.Getenv("LOCALAPPDATA"); la != "" {
			dataDir = filepath.Join(la, "BetterStomp")
		} else {
			dataDir = filepath.Join(os.TempDir(), "BetterStomp")
		}
		os.MkdirAll(dataDir, 0o755)
	}
}

func loadSettings() {
	s := defaultSettings()
	if b, err := os.ReadFile(filepath.Join(dataDir, "settings.json")); err == nil {
		json.Unmarshal(b, &s)
	}
	if s.Overlay.Scale <= 0 {
		s.Overlay.Scale = 1
	}
	if !s.NotifSet { // notifiche di Windows attive di base
		s.Notifications, s.NotifSet = true, true
	}
	if s.Overlay.PillScale <= 0 {
		s.Overlay.PillScale = 0.7
	}
	if s.Overlay.CSGoal == "" {
		s.Overlay.CSGoal, s.Overlay.CSGoalValue, s.Overlay.CSGoalTier, s.Overlay.ShowKP = "elo", 7.5, "MASTER", true
	}
	s.Overlay.EditMode = false
	if s.Overlay.Layout == 2 {
		// versione 3: nuove opzioni e minimappa ricalibrata (valori riferiti a 1080p)
		d := defaultSettings().Overlay
		o := &s.Overlay
		o.OCR, o.BenchAlways, o.BenchKDA, o.BenchCSM, o.BenchGPM, o.BenchLVL = true, true, true, true, true, true
		o.Notify, o.NotifySize, o.NotifyDur, o.NotifyBefore = d.Notify, d.NotifySize, d.NotifyDur, d.NotifyBefore
		o.MinimapSize, o.MinimapRight, o.MinimapBottom = d.MinimapSize, d.MinimapRight, d.MinimapBottom
		o.Layout = 3
	}
	if s.Overlay.Layout < 2 {
		// nuovo overlay in stile Blitz: riparto dalle posizioni predefinite
		d := defaultSettings().Overlay
		d.Enabled, d.Timers, d.MinimapSize, d.MinimapRight, d.MinimapBottom, d.Minimap = s.Overlay.Enabled, s.Overlay.Timers, s.Overlay.MinimapSize, s.Overlay.MinimapRight, s.Overlay.MinimapBottom, s.Overlay.Minimap
		d.Layout = 3
		s.Overlay = d
	}
	settingsMu.Lock()
	settings = s
	settingsMu.Unlock()
}

func getSettings() Settings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return settings
}

func saveSettings(s Settings) error {
	settingsMu.Lock()
	settings = s
	settingsMu.Unlock()
	return writeJSON(filepath.Join(dataDir, "settings.json"), s)
}

func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}
