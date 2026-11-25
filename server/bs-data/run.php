<?php
// Avviato ogni minuto (cron di Aruba o cron-job.org): raccoglie partite per circa 50 secondi,
// da più regioni insieme (ognuna ha la sua quota di richieste nella Riot API).
require __DIR__ . '/lib.php';
check_token();
@set_time_limit(110);
// se PHP si blocca (memoria, tempo...) l'errore resta scritto e ping.php lo mostra
register_shutdown_function(function () {
    $e = error_get_last();
    if ($e && in_array($e['type'], [E_ERROR, E_PARSE, E_CORE_ERROR, E_COMPILE_ERROR], true)) {
        @file_put_contents(STORE . '/last_error.txt', date('c') . ' ' . $e['message'] . ' (' . basename($e['file']) . ':' . $e['line'] . ')');
    }
});
ignore_user_abort(true);
header('Content-Type: text/plain; charset=utf-8');

$lock = fopen(STORE . '/run.lock', 'c');
if (!flock($lock, LOCK_EX | LOCK_NB)) exit("già in esecuzione\n");

$start = microtime(true);
$budget = 50; // secondi per esecuzione (il cron riparte ogni minuto)
$GLOBALS['RESERVE'] = 20; // richieste lasciate al calcolo della stagione (per host)
$GLOBALS['STATE'] = load_json('state.json', ['tierIdx' => 0, 'pools' => [], 'hits' => [], 'calls' => 0, 'runs' => 0, 'error' => '']);
$st = &$GLOBALS['STATE'];
$st['runs'] = ($st['runs'] ?? 0) + 1;
if (!is_array($st['tierIdx'] ?? null)) $st['tierIdx'] = [];
foreach (array_keys($st['pools'] ?? []) as $k) if (strpos($k, '|') === false) unset($st['pools'][$k]); // vecchie chiavi senza regione
$st['lastRun'] = time();

$meta = static_data();
if (empty($meta['version'])) { save_json('state.json', $st); exit("Data Dragon non raggiungibile\n"); }
$patch = major_minor($meta['version']);
// prima un po' di lavoro sulle fonti esterne (LoLalytics, onetricks.gg), poi la raccolta Riot
try { $extSteps = ext_work(14, $meta['version']); } catch (Throwable $e) { $extSteps = 0; @file_put_contents(STORE . '/ext_error.txt', date('c') . ' ' . $e->getMessage()); }
$items = $meta['items'] ?? [];
$sdir = shard_dir($patch);
$oldFile = STORE . '/stats_' . $patch . '.json';
if (is_file($oldFile)) {
    // conversione una tantum dal vecchio file unico; se una volta è già andata in crash
    // per la memoria, lo metto da parte e riparto (meglio che restare bloccati)
    $mark = STORE . '/migrating_' . $patch;
    if (is_file($mark)) {
        rename($oldFile, $oldFile . '.troppo-grande');
        @unlink($mark);
    } else {
        touch($mark);
        @ini_set('memory_limit', '-1');
        $old = json_decode(file_get_contents($oldFile), true);
        if (is_array($old)) {
            $xdir = export_dir($patch);
            foreach (($old['buckets'] ?? []) as $k => $b) {
                write_atomic($sdir . '/' . bucket_file($k), json_encode(['key' => $k, 'b' => $b], JSON_UNESCAPED_UNICODE));
                write_atomic($xdir . '/' . bucket_file($k), json_encode([$k => bucket_go($b)], JSON_UNESCAPED_UNICODE));
            }
            write_atomic($sdir . '/meta.json', json_encode(['total' => $old['total'] ?? 0, 'pro' => $old['pro'] ?? []], JSON_UNESCAPED_UNICODE));
            write_atomic($xdir . '/pro.json', json_encode(pro_go($old['pro'] ?? []), JSON_UNESCAPED_UNICODE));
            unset($old);
            rename($oldFile, $oldFile . '.convertito');
            @unlink(STORE . '/export_' . $patch . '.json');
        }
        @unlink($mark);
        @ini_set('memory_limit', '1024M');
    }
}
$meta0 = is_file($sdir . '/meta.json') ? json_decode(file_get_contents($sdir . '/meta.json'), true) : null;
$db = ['patch' => $patch, 'total' => $meta0['total'] ?? 0, 'buckets' => [], 'pro' => $meta0['pro'] ?? []];
unset($meta0);
$dirty = [];
$loadBucket = function ($key) use (&$db, &$dirty, $sdir) {
    if (!isset($db['buckets'][$key])) {
        $f = $sdir . '/' . bucket_file($key);
        $d = is_file($f) ? json_decode(file_get_contents($f), true) : null;
        $db['buckets'][$key] = $d['b'] ?? ['matches' => 0, 'bans' => [], 'champs' => []];
    }
    $dirty[$key] = true;
};
$seen = load_json('seen_' . $patch . '.json', []);
// la regione del server per prima; le altre raddoppiano/triplicano le partite raccolte
$regions = array_values(array_unique([$REGION, 'euw1', 'na1', 'kr']));
$out = []; // regioni senza più richieste disponibili in questa esecuzione
$added = 0;
$byReg = [];

