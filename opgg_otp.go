package main

// Build degli OTP: i migliori giocatori del campione nella classifica OP.GG della tua regione
// (spesso pro o streamer) e le loro ultime partite classificate con quel campione.

import (
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var opggRegion = map[string]string{"euw1": "EUW", "eun1": "EUNE", "na1": "NA", "kr": "KR", "br1": "BR", "la1": "LAN", "la2": "LAS",
	"oc1": "OCE", "tr1": "TR", "ru": "RU", "jp1": "JP", "me1": "ME", "sg2": "SG", "tw2": "TW", "vn2": "VN"}

type otpCache struct {
	G  []ProGame `json:"g"`
	At time.Time `json:"at"`
}

var (
	otpMu      sync.Mutex
	otpCache_  = map[string]otpCache{}
	otpLoaded  bool
	otpRunning sync.Map
)

// partite OTP: quelle salvate subito, aggiornate in sottofondo se hanno più di 2 ore
func opggOTPCached(champ int) ([]ProGame, bool) {
	region := opggRegion[strings.ToLower(getSettings().Region)]
	if region == "" {
		region = "EUW"
	}
	ck := fmt.Sprintf("%d|%s", champ, region)
	otpMu.Lock()
	if !otpLoaded {
		otpLoaded = true
		readJSON(filepath.Join(opggDir(), "otp.json"), &otpCache_)
	}
	c, ok := otpCache_[ck]
	otpMu.Unlock()
	if ok && time.Since(c.At) < 2*time.Hour {
		return c.G, true
	}
	if _, busy := otpRunning.LoadOrStore(ck, true); !busy {
		go func() {
			defer otpRunning.Delete(ck)
			opggOTPGames(champ)
		}()
	}
	return c.G, ok
}

func opggOTPGames(champ int) []ProGame {
	if err := opgg.ensureChamps(); err != nil {
		return nil
	}
	region := opggRegion[strings.ToLower(getSettings().Region)]
	if region == "" {
		region = "EUW"
	}
	ck := fmt.Sprintf("%d|%s", champ, region)
	opgg.mu.Lock()
	key := opgg.keys[champ]
	opgg.mu.Unlock()
	if key == "" {
		return nil
	}
	lb, err := opgg.call("lol_list_champion_leaderboard", map[string]any{"region": region, "champion": key, "desired_output_fields": []string{
		"leaderboard[].rank", "leaderboard[].summoner.{game_name,tagline}", "leaderboard[].summoner.player.{nickname}",
		"leaderboard[].most_champion_stat.{play,win}", "leaderboard[].summoner.league_stats[].tier_info.{tier,lp}"}})
	if err != nil {
		return nil
	}
	list, _ := lb.([]any)
	if list == nil {
		list = ogl(lb, "leaderboard")
	}
	if len(list) > 8 {
		list = list[:8]
	}
	var out []ProGame
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, x := range list {
		su := og(x, "summoner")
		name, tag := ogs(su, "game_name"), ogs(su, "tagline")
		if name == "" || tag == "" {
			continue
		}
		nick := ogs(su, "player", "nickname")
		tier, lp := "", 0
		if ls := ogl(su, "league_stats"); len(ls) > 0 {
			tier, lp = ogs(ls[0], "tier_info", "tier"), ogi(ls[0], "tier_info", "lp")
		}
		play, win := ogi(x, "most_champion_stat", "play"), ogi(x, "most_champion_stat", "win")
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := opgg.call("lol_list_summoner_matches", map[string]any{"game_name": name, "tag_line": tag, "region": region, "limit": 20,
				"desired_output_fields": []string{"data.game_history[].{created_at,game_type,id}",
					"data.game_history[].participants[].{champion_id,items[],spells[],position}",
					"data.game_history[].participants[].rune.{primary_page_id,primary_rune_id,secondary_page_id}",
					"data.game_history[].participants[].stats.{kill,death,assist,result,minion_kill,neutral_minion_kill}"}})
			if err != nil {
				return
			}
			n := 0
			for _, g := range ogl(v, "data", "game_history") {
				gt := ogs(g, "game_type")
				if gt != "SOLORANKED" && gt != "FLEXRANKED" {
					continue
				}
				ps := ogl(g, "participants")
				if len(ps) == 0 || ogi(ps[0], "champion_id") != champ {
					continue
				}
				p := ps[0]
				pg := ProGame{Name: name, Tag: tag, Tier: tier, LP: lp, Pro: nick != "", Region: region, Champ: champ,
					Pos: opgg2pos[strings.ToLower(ogs(p, "position"))], Win: ogs(p, "stats", "result") == "WIN", Spells: ogids(p, "spells"),
					Mode: "ranked", Match: ogs(g, "id"), Keystone: ogi(p, "rune", "primary_rune_id"),
					K: ogi(p, "stats", "kill"), D: ogi(p, "stats", "death"), A: ogi(p, "stats", "assist"),
					CS: ogi(p, "stats", "minion_kill") + ogi(p, "stats", "neutral_minion_kill"), Otp: true, OtpGames: play}
				if nick != "" {
					pg.Name = nick
				}
				if play > 0 {
					pg.OtpWR = float64(win) / float64(play)
				}
				if t, err := time.Parse(time.RFC3339, ogs(g, "created_at")); err == nil {
					pg.When = t.UnixMilli()
				}
				for _, it := range ogids(p, "items") {
					if it > 0 && it != 3340 && it != 3363 && it != 3364 {
						pg.Items = append(pg.Items, it)
					}
				}
				// pagina di rune: dalla build dello stesso keystone e degli stessi alberi
				pg.Runes = strconv.Itoa(ogi(p, "rune", "primary_page_id")) + ":" + strconv.Itoa(ogi(p, "rune", "secondary_page_id")) + ":"
				mu.Lock()
				out = append(out, pg)
				mu.Unlock()
				if n++; n >= 4 {
					break
				}
			}
		}()
	}
	wg.Wait()
	sort.Slice(out, func(i, j int) bool { return out[i].When > out[j].When })
	if len(out) == 0 && len(list) == 0 {
		return out
	}
	otpMu.Lock()
	otpCache_[ck] = otpCache{out, time.Now()}
	cp := make(map[string]otpCache, len(otpCache_))
	for k, v := range otpCache_ {
		if time.Since(v.At) < 7*24*time.Hour {
			cp[k] = v
		}
	}
	otpMu.Unlock()
	writeJSON(filepath.Join(opggDir(), "otp.json"), cp)
	return out
}

// completa le rune degli OTP con una pagina intera dello stesso keystone e degli stessi alberi
func fillOTPRunes(games []ProGame, pages []*RunePage) []ProGame {
	out := make([]ProGame, len(games))
	for i, g := range games {
		p := strings.Split(g.Runes, ":")
		g.Runes = ""
		var best *RunePage
		for _, rp := range pages {
			if rp == nil || len(rp.Perks) != 9 || rp.Perks[0] != g.Keystone {
				continue
			}
			if best == nil || (len(p) >= 2 && strconv.Itoa(rp.Secondary) == p[1] && strconv.Itoa(best.Secondary) != p[1]) {
				best = rp
			}
		}
		if best != nil {
			var s []string
			for _, x := range best.Perks {
				s = append(s, strconv.Itoa(x))
			}
			g.Runes = fmt.Sprintf("%d:%d:%s", best.Primary, best.Secondary, strings.Join(s, ","))
		}
		out[i] = g
	}
	return out
}
