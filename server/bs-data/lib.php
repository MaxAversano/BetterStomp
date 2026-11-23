<?php
// BetterStomp – motore statistiche lato server (hosting PHP, es. Aruba)
// Raccoglie partite classificate pubbliche con la Riot API ufficiale e salva gli
// aggregati nello stesso formato del programma, che li scarica e li unisce ai suoi.

require __DIR__ . '/config.php';

const TIERS = ['IRON','BRONZE','SILVER','GOLD','PLATINUM','EMERALD','DIAMOND','MASTER','GRANDMASTER','CHALLENGER'];
// cartella dati con nome non indovinabile (niente .htaccess: su alcuni hosting causa errori)
define('STORE', __DIR__ . '/dati_' . substr(hash('sha256', $TOKEN), 0, 16));
if (!is_dir(STORE)) { @mkdir(STORE, 0755, true); }
if (!is_file(STORE . '/index.php')) { @file_put_contents(STORE . '/index.php', "<?php http_response_code(404);\n"); }

function check_token() {
    global $TOKEN;
    if (!isset($_GET['k']) || !hash_equals($TOKEN, (string)$_GET['k'])) {
        http_response_code(403);
        exit('accesso negato');
    }
}

function regional($p) {
    if (in_array($p, ['na1','br1','la1','la2'])) return 'americas';
    if (in_array($p, ['kr','jp1'])) return 'asia';
    if (in_array($p, ['oc1','ph2','sg2','th2','tw2','vn2'])) return 'sea';
    return 'europe';
}

function load_json($file, $default) {
    $p = STORE . '/' . $file;
    if (!is_file($p)) return $default;
    $d = json_decode(file_get_contents($p), true);
    return is_array($d) ? $d : $default;
}

function save_json($file, $data) {
    $p = STORE . '/' . $file;
    file_put_contents($p . '.tmp', json_encode($data, JSON_UNESCAPED_UNICODE));
    rename($p . '.tmp', $p);
}

function http_get($url, $headers = [], &$code = null, &$respHeaders = null) {
    $ch = curl_init($url);
    $respHeaders = [];
    curl_setopt_array($ch, [
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_TIMEOUT => 12,
        CURLOPT_HTTPHEADER => $headers,
        CURLOPT_ENCODING => '',
        CURLOPT_HEADERFUNCTION => function ($c, $h) use (&$respHeaders) {
            $p = explode(':', $h, 2);
            if (count($p) == 2) $respHeaders[strtolower(trim($p[0]))] = trim($p[1]);
            return strlen($h);
        },
    ]);
    $body = curl_exec($ch);
    $code = curl_getinfo($ch, CURLINFO_HTTP_CODE);
    curl_close($ch);
    return $body;
}

// ---------- limiti della chiave (condivisi tra un'esecuzione e l'altra) ----------
$GLOBALS['RL'] = null;

function riot($url) {
    global $RIOT_KEY;
    $st = &$GLOBALS['STATE'];
    $now = microtime(true);
    $hits = array_values(array_filter(isset($st['hits']) ? $st['hits'] : [], function ($t) use ($now) { return $now - $t < 120; }));
    $last1 = count(array_filter($hits, function ($t) use ($now) { return $now - $t < 1; }));
    $max2 = $st['max2'] ?? 95; $max1 = $st['max1'] ?? 18;
    // la raccolta automatica lascia libere alcune richieste per il profilo (player.php)
    $reserve = isset($GLOBALS['RESERVE']) ? $GLOBALS['RESERVE'] : 0;
    if (count($hits) >= $max2 - $reserve) { $st['hits'] = $hits; return ['wait' => true]; }
    if ($last1 >= $max1) { usleep(1100000); }
    if (!empty($st['blockedUntil']) && $st['blockedUntil'] > time()) return ['wait' => true];
    $hits[] = microtime(true);
    $st['hits'] = $hits;
    $st['calls'] = ($st['calls'] ?? 0) + 1;
    $body = http_get($url, ['X-Riot-Token: ' . $RIOT_KEY], $code, $rh);
    if (!empty($rh['x-app-rate-limit'])) {
        foreach (explode(',', $rh['x-app-rate-limit']) as $part) {
            list($n, $s) = array_map('intval', explode(':', $part) + [0, 0]);
            if ($s == 1) $st['max1'] = max(1, intdiv($n * 9, 10));
            if ($s == 120) $st['max2'] = max(1, intdiv($n * 9, 10));
        }
    }
    if ($code == 429) { $st['blockedUntil'] = time() + (int)($rh['retry-after'] ?? 10); return ['wait' => true]; }
    if ($code == 401 || $code == 403) { $st['error'] = 'chiave Riot API non valida o scaduta'; return ['fatal' => true]; }
    if ($code != 200) return ['error' => $code];
    $st['error'] = '';
    return ['data' => json_decode($body, true)];
}

