package main

// Dati da OP.GG tramite la loro API MCP ufficiale e gratuita (https://mcp-api.op.gg/mcp):
// tier list per elo, build, rune, incantesimi, abilità e counter.
// Se OP.GG non risponde si torna ai dati del nostro server/motore.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const opggURL = "https://mcp-api.op.gg/mcp"

type opggClient struct {
	hc      *http.Client
	mu      sync.Mutex
	ids     map[string]int // nome/chiave in minuscolo, senza spazi -> id campione
	keys    map[int]string // id -> chiave OP.GG (es. MONKEYKING)
	meta    []opggMetaRow
	metaAt  time.Time
	builds  map[string]opggBuildCache
	jobs    map[string]*opggTierJob
	failAt  time.Time
	loading sync.Mutex
}

type opggBuildCache struct {
	B  BuildResult
	At time.Time
}

type opggMetaRow struct {
	Champion int     `json:"c"`
	Pos      string  `json:"p"` // TOP JUNGLE MIDDLE BOTTOM UTILITY
	Play     int     `json:"n"`
	WinRate  float64 `json:"w"`
	PickRate float64 `json:"pr"`
	BanRate  float64 `json:"br"`
	RoleRate float64 `json:"rr"`
	Tier     int     `json:"t"`
	Rank     int     `json:"r"`
}

type opggTierJob struct {
	Tier  string        `json:"tier"`
	At    time.Time     `json:"at"`
	Rows  []opggMetaRow `json:"rows"`
	Done  int           `json:"-"`
	Total int           `json:"-"`
	Busy  bool          `json:"-"`
}

var opgg = &opggClient{
	hc:     &http.Client{Timeout: 25 * time.Second},
	builds: map[string]opggBuildCache{},
	jobs:   map[string]*opggTierJob{},
}

func opggDir() string {
	d := filepath.Join(dataDir, "opgg")
	os.MkdirAll(d, 0o755)
	return d
}

// ---------- chiamata JSON-RPC ----------