// se la memoria sale troppo, salvo i gruppi già aggiornati e li tolgo dalla memoria
$flush = function () use (&$db, &$dirty, $sdir, $patch) {
    $xdir = export_dir($patch);
    foreach (array_keys($dirty) as $k) {
        write_atomic($sdir . '/' . bucket_file($k), json_encode(['key' => $k, 'b' => $db['buckets'][$k]], JSON_UNESCAPED_UNICODE));
        write_atomic($xdir . '/' . bucket_file($k), json_encode([$k => bucket_go($db['buckets'][$k])], JSON_UNESCAPED_UNICODE));
        unset($db['buckets'][$k]);
    }
    $dirty = [];
};

$ri = $st['regIdx'] ?? 0;
while (microtime(true) - $start < $budget && count($out) < count($regions)) {
    $region = $regions[$ri++ % count($regions)];
    if (isset($out[$region])) continue;
    $plat = 'https://' . $region . '.api.riotgames.com';
    $reg = 'https://' . regional($region) . '.api.riotgames.com';
    $ti = $st['tierIdx'][$region] ?? 0;
    $tier = TIERS[$ti % count(TIERS)];
    $st['tierIdx'][$region] = $ti + 1;
    // giocatore casuale di quel rango
    $pkey = $region . '|' . $tier;
    $pool = $st['pools'][$pkey] ?? [];
    if (!$pool) {
        if (in_array($tier, ['MASTER', 'GRANDMASTER', 'CHALLENGER'])) {
            $r = riot($plat . '/lol/league/v4/' . strtolower($tier) . 'leagues/by-queue/RANKED_SOLO_5x5');
            $entries = $r['data']['entries'] ?? [];
        } else {
            $div = ['I', 'II', 'III', 'IV'][random_int(0, 3)];
            $r = riot($plat . "/lol/league/v4/entries/RANKED_SOLO_5x5/$tier/$div?page=" . random_int(1, 3));
            $entries = $r['data'] ?? [];
        }
        if (!empty($r['fatal'])) break;
        if (!empty($r['wait'])) { $out[$region] = 1; continue; }
        foreach ($entries as $e) if (!empty($e['puuid'])) $pool[] = $e['puuid'];
        shuffle($pool);
        $pool = array_slice($pool, 0, 60);
    }
    if (!$pool) continue;
    $puuid = array_pop($pool);
    $st['pools'][$pkey] = $pool;
    $r = riot($reg . "/lol/match/v5/matches/by-puuid/$puuid/ids?start=0&count=12");
    if (!empty($r['fatal'])) break;
    if (!empty($r['wait'])) { $out[$region] = 1; continue; }
    $taken = 0;
    foreach (($r['data'] ?? []) as $id) {
        if ($taken >= 5 || microtime(true) - $start > $budget) break;
        if (isset($seen[$id])) continue;
        $mr = riot($reg . '/lol/match/v5/matches/' . $id);
        if (!empty($mr['fatal'])) break 2;
        if (!empty($mr['wait'])) { $out[$region] = 1; break; }
        $seen[$id] = 1;
        $m = $mr['data'] ?? null;
        if (!$m) continue;
        if (major_minor($m['info']['gameVersion'] ?? '') !== $patch) break;
        $mode = queue_group($m['info']['queueId'] ?? 0, $m['info']['gameMode'] ?? '');
        if ($mode === '' || ($m['info']['gameDuration'] ?? 0) < 300) continue;
        $tl = null;
        if (positional($mode) && mt_rand(1, 100) <= 25) {
            $tr = riot($reg . '/lol/match/v5/matches/' . $id . '/timeline');
            $tl = $tr['data'] ?? null;
        }
        $loadBucket($mode . '|' . $tier);
        ingest($db, $m, $mode, $tier, $tl, $items, $region);
        $taken++; $added++;
        $byReg[$region] = ($byReg[$region] ?? 0) + 1;
        if (memory_get_usage(true) > 500 * 1048576) $flush();
    }
}
$st['regIdx'] = $ri % count($regions);
$st['byRegion'] = $byReg;

if ($added > 0) {
    $xdir = export_dir($patch);
    foreach (array_keys($dirty) as $k) {
        write_atomic($sdir . '/' . bucket_file($k), json_encode(['key' => $k, 'b' => $db['buckets'][$k]], JSON_UNESCAPED_UNICODE));
        write_atomic($xdir . '/' . bucket_file($k), json_encode([$k => bucket_go($db['buckets'][$k])], JSON_UNESCAPED_UNICODE));
    }
    write_atomic($sdir . '/meta.json', json_encode(['total' => $db['total'], 'pro' => $db['pro']], JSON_UNESCAPED_UNICODE));
    write_atomic($xdir . '/pro.json', json_encode(pro_go($db['pro']), JSON_UNESCAPED_UNICODE));
}
if (count($seen) > 150000) $seen = array_slice($seen, -100000, null, true);
save_json('seen_' . $patch . '.json', $seen);
$st['patch'] = $patch;
$st['total'] = $db['total'];
$st['lastAdded'] = $added;
save_json('state.json', $st);
flock($lock, LOCK_UN);
@unlink(STORE . '/last_error.txt');
echo "ok: +$added partite (totale {$db['total']}, patch $patch) · fonti esterne: $extSteps passi\n";
