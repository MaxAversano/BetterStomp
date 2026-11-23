package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------- Riot API ufficiale (con la chiave dell'utente) ----------

type limitWin struct {
	max  int
	span time.Duration
	hits []time.Time
}

type RiotClient struct {
	mu      sync.Mutex
	wins    []*limitWin
	status  string
	calls   int64
	blocked time.Time
}

var riot = &RiotClient{wins: []*limitWin{{18, time.Second, nil}, {95, 2 * time.Minute, nil}}}

func regionalHost(platform string) string {
	switch platform {
	case "na1", "br1", "la1", "la2":
		return "americas"
	case "kr", "jp1":
		return "asia"
	case "oc1", "ph2", "sg2", "th2", "tw2", "vn2":
		return "sea"
	}
	return "europe"
}

func (r *RiotClient) wait() {
	for {
		r.mu.Lock()
		now := time.Now()
		if now.Before(r.blocked) {
			d := r.blocked.Sub(now)
			r.mu.Unlock()
			time.Sleep(d)
			continue
		}
		ok := true
		var sleep time.Duration
		for _, w := range r.wins {
			i := 0
			for i < len(w.hits) && now.Sub(w.hits[i]) > w.span {
				i++
			}
			w.hits = w.hits[i:]
			if len(w.hits) >= w.max {
				ok = false
				if d := w.span - now.Sub(w.hits[0]) + 50*time.Millisecond; d > sleep {
					sleep = d
				}
			}
		}
		if ok {
			for _, w := range r.wins {
				w.hits = append(w.hits, now)
			}
			r.calls++
			r.mu.Unlock()
			return
		}
		r.mu.Unlock()
		time.Sleep(sleep)
	}
}

// adatta i limiti a quelli dichiarati da Riot (chiave dev / personale / produzione)
func (r *RiotClient) adapt(h string) {
	if h == "" {
		return
	}
	var wins []*limitWin
	for _, part := range strings.Split(h, ",") {
		p := strings.Split(part, ":")
		if len(p) != 2 {
			continue
		}
		n, _ := strconv.Atoi(p[0])
		s, _ := strconv.Atoi(p[1])
		if n > 0 && s > 0 {
			m := n * 9 / 10
			if m < 1 {
				m = 1
			}
			wins = append(wins, &limitWin{max: m, span: time.Duration(s) * time.Second})
		}
	}
	if len(wins) == 0 {
		return
	}
	r.mu.Lock()
	// mantengo la cronologia per finestre uguali
	for _, nw := range wins {
		for _, ow := range r.wins {
			if ow.span == nw.span {
				nw.hits = ow.hits
			}
		}
	}
	r.wins = wins
	r.mu.Unlock()
}

type riotErr struct{ code int }

func (e riotErr) Error() string {
	switch e.code {
	case 401, 403:
		return "chiave Riot API non valida o scaduta"
	case 404:
		return "non trovato"
	}
	return fmt.Sprintf("Riot API HTTP %d", e.code)
}

func (r *RiotClient) get(u string, out any) error {
	key := strings.TrimSpace(getSettings().RiotAPIKey)
	if key == "" {
		return fmt.Errorf("serve una chiave Riot API (Impostazioni)")
	}
	for attempt := 0; attempt < 4; attempt++ {
		r.wait()
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("X-Riot-Token", key)
		resp, err := webClient.Do(req)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if attempt == 0 {
			r.adapt(resp.Header.Get("X-App-Rate-Limit"))
		}
		switch {
		case resp.StatusCode == 200:
			r.setStatus("ok")
			return json.Unmarshal(b, out)
		case resp.StatusCode == 429:
			ra, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
			if ra <= 0 {
				ra = 5
			}
			r.mu.Lock()
			r.blocked = time.Now().Add(time.Duration(ra) * time.Second)
			r.mu.Unlock()
			continue
		case resp.StatusCode >= 500:
			time.Sleep(2 * time.Second)
			continue
		case resp.StatusCode == 401 || resp.StatusCode == 403:
			r.setStatus("invalid")
			return riotErr{resp.StatusCode}
		default:
			return riotErr{resp.StatusCode}
		}
	}
	return fmt.Errorf("Riot API non raggiungibile")
}

func (r *RiotClient) setStatus(s string) { r.mu.Lock(); r.status = s; r.mu.Unlock() }
func (r *RiotClient) getStatus() string  { r.mu.Lock(); defer r.mu.Unlock(); return r.status }

func platHost(p string) string { return "https://" + p + ".api.riotgames.com" }
func regHost(p string) string  { return "https://" + regionalHost(p) + ".api.riotgames.com" }

// ---------- DTO match-v5 ----------

type MParticipant struct {
	Puuid       string `json:"puuid"`
	GameName    string `json:"riotIdGameName"`
	Tag         string `json:"riotIdTagline"`
	SummName    string `json:"summonerName"`
	ChampionID  int    `json:"championId"`
	TeamID      int    `json:"teamId"`
	TeamPos     string `json:"teamPosition"`
	Win         bool   `json:"win"`
	Kills       int    `json:"kills"`
	Deaths      int    `json:"deaths"`
	Assists     int    `json:"assists"`
	Minions     int    `json:"totalMinionsKilled"`
	Neutral     int    `json:"neutralMinionsKilled"`
	Gold        int    `json:"goldEarned"`
	Vision      int    `json:"visionScore"`
	Damage      int    `json:"totalDamageDealtToChampions"`
	Item0       int    `json:"item0"`
	Item1       int    `json:"item1"`
	Item2       int    `json:"item2"`
	Item3       int    `json:"item3"`
	Item4       int    `json:"item4"`
	Item5       int    `json:"item5"`
	Item6       int    `json:"item6"`
	Spell1      int    `json:"summoner1Id"`
	Spell2      int    `json:"summoner2Id"`
	Level       int    `json:"champLevel"`
	Placement   int    `json:"placement"`
	ProfileIcon int    `json:"profileIcon"`
	EarlySurr   bool   `json:"gameEndedInEarlySurrender"`
	Perks       struct {
		Stat struct {
			Offense int `json:"offense"`
			Flex    int `json:"flex"`
			Defense int `json:"defense"`
		} `json:"statPerks"`
		Styles []struct {
			Style      int `json:"style"`
			Selections []struct {
				Perk int `json:"perk"`
			} `json:"selections"`
		} `json:"styles"`
	} `json:"perks"`
}

func (p MParticipant) items() []int {
	return []int{p.Item0, p.Item1, p.Item2, p.Item3, p.Item4, p.Item5, p.Item6}
}

type MMatch struct {
	Metadata struct {
		MatchID string `json:"matchId"`
	} `json:"metadata"`
	Info struct {
		GameVersion  string         `json:"gameVersion"`
		QueueID      int            `json:"queueId"`
		GameMode     string         `json:"gameMode"`
		Duration     int            `json:"gameDuration"`
		Start        int64          `json:"gameStartTimestamp"`
		Participants []MParticipant `json:"participants"`
		Teams        []struct {
			TeamID int  `json:"teamId"`
			Win    bool `json:"win"`
			Bans   []struct {
				ChampionID int `json:"championId"`
			} `json:"bans"`
		} `json:"teams"`
	} `json:"info"`
}

type MTimeline struct {
	Info struct {
		Frames []struct {
			Events []struct {
				Type          string `json:"type"`
				ParticipantID int    `json:"participantId"`
				ItemID        int    `json:"itemId"`
				SkillSlot     int    `json:"skillSlot"`
				Timestamp     int64  `json:"timestamp"`
				LevelUpType   string `json:"levelUpType"`
			} `json:"events"`
		} `json:"frames"`
	} `json:"info"`
}

func riotMatchIDs(platform, puuid string, count int, queue int) ([]string, error) {
	u := fmt.Sprintf("%s/lol/match/v5/matches/by-puuid/%s/ids?start=0&count=%d", regHost(platform), puuid, count)
	if queue > 0 {
		u += fmt.Sprintf("&queue=%d", queue)
	}
	var ids []string
	err := riot.get(u, &ids)
	return ids, err
}

func riotMatch(platform, id string) (*MMatch, error) {
	var m MMatch
	err := riot.get(regHost(platform)+"/lol/match/v5/matches/"+id, &m)
	return &m, err
}

func riotTimeline(platform, id string) (*MTimeline, error) {
	var t MTimeline
	err := riot.get(regHost(platform)+"/lol/match/v5/matches/"+id+"/timeline", &t)
	return &t, err
}

type riotAccount struct {
	Puuid    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

func riotAccountByID(platform, name, tag string) (*riotAccount, error) {
	var a riotAccount
	err := riot.get(fmt.Sprintf("%s/riot/account/v1/accounts/by-riot-id/%s/%s", regHost(platform), url.PathEscape(name), url.PathEscape(tag)), &a)
	return &a, err
}