func (o *opggClient) call(tool string, args map[string]any) (any, error) {
	args["client_app"] = "BetterStomp/1.0"
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args}})
	req, _ := http.NewRequest("POST", opggURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("User-Agent", "BetterStomp/1.0")
	resp, err := o.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	raw := buf.Bytes()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("op.gg: HTTP %d", resp.StatusCode)
	}
	// a volte la risposta arriva come stream SSE ("data: {...}")
	if t := bytes.TrimSpace(raw); len(t) > 0 && t[0] != '{' {
		for _, ln := range strings.Split(string(t), "\n") {
			if strings.HasPrefix(ln, "data:") {
				raw = []byte(strings.TrimSpace(strings.TrimPrefix(ln, "data:")))
			}
		}
	}
	var r struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			IsError bool `json:"isError"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	if r.Error != nil {
		return nil, errors.New("op.gg: " + r.Error.Message)
	}
	if len(r.Result.Content) == 0 {
		return nil, errors.New("op.gg: risposta vuota")
	}
	if r.Result.IsError {
		return nil, errors.New("op.gg: " + r.Result.Content[0].Text)
	}
	return parseOpgg(r.Result.Content[0].Text)
}

// ---------- parser del formato "class X: a,b\n...\nX(1,[...],\"s\")" ----------

func parseOpgg(s string) (any, error) {
	classes := map[string][]string{}
	lines := strings.Split(s, "\n")
	expr := ""
	for i, ln := range lines {
		if strings.HasPrefix(ln, "class ") {
			p := strings.SplitN(strings.TrimPrefix(ln, "class "), ":", 2)
			if len(p) == 2 {
				var f []string
				for _, x := range strings.Split(p[1], ",") {
					f = append(f, strings.TrimSpace(x))
				}
				classes[strings.TrimSpace(p[0])] = f
			}
			continue
		}
		if strings.TrimSpace(ln) != "" {
			expr = strings.Join(lines[i:], "\n")
			break
		}
	}
	p := &opParser{s: expr, cls: classes}
	v, err := p.value()
	if err != nil {
		return nil, err
	}
	return v, nil
}

type opParser struct {
	s   string
	i   int
	cls map[string][]string
}

func (p *opParser) ws() {
	for p.i < len(p.s) && (p.s[p.i] == ' ' || p.s[p.i] == '\n' || p.s[p.i] == '\r' || p.s[p.i] == '\t') {
		p.i++
	}
}

func (p *opParser) value() (any, error) {
	p.ws()
	if p.i >= len(p.s) {
		return nil, errors.New("op.gg: fine inattesa")
	}
	c := p.s[p.i]
	switch {
	case c == '[':
		p.i++
		var out []any
		for {
			p.ws()
			if p.i < len(p.s) && p.s[p.i] == ']' {
				p.i++
				return out, nil
			}
			v, err := p.value()
			if err != nil {
				return nil, err
			}
			out = append(out, v)
			p.ws()
			if p.i < len(p.s) && p.s[p.i] == ',' {
				p.i++
			}
		}
	case c == '"' || c == '\'':
		q := c
		j := p.i + 1
		for j < len(p.s) && p.s[j] != q {
			if p.s[j] == '\\' {
				j++
			}
			j++
		}
		if j >= len(p.s) {
			return nil, errors.New("op.gg: stringa non chiusa")
		}
		raw := p.s[p.i : j+1]
		p.i = j + 1
		if q == '"' {
			if u, err := strconv.Unquote(raw); err == nil {
				return u, nil
			}
		}
		return strings.ReplaceAll(raw[1:len(raw)-1], "\\", ""), nil
	case c == '-' || (c >= '0' && c <= '9'):
		j := p.i + 1
		for j < len(p.s) && strings.IndexByte("0123456789.eE+-", p.s[j]) >= 0 {
			j++
		}
		f, err := strconv.ParseFloat(p.s[p.i:j], 64)
		p.i = j
		return f, err
	default:
		j := p.i
		for j < len(p.s) && (p.s[j] == '_' || (p.s[j] >= 'a' && p.s[j] <= 'z') || (p.s[j] >= 'A' && p.s[j] <= 'Z') || (p.s[j] >= '0' && p.s[j] <= '9')) {
			j++
		}
		name := p.s[p.i:j]
		if name == "" {
			return nil, fmt.Errorf("op.gg: carattere inatteso %q", c)
		}
		p.i = j
		switch name {
		case "true", "True":
			return true, nil
		case "false", "False":
			return false, nil
		case "null", "None", "nil":
			return nil, nil
		}
		p.ws()
		if p.i >= len(p.s) || p.s[p.i] != '(' {
			return name, nil
		}
		p.i++
		var args []any
		for {
			p.ws()
			if p.i < len(p.s) && p.s[p.i] == ')' {
				p.i++
				break
			}
			v, err := p.value()
			if err != nil {
				return nil, err
			}
			args = append(args, v)
			p.ws()
			if p.i < len(p.s) && p.s[p.i] == ',' {
				p.i++
			}
		}
		f, ok := p.cls[name]
		if !ok {
			return args, nil
		}
		m := map[string]any{}
		for k, v := range args {
			if k < len(f) {
				m[f[k]] = v
			}
		}
		return m, nil
	}
}

// ---------- accesso ai valori ----------

func og(v any, path ...string) any {
	for _, k := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}
func ogl(v any, path ...string) []any { l, _ := og(v, path...).([]any); return l }
func ogf(v any, path ...string) float64 {
	f, _ := og(v, path...).(float64)
	return f
}
func ogi(v any, path ...string) int    { return int(math.Round(ogf(v, path...))) }
func ogs(v any, path ...string) string { s, _ := og(v, path...).(string); return s }
func ogids(v any, path ...string) []int {
	var out []int
	for _, x := range ogl(v, path...) {
		if f, ok := x.(float64); ok {
			out = append(out, int(f))
		}
	}
	return out
}
func ogstr(v any, path ...string) []string {
	var out []string
	for _, x := range ogl(v, path...) {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// ---------- posizioni, elo, campioni ----------

var opgg2pos = map[string]string{"top": "TOP", "jungle": "JUNGLE", "mid": "MIDDLE", "adc": "BOTTOM", "support": "UTILITY"}
var pos2opgg = map[string]string{"TOP": "top", "JUNGLE": "jungle", "MIDDLE": "mid", "MID": "mid", "BOTTOM": "adc", "ADC": "adc", "UTILITY": "support", "SUPPORT": "support"}

// elo di BetterStomp -> filtro OP.GG ("" = tutti i ranghi)
func opggTier(tf string) string {
	t := strings.ToUpper(strings.TrimSpace(tf))
	plus := strings.HasSuffix(t, "+")
	base := strings.TrimSuffix(t, "+")
	switch base {
	case "", "ALL", "AUTO", "IRON", "BRONZE", "SILVER":
		if base == "" || base == "ALL" || base == "AUTO" || plus {
			return ""
		}
	}
	if plus {
		switch base {
		case "GOLD", "PLATINUM", "EMERALD", "DIAMOND", "MASTER":
			return strings.ToLower(base) + "_plus"
		case "GRANDMASTER":
			return "grandmaster"
		}
	}
	return strings.ToLower(base)
}

func normName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (o *opggClient) ensureChamps() error {
	o.mu.Lock()
	ok := len(o.keys) > 0
	o.mu.Unlock()
	if ok {
		return nil
	}
	type ch struct {
		ID   int    `json:"id"`
		Key  string `json:"key"`
		Name string `json:"name"`
	}
	var list []ch
	path := filepath.Join(opggDir(), "campioni.json")
	if st, err := os.Stat(path); err == nil && time.Since(st.ModTime()) < 7*24*time.Hour {
		readJSON(path, &list)
	}
	if len(list) < 100 {
		v, err := o.call("lol_list_champions", map[string]any{"lang": "en_US",
			"desired_output_fields": []string{"data.champions[].{champion_id,key,name}"}})
		if err != nil {
			return err
		}
		list = nil
		for _, c := range ogl(v, "data", "champions") {
			list = append(list, ch{ogi(c, "champion_id"), ogs(c, "key"), ogs(c, "name")})
		}
		if len(list) < 100 {
			return errors.New("op.gg: lista campioni incompleta")
		}
		writeJSON(path, list)
	}
	o.mu.Lock()
	o.ids, o.keys = map[string]int{}, map[int]string{}
	for _, c := range list {
		// OP.GG elenca anche copie speciali dei campioni (id 60000+, modalità a tempo): vanno ignorate
		if c.ID <= 0 || c.ID >= 10000 || !knownChamp(c.ID) {
			continue
		}
		o.keys[c.ID] = strings.ToUpper(normName(c.Key))
		o.ids[normName(c.Key)] = c.ID
		o.ids[normName(c.Name)] = c.ID
	}
	o.mu.Unlock()
	return nil
}

func knownChamp(id int) bool {
	static.mu.RLock()
	defer static.mu.RUnlock()
	if len(static.Champs) == 0 {
		return id < 10000
	}
	_, ok := static.Champs[id]
	return ok
}

func (o *opggClient) champID(name string) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	if id, ok := o.ids[normName(name)]; ok {
		return id
	}
	return champByName(name)
}

// ---------- meta delle corsie (tutti i ranghi) ----------

func (o *opggClient) laneMeta() ([]opggMetaRow, error) {
	o.mu.Lock()
	if len(o.meta) > 0 && time.Since(o.metaAt) < 3*time.Hour {
		m := o.meta
		o.mu.Unlock()
		return m, nil
	}
	if time.Since(o.failAt) < 2*time.Minute {
		m := o.meta
		o.mu.Unlock()
		if len(m) > 0 {
			return m, nil
		}
		return nil, errors.New("op.gg non raggiungibile")
	}
	o.mu.Unlock()
	o.loading.Lock()
	defer o.loading.Unlock()
	o.mu.Lock()
	if len(o.meta) > 0 && time.Since(o.metaAt) < 3*time.Hour {
		m := o.meta
		o.mu.Unlock()
		return m, nil
	}
	o.mu.Unlock()
	path := filepath.Join(opggDir(), "meta_v2.json")
	var disk struct {
		At   time.Time     `json:"at"`
		Rows []opggMetaRow `json:"rows"`
	}
	readJSON(path, &disk)
	if len(disk.Rows) > 0 && time.Since(disk.At) < 3*time.Hour {
		o.mu.Lock()
		o.meta, o.metaAt = disk.Rows, disk.At
		o.mu.Unlock()
		return disk.Rows, nil
	}
	rows, err := o.fetchMeta()
	if err != nil {
		log.Printf("op.gg meta: %v", err)
		o.mu.Lock()
		o.failAt = time.Now()
		o.mu.Unlock()
		if len(disk.Rows) > 0 { // meglio dati vecchi di qualche ora che niente
			return disk.Rows, nil
		}
		return nil, err
	}
	o.mu.Lock()
	o.meta, o.metaAt = rows, time.Now()
	o.mu.Unlock()
	writeJSON(path, map[string]any{"at": time.Now(), "rows": rows})
	return rows, nil
}

func (o *opggClient) fetchMeta() ([]opggMetaRow, error) {
	if err := o.ensureChamps(); err != nil {
		return nil, err
	}
	var f []string
	for k := range opgg2pos {
		f = append(f, "data.positions."+k+"[].{champion,play,win_rate,pick_rate,role_rate,ban_rate,tier,rank}")
	}
	v, err := o.call("lol_list_lane_meta_champions", map[string]any{"position": "all", "desired_output_fields": f})
	if err != nil {
		return nil, err
	}
	var rows []opggMetaRow
	for k, pos := range opgg2pos {
		for _, x := range ogl(v, "data", "positions", k) {
			id := o.champID(ogs(x, "champion"))
			if id == 0 {
				continue
			}
			rows = append(rows, opggMetaRow{Champion: id, Pos: pos, Play: ogi(x, "play"), WinRate: ogf(x, "win_rate"),
				PickRate: ogf(x, "pick_rate"), BanRate: ogf(x, "ban_rate"), RoleRate: ogf(x, "role_rate"),
				Tier: ogi(x, "tier"), Rank: ogi(x, "rank")})
		}
	}
	if len(rows) < 50 {
		return nil, errors.New("op.gg: meta incompleta")
	}
	return rows, nil
}

// ruolo principale di un campione secondo OP.GG
func (o *opggClient) mainPos(champ int) string {
	meta, _ := o.laneMeta()
	best, bn := "", -1
	for _, r := range meta {
		if r.Champion == champ && r.Play > bn {
			best, bn = r.Pos, r.Play
		}
	}
	return best
}

// ---------- tier list ----------

func opggTierLabel(t, rank int) string {
	switch {
	case t <= 1 && rank > 0 && rank <= 3:
		return "S+"
	case t <= 1:
		return "S"
	case t == 2:
		return "A"
	case t == 3:
		return "B"
	case t == 4:
		return "C"
	}
	return "D"
}

func metaToRows(meta []opggMetaRow, pos string) ([]TierRow, int) {
	var rows []TierRow
	tot := 0
	for _, r := range meta {
		if (pos != "" && pos != "ALL" && r.Pos != pos) || !knownChamp(r.Champion) {
			continue
		}
		tot += r.Play
		if r.RoleRate < 0.08 && r.Play < 3000 { // ruoli improvvisati
			continue
		}
		t := r.Tier
		if t <= 0 {
			t = 5
		}
		rows = append(rows, TierRow{Champion: r.Champion, Position: r.Pos, Games: r.Play, WinRate: r.WinRate,
			PickRate: r.PickRate, BanRate: r.BanRate, Tier: opggTierLabel(r.Tier, r.Rank),
			Score: float64(6-t)*1000 + float64(1000-r.Rank)})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Score != rows[j].Score {
			return rows[i].Score > rows[j].Score
		}
		return rows[i].WinRate > rows[j].WinRate
	})
	return rows, tot
}

type opggTierInfo struct {
	Games   int
	Loading bool
	Done    int
	Total   int
	Elo     bool // i dati sono davvero dell'elo scelto
}

func opggTierList(mode, tf, pos string) ([]TierRow, opggTierInfo, error) {
	var info opggTierInfo
	if mode != "" && mode != "ranked" {
		return nil, info, errors.New("modalità non coperta da op.gg")
	}
	meta, err := opgg.laneMeta()
	if err != nil {
		return nil, info, err
	}
	ot := opggTier(tf)
	if ot == "" {
		rows, tot := metaToRows(meta, pos)
		info.Games, info.Elo = tot, true
		return rows, info, nil
	}
	job := opgg.tierJob(ot, meta)
	opgg.mu.Lock()
	defer opgg.mu.Unlock()
	if len(job.Rows) > 0 && !job.Busy {
		rows, tot := metaToRows(job.Rows, pos)
		info.Games, info.Elo = tot, true
		return rows, info, nil
	}
	// mentre carico l'elo scelto mostro i dati di tutti i ranghi
	src := meta
	if len(job.Rows) > 0 {
		src = job.Rows
	}
	rows, tot := metaToRows(src, pos)
	info.Games, info.Loading, info.Done, info.Total = tot, true, job.Done, job.Total
	return rows, info, nil
}

// la tier list di un elo si costruisce chiedendo a OP.GG le statistiche di ogni campione
// in ogni ruolo per quell'elo (in sottofondo, poi resta salvata per 12 ore)
func (o *opggClient) tierJob(ot string, meta []opggMetaRow) *opggTierJob {
	o.mu.Lock()
	j := o.jobs[ot]
	if j == nil {
		j = &opggTierJob{Tier: ot}
		readJSON(filepath.Join(opggDir(), "tier3_"+ot+".json"), j)
		o.jobs[ot] = j
	}
	stale := len(j.Rows) == 0 || time.Since(j.At) > 12*time.Hour
	start := stale && !j.Busy && time.Since(o.failAt) > 2*time.Minute
	if start {
		j.Busy, j.Done = true, 0
	}
	o.mu.Unlock()
	if start {
		go o.runTierJob(j, meta)
	}
	return j
}

func (o *opggClient) runTierJob(j *opggTierJob, meta []opggMetaRow) {
	// una chiamata per campione: OP.GG restituisce le statistiche di ogni ruolo per quell'elo
	mainPos := map[int]opggMetaRow{}
	for _, r := range meta {
		if m, ok := mainPos[r.Champion]; !ok || r.Play > m.Play {
			mainPos[r.Champion] = r
		}
	}
	var todo []opggMetaRow
	for _, r := range mainPos {
		todo = append(todo, r)
	}
	o.mu.Lock()
	j.Total = len(todo)
	o.mu.Unlock()
	fields := []string{"data.summary.positions[].name", "data.summary.positions[].stats.{play,win_rate,pick_rate,ban_rate,role_rate}",
		"data.summary.positions[].stats.tier_data.{tier,rank}"}
	var out []opggMetaRow
	var omu sync.Mutex
	fails := 0
	ch := make(chan opggMetaRow)
	var wg sync.WaitGroup
	for w := 0; w < 5; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range ch {
				o.mu.Lock()
				key := o.keys[p.Champion]
				o.mu.Unlock()
				var v any
				var err error
				for try := 0; try < 2; try++ {
					v, err = o.call("lol_get_champion_analysis", map[string]any{"game_mode": "ranked", "champion": key,
						"position": pos2opgg[p.Pos], "tier": j.Tier, "desired_output_fields": fields})
					if err == nil {
						break
					}
					time.Sleep(time.Second)
				}
				omu.Lock()
				if err != nil {
					fails++
				} else {
					for _, x := range ogl(v, "data", "summary", "positions") {
						ps := opgg2pos[strings.ToLower(ogs(x, "name"))]
						st := og(x, "stats")
						if ps == "" || ogi(st, "play") == 0 {
							continue
						}
						out = append(out, opggMetaRow{Champion: p.Champion, Pos: ps, Play: ogi(st, "play"), WinRate: ogf(st, "win_rate"),
							PickRate: ogf(st, "pick_rate"), BanRate: ogf(st, "ban_rate"), RoleRate: ogf(st, "role_rate"),
							Tier: ogi(st, "tier_data", "tier"), Rank: ogi(st, "tier_data", "rank")})
					}
				}
				omu.Unlock()
				o.mu.Lock()
				j.Done++
				o.mu.Unlock()
			}
		}()
	}
	for _, p := range todo {
		ch <- p
	}
	close(ch)
	wg.Wait()
	log.Printf("op.gg tier list %s: %d righe da %d campioni (%d errori)", j.Tier, len(out), len(todo), fails)
	o.mu.Lock()
	defer o.mu.Unlock()
	j.Busy = false
	if len(out) < len(todo)/2 {
		o.failAt = time.Now()
		return
	}
	j.Rows, j.At = out, time.Now()
	writeJSON(filepath.Join(opggDir(), "tier3_"+j.Tier+".json"), j)
}

// ---------- build del campione ----------

func opggOpt(x any, key string) Opt {
	n := ogi(x, "play")
	o := Opt{Key: key, IDs: ogids(x, "ids"), Games: n, Pick: ogf(x, "pick_rate")}
	if n > 0 {
		o.WinRate = ogf(x, "win") / float64(n)
	}
	if o.Key == "" {
		var s []string
		for _, id := range o.IDs {
			s = append(s, strconv.Itoa(id))
		}
		o.Key = strings.Join(s, ",")
	}
	return o
}

func opggBuild(champ int, pos, mode, tf string) (BuildResult, error) {
	if mode == "" {
		mode = "ranked"
	}
	if mode != "ranked" && mode != "flex" {
		return BuildResult{}, errors.New("modalità non coperta da op.gg")
	}
	if err := opgg.ensureChamps(); err != nil {
		return BuildResult{}, err
	}
	pos = strings.ToUpper(pos)
	if _, ok := pos2opgg[pos]; !ok {
		pos = opgg.mainPos(champ)
	}
	if pos == "" {
		return BuildResult{}, errors.New("ruolo sconosciuto")
	}
	if pos == "MID" {
		pos = "MIDDLE"
	}
	ot := opggTier(tf)
	ck := fmt.Sprintf("%d|%s|%s|%s", champ, pos, mode, ot)
	opgg.mu.Lock()
	if c, ok := opgg.builds[ck]; ok && time.Since(c.At) < 3*time.Hour {
		opgg.mu.Unlock()
		return c.B, nil
	}
	key := opgg.keys[champ]
	failed := time.Since(opgg.failAt) < time.Minute
	opgg.mu.Unlock()
	if key == "" || failed {
		return BuildResult{}, errors.New("op.gg non disponibile")
	}
	args := map[string]any{"game_mode": mode, "champion": key, "position": pos2opgg[pos], "desired_output_fields": []string{
		"data.summary.average_stats.{ban_rate,pick_rate,play,rank,tier,win_rate}",
		"data.summary.positions[].name",
		"data.summary.positions[].stats.{play,role_rate,win_rate,pick_rate,ban_rate}",
		"data.summary.positions[].stats.tier_data.{tier,rank}",
		"data.runes.{primary_page_id,primary_rune_ids[],secondary_page_id,secondary_rune_ids[],stat_mod_ids[],play,win,pick_rate}",
		"data.core_items.{ids[],pick_rate,play,win}",
		"data.boots.{ids[],pick_rate,play,win}",
		"data.starter_items.{ids[],pick_rate,play,win}",
		"data.fourth_items[].{ids[],pick_rate,play,win}",
		"data.fifth_items[].{ids[],pick_rate,play,win}",
		"data.sixth_items[].{ids[],pick_rate,play,win}",
		"data.last_items[].{ids[],pick_rate,play,win}",
		"data.summoner_spells.{ids[],pick_rate,play,win}",
		"data.skills.{order[],pick_rate,play,win}",
		"data.skill_masteries.{ids[],pick_rate,play,win}",
		"data.strong_counters[].{champion_id,my_win_rate,play}",
		"data.weak_counters[].{champion_id,my_win_rate,play}",
	}}
	if ot != "" {
		args["tier"] = ot
	}
	v, err := opgg.call("lol_get_champion_analysis", args)
	if err != nil {
		log.Printf("op.gg build %s %s: %v", key, pos, err)
		opgg.mu.Lock()
		opgg.failAt = time.Now()
		opgg.mu.Unlock()
		return BuildResult{}, err
	}
	d := og(v, "data")
	a := og(d, "summary", "average_stats")
	r := BuildResult{Champion: champ, Position: pos, Mode: mode, Tier: tf, Source: "OP.GG", ItemWR: map[int]float64{},
		Patch: majorMinor(static.version())}
	r.Games = ogi(a, "play")
	if r.Games == 0 {
		return r, errors.New("op.gg: nessun dato")
	}
	r.Matches = r.Games
	r.WinRate, r.PickRate, r.BanRate = ogf(a, "win_rate"), ogf(a, "pick_rate"), ogf(a, "ban_rate")
	r.TierLabel = opggTierLabel(ogi(a, "tier"), ogi(a, "rank"))
	for _, p := range ogl(d, "summary", "positions") {
		ps := opgg2pos[strings.ToLower(ogs(p, "name"))]
		if ps == "" {
			continue
		}
		st := og(p, "stats")
		r.Positions = append(r.Positions, PosShare{Position: ps, Games: ogi(st, "play"), Share: ogf(st, "role_rate")})
		if ps == pos && ogi(st, "play") > 0 { // i numeri in alto sono quelli del ruolo scelto, non del campione in generale
			r.Games, r.Matches = ogi(st, "play"), ogi(st, "play")
			r.WinRate, r.PickRate, r.BanRate = ogf(st, "win_rate"), ogf(st, "pick_rate"), ogf(st, "ban_rate")
			r.TierLabel = opggTierLabel(ogi(st, "tier_data", "tier"), ogi(st, "tier_data", "rank"))
		}
	}
	// rune
	if ru := og(d, "runes"); ru != nil {
		perks := append(append(ogids(ru, "primary_rune_ids"), ogids(ru, "secondary_rune_ids")...), ogids(ru, "stat_mod_ids")...)
		if len(perks) == 9 {
			n := ogi(ru, "play")
			rp := &RunePage{Primary: ogi(ru, "primary_page_id"), Secondary: ogi(ru, "secondary_page_id"), Perks: perks, Games: n, Source: "OP.GG"}
			if n > 0 {
				rp.WinRate = ogf(ru, "win") / float64(n)
			}
			r.Runes = rp
			r.RunePages = []*RunePage{rp}
			r.RuneKeystones = []Opt{{Key: strconv.Itoa(perks[0]), IDs: []int{perks[0]}, Games: n, WinRate: rp.WinRate, Pick: math.Max(0.01, ogf(ru, "pick_rate"))}}
		}
	}
	// incantesimi e abilità
	if sp := og(d, "summoner_spells"); sp != nil {
		o := opggOpt(sp, "")
		if len(o.IDs) == 2 {
			r.Spells, r.SpellsWR = o.IDs, o.WinRate
			r.SpellSets = []Opt{o}
		}
	}
	if sm := og(d, "skill_masteries"); sm != nil {
		ks := ogstr(sm, "ids")
		if len(ks) > 0 {
			r.Skills = strings.Join(ks, ">")
			o := opggOpt(sm, r.Skills)
			o.IDs = nil
			r.SkillOrders = []Opt{o}
		}
	}
	if sk := og(d, "skills"); sk != nil {
		seq := strings.Join(ogstr(sk, "order"), "")
		if seq != "" {
			r.SkillSeq, r.SkillSeqN = seq, ogi(sk, "play")
			if r.SkillSeqN > 0 {
				r.SkillSeqWR = ogf(sk, "win") / float64(r.SkillSeqN)
			}
			if len(seq) >= 3 {
				r.SkillFirst = seq[:3]
			}
		}
	}
	// oggetti
	if st := og(d, "starter_items"); st != nil {
		o := opggOpt(st, "")
		if len(o.IDs) > 0 {
			seen := map[int]bool{}
			var u []int
			for _, id := range o.IDs { // le pozioni ripetute contano una volta sola nell'import
				if !seen[id] {
					u = append(u, id)
					seen[id] = true
				}
			}
			r.Starting = u
			r.StartSets = []Opt{o}
		}
	}
	boots := 0
	if b := og(d, "boots"); b != nil {
		o := opggOpt(b, "")
		if len(o.IDs) > 0 {
			boots = o.IDs[0]
			r.BootsList = []Opt{o}
			r.ItemWR[boots] = o.WinRate
		}
	}
	core := opggOpt(og(d, "core_items"), "")
	r.Core = core.IDs
	r.BuildGames = core.Games
	for _, id := range core.IDs {
		r.ItemWR[id] = core.WinRate
	}
	var sit []int
	seen := map[int]bool{}
	for _, id := range core.IDs {
		seen[id] = true
	}
	for _, k := range []string{"fourth_items", "fifth_items", "sixth_items", "last_items"} {
		for _, x := range ogl(d, k) {
			o := opggOpt(x, "")
			for _, id := range o.IDs {
				if id == boots || seen[id] {
					continue
				}
				seen[id] = true
				sit = append(sit, id)
				r.ItemWR[id] = o.WinRate
				r.ItemsList = append(r.ItemsList, Opt{Key: strconv.Itoa(id), IDs: []int{id}, Games: o.Games, WinRate: o.WinRate, Pick: o.Pick})
			}
		}
	}
	r.Situational = sit
	if len(core.IDs) > 0 {
		r.Variants = []BuildVariant{{Label: "OP.GG", Path: core.IDs, Boots: boots, Games: core.Games, WinRate: core.WinRate,
			Share: math.Min(1, core.Pick), Core: core.IDs, Situational: sit, Runes: r.Runes}}
		if r.Runes != nil {
			r.Variants[0].Keystone = r.Runes.Perks[0]
		}
	}
	// counter
	pair := func(x any) Pair {
		wr := ogf(x, "my_win_rate")
		return Pair{Champion: ogi(x, "champion_id"), Games: ogi(x, "play"), WinRate: wr, Delta: wr - r.WinRate}
	}
	for _, x := range ogl(d, "strong_counters") {
		r.Best = append(r.Best, pair(x))
	}
	for _, x := range ogl(d, "weak_counters") {
		r.Worst = append(r.Worst, pair(x))
	}
	opgg.mu.Lock()
	opgg.builds[ck] = opggBuildCache{r, time.Now()}
	opgg.mu.Unlock()
	return r, nil
}