// ---------- dati statici (Data Dragon) ----------
function static_data() {
    $meta = load_json('ddragon.json', []);
    if (empty($meta['version']) || time() - ($meta['at'] ?? 0) > 3 * 3600) {
        $v = json_decode((string)http_get('https://ddragon.leagueoflegends.com/api/versions.json'), true);
        if (is_array($v) && $v) {
            if (($meta['version'] ?? '') !== $v[0] || empty($meta['items'])) {
                $items = json_decode((string)http_get("https://ddragon.leagueoflegends.com/cdn/{$v[0]}/data/en_US/item.json"), true);
                $cls = [];
                foreach (($items['data'] ?? []) as $id => $it) {
                    $tags = $it['tags'] ?? [];
                    $gold = $it['gold']['total'] ?? 0;
                    $into = !empty($it['into']);
                    if (in_array('Boots', $tags) && $gold > 300 && !$into) $cls[$id] = 'b';
                    elseif (!in_array('Consumable', $tags) && !in_array('Trinket', $tags) && !in_array('Boots', $tags) && !$into && $gold >= 900) $cls[$id] = 'c';
                }
                $meta['items'] = $cls;
            }
            $meta['version'] = $v[0];
            $meta['at'] = time();
            save_json('ddragon.json', $meta);
        }
    }
    return $meta;
}

function major_minor($v) { $p = explode('.', $v); return count($p) >= 2 ? $p[0] . '.' . $p[1] : $v; }

function queue_group($q, $mode) {
    $m = [420=>'ranked',440=>'flex',400=>'normal',430=>'normal',490=>'normal',480=>'normal',450=>'aram',2400=>'mayhem',1700=>'arena',1710=>'arena',900=>'urf',1900=>'urf',1010=>'urf'];
    if (isset($m[$q])) return $m[$q];
    $gm = strtoupper((string)$mode);
    if ($gm === 'ARAM') return 'aram';
    if ($gm === 'CHERRY') return 'arena';
    if (strpos($gm, 'URF') !== false) return 'urf';
    if (strpos($gm, 'KIWI') !== false || strpos($gm, 'MAYHEM') !== false) return 'mayhem';
    return '';
}

function positional($m) { return in_array($m, ['ranked','flex','normal']); }

function add_stat(&$map, $k, $win) {
    $k = (string)$k;
    if (!isset($map[$k])) $map[$k] = ['n' => 0, 'w' => 0];
    $map[$k]['n']++;
    if ($win) $map[$k]['w']++;
}

function new_agg() {
    return ['n'=>0,'w'=>0,'items'=>[],'boots'=>[],'start'=>[],'runes'=>[],'spells'=>[],'skills'=>[],'seq'=>[],'builds'=>[],'lane'=>[],'vs'=>[],'with'=>[],'k'=>0,'d'=>0,'a'=>0,'cs'=>0,'g'=>0,'kpm'=>0,'dur'=>0];
}

function rune_key($p) {
    $st = $p['perks']['styles'] ?? [];
    if (count($st) < 2) return '';
    $perks = [];
    foreach ($st as $s) foreach (($s['selections'] ?? []) as $sel) $perks[] = $sel['perk'];
    if (count($perks) != 6) return '';
    $sp = $p['perks']['statPerks'] ?? [];
    $perks[] = $sp['offense'] ?? 0; $perks[] = $sp['flex'] ?? 0; $perks[] = $sp['defense'] ?? 0;
    return $st[0]['style'] . ':' . $st[1]['style'] . ':' . implode(',', $perks);
}

