package main

// Storico dei LP: ogni volta che il rango cambia ne salvo una "foto", per il grafico
// dell'andamento e per i LP guadagnati/persi a fine partita.

import (
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type LPPoint struct {
	T        int64  `json:"t"`
	Queue    string `json:"queue"`
	Tier     string `json:"tier"`
	Division string `json:"division"`
	LP       int    `json:"lp"`
	Wins     int    `json:"wins"`
	Losses   int    `json:"losses"`
	Value    int    `json:"value"` // posizione assoluta: 400 per lega, 100 per divisione
}

var lpWatch atomic.Int64 // fine dell'ultima partita (ms)

var (
	lpMu   sync.Mutex
	lpHist map[string][]LPPoint // puuid → punti
)

func lpPath() string { return filepath.Join(dataDir, "lp_storico.json") }

func lpValue(tier, div string, lp int) int {
	ti := -1
	for i, t := range Tiers {
		if t == tier {
			ti = i
		}
	}
	if ti < 0 {
		return -1
	}
	if ti >= 7 { // Master e oltre: LP senza divisioni
		return 7*400 + lp
	}
	d := map[string]int{"IV": 0, "III": 1, "II": 2, "I": 3}[div]
	return ti*400 + d*100 + lp
}

func recordLP(puuid string, rk map[string]RankedQueue) {
	if puuid == "" {
		return
	}
	lpMu.Lock()
	defer lpMu.Unlock()
	if lpHist == nil {
		lpHist = map[string][]LPPoint{}
		readJSON(lpPath(), &lpHist)
	}
	changed := false
	for _, q := range []string{"RANKED_SOLO_5x5", "RANKED_FLEX_SR"} {
		r, ok := rk[q]
		if !ok || r.Tier == "" || r.Tier == "NONE" {
			continue
		}
		v := lpValue(r.Tier, r.Division, r.LeaguePoints)
		var last *LPPoint
		for i := len(lpHist[puuid]) - 1; i >= 0; i-- {
			if lpHist[puuid][i].Queue == q {
				last = &lpHist[puuid][i]
				break
			}
		}
		if last != nil && last.Value == v && last.Wins == r.Wins && last.Losses == r.Losses {
			continue
		}
		lpHist[puuid] = append(lpHist[puuid], LPPoint{time.Now().UnixMilli(), q, r.Tier, r.Division, r.LeaguePoints, r.Wins, r.Losses, v})
		changed = true
		// subito dopo una partita: notifica con i LP guadagnati o persi
		if last != nil && time.Now().UnixMilli()-lpWatch.Load() < 10*60*1000 {
			d := v - last.Value
			name := map[string]string{"RANKED_SOLO_5x5": "Solo/Duo", "RANKED_FLEX_SR": "Flex"}[q]
			sign := ""
			if d >= 0 {
				sign = "+"
			}
			notify(fmt.Sprintf("%s%d LP · %s", sign, d, name), fmt.Sprintf("Ora sei %s %s con %d LP.", tierIT[r.Tier], r.Division, r.LeaguePoints))
		}
	}
	if n := len(lpHist[puuid]); n > 2000 {
		lpHist[puuid] = lpHist[puuid][n-2000:]
	}
	if changed {
		writeJSON(lpPath(), lpHist)
	}
}

func lpHistory(puuid string) []LPPoint {
	lpMu.Lock()
	defer lpMu.Unlock()
	if lpHist == nil {
		lpHist = map[string][]LPPoint{}
		readJSON(lpPath(), &lpHist)
	}
	return append([]LPPoint(nil), lpHist[puuid]...)
}
