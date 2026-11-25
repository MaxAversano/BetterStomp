<?php
// Diagnostica: apri questa pagina per controllare che PHP e le connessioni funzionino
header('Content-Type: text/plain; charset=utf-8');
echo "PHP " . PHP_VERSION . "\n";
echo "curl: " . (function_exists('curl_init') ? 'si' : 'NO') . "\n";
echo "scrittura cartella: " . (is_writable(__DIR__) ? 'si' : 'NO') . "\n";
if (function_exists('curl_init')) {
    $ch = curl_init('https://ddragon.leagueoflegends.com/api/versions.json');
    curl_setopt($ch, CURLOPT_RETURNTRANSFER, true);
    curl_setopt($ch, CURLOPT_TIMEOUT, 10);
    $r = curl_exec($ch);
    echo "connessione a Riot: " . ($r ? 'si' : 'NO (' . curl_error($ch) . ')') . "\n";
}

// stato della raccolta (niente dati riservati: solo numeri e l'ultimo errore)
if (is_file(__DIR__ . '/config.php')) {
    require __DIR__ . '/config.php';
    $store = __DIR__ . '/dati_' . substr(hash('sha256', $TOKEN), 0, 16);
    echo "memory_limit: " . ini_get('memory_limit') . " · max_execution_time: " . ini_get('max_execution_time') . "\n";
    $st = is_file($store . '/state.json') ? json_decode(file_get_contents($store . '/state.json'), true) : null;
    if ($st) {
        echo "patch: " . ($st['patch'] ?? '?') . " · partite: " . ($st['total'] ?? 0) . " · esecuzioni: " . ($st['runs'] ?? 0) . "\n";
        echo "ultima esecuzione: " . (!empty($st['lastRun']) ? date('Y-m-d H:i:s', $st['lastRun']) : 'mai') . " (+" . ($st['lastAdded'] ?? 0) . ")\n";
        if (!empty($st['error'])) echo "errore Riot: " . $st['error'] . "\n";
    }
    if (is_file($store . '/last_error.txt')) echo "ultimo errore PHP: " . file_get_contents($store . '/last_error.txt') . "\n";
    $files = glob($store . '/*.json') ?: [];
    usort($files, function ($a, $b) { return filesize($b) - filesize($a); });
    foreach (array_slice($files, 0, 6) as $f) echo "file " . basename($f) . ": " . round(filesize($f) / 1048576, 1) . " MB\n";
    $sh = glob($store . '/s_*/*.json') ?: [];
    if ($sh) { $big = 0; foreach ($sh as $f) $big = max($big, filesize($f)); echo "file divisi: " . count($sh) . " (il più grande " . round($big / 1048576, 1) . " MB)\n"; }
    $free = @disk_free_space(__DIR__);
    if ($free) echo "spazio libero: " . round($free / 1073741824, 1) . " GB\n";
}