function skill_key($seq) {
    $L = [1=>'Q',2=>'W',3=>'E',4=>'R'];
    $first = '';
    foreach ($seq as $s) { if (strlen($first) >= 3) break; $first .= $L[$s] ?? ''; }
    $count = [1=>0,2=>0,3=>0]; $reach = [1=>99,2=>99,3=>99];
    $lim = min(count($seq), 13);
    for ($i = 0; $i < $lim; $i++) { $s = $seq[$i]; if ($s == 4 || !isset($count[$s])) continue; $count[$s]++; $reach[$s] = $i; }
    $order = [1,2,3];
    usort($order, function ($a, $b) use ($count, $reach) { return $count[$a] != $count[$b] ? $count[$b] - $count[$a] : $reach[$a] - $reach[$b]; });
    return $L[$order[0]] . '>' . $L[$order[1]] . '>' . $L[$order[2]] . '|' . $first;
}

function seq_key($seq) { $L = [1=>'Q',2=>'W',3=>'E',4=>'R']; $s = ''; foreach ($seq as $x) $s .= $L[$x] ?? ''; return $s; }

// stesso calcolo di engine.go → ingest
function ingest(&$db, $m, $mode, $tier, $tl, $items, $region) {
    $key = $mode . '|' . $tier;
    if (!isset($db['buckets'][$key])) $db['buckets'][$key] = ['matches' => 0, 'bans' => [], 'champs' => []];
    $b = &$db['buckets'][$key];
    $b['matches']++;
    $db['total']++;
    foreach (($m['info']['teams'] ?? []) as $t) foreach (($t['bans'] ?? []) as $bn) {
        $c = (string)($bn['championId'] ?? 0);
        if ($c > 0) $b['bans'][$c] = ($b['bans'][$c] ?? 0) + 1;
    }
    $starts = []; $skills = []; $order = [];
    if ($tl) foreach (($tl['info']['frames'] ?? []) as $f) foreach (($f['events'] ?? []) as $ev) {
        $pid = $ev['participantId'] ?? 0;
        if (($ev['type'] ?? '') === 'ITEM_PURCHASED') {
            $it = $ev['itemId'] ?? 0;
            if (($ev['timestamp'] ?? 0) < 65000 && !in_array($it, [3340, 3363, 3364])) $starts[$pid][] = $it;
            if (($items[(string)$it] ?? '') === 'c') $order[$pid][] = $it;
        } elseif (($ev['type'] ?? '') === 'SKILL_LEVEL_UP' && in_array($ev['levelUpType'] ?? 'NORMAL', ['NORMAL', ''])) {
            $skills[$pid][] = $ev['skillSlot'] ?? 0;
        }
    }
    $parts = $m['info']['participants'] ?? [];
    $teamKills = [];
    foreach ($parts as $p) $teamKills[$p['teamId']] = ($teamKills[$p['teamId']] ?? 0) + $p['kills'];
    $isPos = positional($mode); $arena = $mode === 'arena';
    $dur = $m['info']['gameDuration'] ?? 0;
    foreach ($parts as $i => $p) {
        $pos = ($isPos && !empty($p['teamPosition'])) ? $p['teamPosition'] : 'ALL';
        $ck = $p['championId'] . '|' . $pos;
        if (!isset($b['champs'][$ck])) $b['champs'][$ck] = new_agg();
        $a = &$b['champs'][$ck];
        $win = !empty($p['win']);
        $a['n']++; if ($win) $a['w']++;
        $a['k'] += $p['kills']; $a['d'] += $p['deaths']; $a['a'] += $p['assists'];
        $a['cs'] += $p['totalMinionsKilled'] + $p['neutralMinionsKilled'];
        $a['g'] += $p['goldEarned'] ?? 0;
        $a['dur'] += $dur;
        if (($tk = $teamKills[$p['teamId']] ?? 0) > 0) $a['kpm'] += intdiv(($p['kills'] + $p['assists']) * 1000, $tk);
        $boots = 0; $final = [];
        for ($s = 0; $s <= 6; $s++) {
            $it = $p['item' . $s] ?? 0;
            $c = $items[(string)$it] ?? '';
            if ($c === 'b') { add_stat($a['boots'], $it, $win); $boots = $it; }
            elseif ($c === 'c') { add_stat($a['items'], $it, $win); $final[] = $it; }
        }
        $path = $final;
        if (!empty($order[$i + 1]) && count($order[$i + 1]) >= 3) $path = array_values(array_unique($order[$i + 1]));
        if (count($path) >= 3) add_stat($a['builds'], $boots . ':' . $path[0] . ',' . $path[1] . ',' . $path[2], $win);
        if (($rk = rune_key($p)) !== '') add_stat($a['runes'], $rk, $win);
        $s1 = $p['summoner1Id']; $s2 = $p['summoner2Id'];
        if ($s1 > $s2) { $tmp = $s1; $s1 = $s2; $s2 = $tmp; }
        add_stat($a['spells'], $s1 . ',' . $s2, $win);
        if (!empty($starts[$i + 1])) { $st = $starts[$i + 1]; sort($st); add_stat($a['start'], implode(',', $st), $win); }
        if (!empty($skills[$i + 1]) && count($skills[$i + 1]) >= 9) {
            add_stat($a['skills'], skill_key($skills[$i + 1]), $win);
            if (count($skills[$i + 1]) >= 15) add_stat($a['seq'], seq_key(array_slice($skills[$i + 1], 0, 15)), $win);
        }
        if (in_array($tier, ['MASTER', 'GRANDMASTER', 'CHALLENGER'])) add_pro($db, $m, $p, $tier, $region, $mode, $starts[$i + 1] ?? [], $skills[$i + 1] ?? []);
        if (!$arena) foreach ($parts as $o) {
            if ($o['puuid'] === $p['puuid']) continue;
            if ($o['teamId'] != $p['teamId']) {
                add_stat($a['vs'], $o['championId'], $win);
                if ($isPos && !empty($o['teamPosition']) && $o['teamPosition'] === ($p['teamPosition'] ?? '')) add_stat($a['lane'], $o['championId'], $win);
            } else {
                add_stat($a['with'], $o['championId'], $win);
            }
        }
        unset($a);
    }
    unset($b);
}

