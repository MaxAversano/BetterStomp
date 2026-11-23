<?php
// Rendimento campioni della stagione (solo Solo/Duo) di un giocatore, calcolato con la Riot API.
// Il client di League mostra solo le ultime 20 partite: qui si scorrono tutte quelle della stagione,
// un po' alla volta (le partite già lette restano salvate). ?k=TOKEN&puuid=...&since=ms
require __DIR__ . '/lib.php';
check_token();
@set_time_limit(60);
header('Content-Type: application/json; charset=utf-8');
header('Cache-Control: no-store');

// il PUUID del client non vale per la Riot API (ogni chiave ha il suo): uso il Riot ID
$name = trim((string)($_GET['name'] ?? ''));
$tag = trim((string)($_GET['tag'] ?? ''));
if ($name === '' || $tag === '') { http_response_code(400); exit('{"error":"Riot ID mancante"}'); }
$since = (int)($_GET['since'] ?? 0);
if ($since <= 0) $since = mktime(0, 0, 0, 1, 1, (int)date('Y')) * 1000;

$lock = fopen(STORE . '/run.lock', 'c');
$t0 = microtime(true);
while (!flock($lock, LOCK_EX | LOCK_NB)) { // aspetto che finisca la raccolta automatica
    if (microtime(true) - $t0 > 30) { echo '{"busy":true}'; exit; }
    usleep(500000);
}
$GLOBALS['STATE'] = load_json('state.json', ['hits' => []]);
$map = load_json('accounts.json', []);
$key = strtolower($name . '#' . $tag);
$puuid = $map[$key] ?? '';
if ($puuid === '') {
    for ($try = 0; $try < 10; $try++) {
        $r = riot('https://' . regional($REGION) . '.api.riotgames.com/riot/account/v1/accounts/by-riot-id/' . rawurlencode($name) . '/' . rawurlencode($tag));
        if (empty($r['wait'])) break;
        usleep(2000000); // limite di richieste momentaneamente pieno
    }
    $puuid = $r['data']['puuid'] ?? '';
    if ($puuid === '') {
        save_json('state.json', $GLOBALS['STATE']);
        flock($lock, LOCK_UN);
        $why = !empty($r['wait']) ? 'server occupato, riprovo tra poco' : (!empty($r['fatal']) ? 'chiave Riot non valida' : 'account non trovato (codice ' . ($r['error'] ?? '?') . ')');
        echo json_encode(['champs' => [], 'total' => 0, 'pending' => 1, 'error' => $why]);
        exit;
    }
    $map[$key] = $puuid;
    save_json('accounts.json', $map);
}
$file = 'player_' . md5($puuid) . '.json';
$p = load_json($file, ['done' => [], 'champs' => [], 'ids' => [], 'idsAt' => 0, 'since' => $since]);
if (($p['since'] ?? 0) != $since) $p = ['done' => [], 'champs' => [], 'ids' => [], 'idsAt' => 0, 'since' => $since];
$reg = 'https://' . regional($REGION) . '.api.riotgames.com';
$start = microtime(true);
$error = '';

// elenco partite della stagione (aggiornato ogni 10 minuti)
if (time() - $p['idsAt'] > 600) {
    $ids = [];
    for ($s = 0; $s < 1000; $s += 100) {
        $r = riot($reg . "/lol/match/v5/matches/by-puuid/$puuid/ids?queue=420&startTime=" . intdiv($since, 1000) . "&start=$s&count=100");
        if (!empty($r['wait']) || !empty($r['fatal']) || !isset($r['data'])) { $error = !empty($GLOBALS['STATE']['error']) ? $GLOBALS['STATE']['error'] : 'riprovo tra poco'; break; }
        $ids = array_merge($ids, $r['data']);
        if (count($r['data']) < 100) { $p['idsAt'] = time(); break; }
    }
    if ($ids) $p['ids'] = array_values(array_unique(array_merge($ids, $p['ids'])));
}

// legge le partite non ancora analizzate finché c'è tempo
foreach ($p['ids'] as $id) {
    if (isset($p['done'][$id])) continue;
    if (microtime(true) - $start > 22) break;
    $r = riot($reg . '/lol/match/v5/matches/' . $id);
    if (!empty($r['wait']) || !empty($r['fatal'])) break;
    $p['done'][$id] = 1;
    $m = $r['data'] ?? null;
    if (!$m || ($m['info']['gameDuration'] ?? 0) < 300) continue;
    foreach ($m['info']['participants'] as $pt) {
        if ($pt['puuid'] !== $puuid) continue;
        $c = (string)$pt['championId'];
        if (!isset($p['champs'][$c])) $p['champs'][$c] = ['g' => 0, 'w' => 0, 'k' => 0, 'd' => 0, 'a' => 0, 'cs' => 0, 'dur' => 0];
        $a = &$p['champs'][$c];
        $a['g']++; if (!empty($pt['win'])) $a['w']++;
        $a['k'] += $pt['kills']; $a['d'] += $pt['deaths']; $a['a'] += $pt['assists'];
        $a['cs'] += $pt['totalMinionsKilled'] + $pt['neutralMinionsKilled'];
        $a['dur'] += $m['info']['gameDuration'];
        unset($a);
        $pos = $pt['teamPosition'] ?? '';
        if ($pos !== '') $p['pos'][$pos] = ($p['pos'][$pos] ?? 0) + 1;
    }
}
save_json($file, $p);
save_json('state.json', $GLOBALS['STATE']);
flock($lock, LOCK_UN);

$out = [];
foreach ($p['champs'] as $c => $a) {
    $out[] = ['champion' => (int)$c, 'games' => $a['g'], 'wins' => $a['w'],
        'kda' => ($a['k'] + $a['a']) / max(1, $a['d']), 'csMin' => $a['dur'] > 0 ? $a['cs'] / ($a['dur'] / 60) : 0];
}
usort($out, function ($x, $y) { return $y['games'] - $x['games']; });
$pending = 0;
foreach ($p['ids'] as $id) if (!isset($p['done'][$id])) $pending++;
echo json_encode(['champs' => $out, 'total' => count($p['ids']), 'pending' => $pending, 'error' => $error, 'positions' => (object)($p['pos'] ?? [])]);
