package main

// Champion select: informazioni sui compagni (rango, win rate, campione, serie, etichette)
// lette dal client di League. I nemici in classificata sono anonimi fino al caricamento.

import (
	"fmt"
	"sync"
	"time"
)

type CSMate struct {
	CellID    int      `json:"cellId"`
	Name      string   `json:"name"`
	Tier      string   `json:"tier"`
	Division  string   `json:"division"`
	LP        int      `json:"lp"`
	Wins      int      `json:"wins"`
	Losses    int      `json:"losses"`
	Level     int      `json:"level"`
	Recent    []bool   `json:"recent"`     // ultime partite, dalla più recente (true = vittoria)
	ChampG    int      `json:"champGames"` // partite recenti con il campione scelto
	ChampW    int      `json:"champWins"`
	MainChamp int      `json:"mainChamp"` // campione più giocato di recente
	MainShare float64  `json:"mainShare"`
	Tags      []string `json:"tags"`
}

type csCacheEntry struct {
	at     time.Time
	rank   RankedQueue
	level  int
	champs []int // campioni delle ultime partite
	wins   []bool
	games  []csGame // statistiche delle stesse partite (scheda giocatore)
	sumID  int64
	name   string
	tag    string
}

var (
	csMu    sync.Mutex
	csCache = map[string]*csCacheEntry{}
)

func csPlayerData(puuid string) *csCacheEntry {
	csMu.Lock()
	if c, ok := csCache[puuid]; ok && time.Since(c.at) < 10*time.Minute {
		csMu.Unlock()
		return c
	}
	csMu.Unlock()
	c := &csCacheEntry{at: time.Now()}
	c.rank = soloRank(puuid)
	var sm struct {
		Level int    `json:"summonerLevel"`
		ID    int64  `json:"summonerId"`
		Name  string `json:"gameName"`
		Tag   string `json:"tagLine"`
	}
	lcu.getJSON("/lol-summoner/v2/summoners/puuid/"+puuid, &sm)
	c.level, c.sumID, c.name, c.tag = sm.Level, sm.ID, sm.Name, sm.Tag
	var hist struct {
		Games struct {
			Games []lcuGame `json:"games"`
		} `json:"games"`
	}
	lcu.getJSON(fmt.Sprintf("/lol-match-history/v1/products/lol/%s/matches?begIndex=0&endIndex=20", puuid), &hist)
	games := hist.Games.Games
	// dalla più recente
	for i := 0; i < len(games); i++ {
		for j := i + 1; j < len(games); j++ {
			if games[j].GameCreation > games[i].GameCreation {
				games[i], games[j] = games[j], games[i]
			}
		}
	}
	for _, g := range games {
		if len(g.Participants) == 0 || g.GameDuration > 0 && g.GameDuration < 300 && g.GameDuration < 100000 {
			continue
		}
		p := g.Participants[0]
		c.champs = append(c.champs, p.ChampionID)
		c.wins = append(c.wins, p.Stats.Win)
		c.games = append(c.games, csGameFrom(g))
	}
	csMu.Lock()
	csCache[puuid] = c
	csMu.Unlock()
	return c
}

// compagni della champion select con le etichette utili (OTP, serie, primo game col campione…)
func champSelectMates() []CSMate {
	cs := getClientState().ChampSel
	if cs == nil {
		return nil
	}
	var out []CSMate
	var wg sync.WaitGroup
	res := make([]CSMate, len(cs.MyTeam))
	for i, p := range cs.MyTeam {
		wg.Add(1)
		go func(i int, p CSPlayer) {
			defer wg.Done()
			m := CSMate{CellID: p.CellID, Name: p.GameName}
			if p.Puuid == "" {
				res[i] = m
				return
			}
			d := csPlayerData(p.Puuid)
			m.Tier, m.Division, m.LP, m.Wins, m.Losses, m.Level = d.rank.Tier, d.rank.Division, d.rank.LeaguePoints, d.rank.Wins, d.rank.Losses, d.level
			m.Recent = d.wins
			champ := p.ChampionID
			if champ == 0 {
				champ = p.Intent
			}
			count := map[int]int{}
			for j, c := range d.champs {
				count[c]++
				if c == champ {
					m.ChampG++
					if d.wins[j] {
						m.ChampW++
					}
				}
			}
			for c, n := range count {
				if n > count[m.MainChamp] {
					m.MainChamp = c
				}
			}
			if len(d.champs) > 0 {
				m.MainShare = float64(count[m.MainChamp]) / float64(len(d.champs))
			}
			// etichette
			if len(d.champs) >= 8 && m.MainShare >= 0.6 {
				m.Tags = append(m.Tags, "OTP "+champName(m.MainChamp))
			}
			streak := 0
			for j, w := range d.wins {
				if j == 0 || w == d.wins[0] {
					streak++
				} else {
					break
				}
			}
			if streak >= 3 && len(d.wins) > 0 {
				if d.wins[0] {
					m.Tags = append(m.Tags, fmt.Sprintf("%d vittorie di fila", streak))
				} else {
					m.Tags = append(m.Tags, fmt.Sprintf("%d sconfitte di fila", streak))
				}
			}
			if champ > 0 && m.ChampG == 0 && len(d.champs) >= 10 {
				m.Tags = append(m.Tags, "Mai giocato di recente")
			}
			if g := m.Wins + m.Losses; m.Losses > 0 && g >= 20 && m.Level > 0 && m.Level < 80 && float64(m.Wins)/float64(g) >= 0.62 {
				m.Tags = append(m.Tags, "Possibile smurf")
			}
			res[i] = m
		}(i, p)
	}
	wg.Wait()
	out = append(out, res...)
	return out
}