function add_pro(&$db, $m, $p, $tier, $region, $mode, $starts, $skills) {
    $g = ['name' => $p['riotIdGameName'] ?? ($p['summonerName'] ?? ''), 'tag' => $p['riotIdTagline'] ?? '', 'tier' => $tier, 'pro' => false,
        'region' => $region, 'champ' => $p['championId'], 'pos' => $p['teamPosition'] ?? '', 'opp' => 0, 'win' => !empty($p['win']),
        'items' => [], 'start' => array_values($starts), 'runes' => rune_key($p), 'spells' => [$p['summoner1Id'], $p['summoner2Id']],
        'seq' => count($skills) >= 6 ? seq_key(array_slice($skills, 0, 18)) : '', 'mode' => $mode,
        'when' => ($m['info']['gameStartTimestamp'] ?? 0) + ($m['info']['gameDuration'] ?? 0) * 1000, 'match' => $m['metadata']['matchId']];
    for ($s = 0; $s < 6; $s++) if (($it = $p['item' . $s] ?? 0) > 0) $g['items'][] = $it;
    foreach ($m['info']['participants'] as $o) if ($o['teamId'] != $p['teamId'] && !empty($o['teamPosition']) && $o['teamPosition'] === ($p['teamPosition'] ?? '')) $g['opp'] = $o['championId'];
    $c = (string)$p['championId'];
    $list = $db['pro'][$c] ?? [];
    $list[] = $g;
    usort($list, function ($x, $y) { return $y['when'] <=> $x['when']; });
    $db['pro'][$c] = array_slice($list, 0, 30);
}

// Go si aspetta oggetti JSON per le mappe (anche vuote o con chiavi numeriche)
function to_go($db) {
    $obj = function ($a) { return (object)($a ?: []); };
    $out = ['patch' => $db['patch'], 'total' => $db['total'], 'buckets' => [], 'seen' => new stdClass(), 'pro' => []];
    foreach ($db['buckets'] as $k => $b) {
        $champs = [];
        foreach ($b['champs'] as $ck => $a) {
            foreach (['items','boots','start','runes','spells','skills','seq','builds','lane','vs','with'] as $f) $a[$f] = $obj($a[$f]);
            $champs[$ck] = $a;
        }
        $out['buckets'][$k] = ['matches' => $b['matches'], 'bans' => $obj($b['bans']), 'champs' => $obj($champs)];
    }
    $out['buckets'] = $obj($out['buckets']);
    foreach (($db['pro'] ?? []) as $c => $l) $out['pro'][(string)$c] = array_values($l);
    $out['pro'] = $obj($out['pro']);
    return $out;
}
