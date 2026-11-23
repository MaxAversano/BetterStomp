<?php
// Avviato ogni minuto (cron di Aruba o cron-job.org): raccoglie partite per circa 25 secondi.
require __DIR__ . '/lib.php';
check_token();
@set_time_limit(60);
ignore_user_abort(true);
header('Content-Type: text/plain; charset=utf-8');

$lock = fopen(STORE . '/run.lock', 'c');
if (!flock($lock, LOCK_EX | LOCK_NB)) exit("già in esecuzione\n");

$start = microtime(true);
$budget = 25; // secondi per esecuzione (sotto i limiti dell'hosting condiviso)
$GLOBALS['STATE'] = load_json('state.json', ['tierIdx' => 0, 'pools' => [], 'hits' => [], 'calls' => 0, 'runs' => 0, 'error' => '']);
$st = &$GLOBALS['STATE'];
$st['runs'] = ($st['runs'] ?? 0) + 1;
$st['lastRun'] = time();

$meta = static_data();
if (empty($meta['version'])) { save_json('state.json', $st); exit("Data Dragon non raggiungibile\n"); }
$patch = major_minor($meta['version']);
$items = $meta['items'] ?? [];
$dbFile = 'stats_' . $patch . '.json';
$db = load_json($dbFile, null);
if (!$db) $db = ['patch' => $patch, 'total' => 0, 'buckets' => [], 'pro' => []];
$seen = load_json('seen_' . $patch . '.json', []);
$plat = 'https://' . $REGION . '.api.riotgames.com';
$reg = 'https://' . regional($REGION) . '.api.riotgames.com';
$added = 0;

while (microtime(true) - $start < $budget) {
    $tier = TIERS[$st['tierIdx'] % count(TIERS)];
    $st['tierIdx']++;
    // giocatore casuale di quel rango
    $pool = $st['pools'][$tier] ?? [];
    if (!$pool) {
        if (in_array($tier, ['MASTER', 'GRANDMASTER', 'CHALLENGER'])) {
            $r = riot($plat . '/lol/league/v4/' . strtolower($tier) . 'leagues/by-queue/RANKED_SOLO_5x5');
            $entries = $r['data']['entries'] ?? [];
        } else {
            $div = ['I', 'II', 'III', 'IV'][random_int(0, 3)];
            $r = riot($plat . "/lol/league/v4/entries/RANKED_SOLO_5x5/$tier/$div?page=" . random_int(1, 3));
            $entries = $r['data'] ?? [];
        }
        if (!empty($r['wait']) || !empty($r['fatal'])) break;
        foreach ($entries as $e) if (!empty($e['puuid'])) $pool[] = $e['puuid'];
        shuffle($pool);
        $pool = array_slice($pool, 0, 60);
    }
    if (!$pool) continue;
    $puuid = array_pop($pool);
    $st['pools'][$tier] = $pool;
    $r = riot($reg . "/lol/match/v5/matches/by-puuid/$puuid/ids?start=0&count=12");
    if (!empty($r['wait']) || !empty($r['fatal'])) break;
    $taken = 0;
    foreach (($r['data'] ?? []) as $id) {
        if ($taken >= 5 || microtime(true) - $start > $budget) break;
        if (isset($seen[$id])) continue;
        $mr = riot($reg . '/lol/match/v5/matches/' . $id);
        if (!empty($mr['wait']) || !empty($mr['fatal'])) break 2;
        $seen[$id] = 1;
        $m = $mr['data'] ?? null;
        if (!$m) continue;
        if (major_minor($m['info']['gameVersion'] ?? '') !== $patch) break;
        $mode = queue_group($m['info']['queueId'] ?? 0, $m['info']['gameMode'] ?? '');
        if ($mode === '' || ($m['info']['gameDuration'] ?? 0) < 300) continue;
        $tl = null;
        if (positional($mode) && mt_rand(1, 100) <= 35) {
            $tr = riot($reg . '/lol/match/v5/matches/' . $id . '/timeline');
            $tl = $tr['data'] ?? null;
        }
        ingest($db, $m, $mode, $tier, $tl, $items, $REGION);
        $taken++; $added++;
    }
}

if ($added > 0) {
    save_json($dbFile, $db);
    file_put_contents(STORE . '/export_' . $patch . '.json.tmp', json_encode(to_go($db), JSON_UNESCAPED_UNICODE));
    rename(STORE . '/export_' . $patch . '.json.tmp', STORE . '/export_' . $patch . '.json');
}
if (count($seen) > 400000) $seen = array_slice($seen, -300000, null, true);
save_json('seen_' . $patch . '.json', $seen);
$st['patch'] = $patch;
$st['total'] = $db['total'];
$st['lastAdded'] = $added;
save_json('state.json', $st);
flock($lock, LOCK_UN);
echo "ok: +$added partite (totale {$db['total']}, patch $patch)\n";
