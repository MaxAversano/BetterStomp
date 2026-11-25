<?php
// Statistiche per BetterStomp: ?k=TOKEN (dati) oppure ?k=TOKEN&status=1 (stato del motore)
require __DIR__ . '/config.php';
define('STORE', __DIR__ . '/dati_' . substr(hash('sha256', $TOKEN), 0, 16));
if (!isset($_GET['k']) || !hash_equals($TOKEN, (string)$_GET['k'])) { http_response_code(403); exit('accesso negato'); }
header('Cache-Control: no-store');

$state = is_file(STORE . '/state.json') ? json_decode(file_get_contents(STORE . '/state.json'), true) : [];
if (isset($_GET['status'])) {
    header('Content-Type: application/json; charset=utf-8');
    echo json_encode([
        'patch' => $state['patch'] ?? '', 'total' => $state['total'] ?? 0, 'lastRun' => $state['lastRun'] ?? 0,
        'lastAdded' => $state['lastAdded'] ?? 0, 'runs' => $state['runs'] ?? 0, 'error' => $state['error'] ?? '',
        'region' => $REGION, 'php' => PHP_VERSION,
    ]);
    exit;
}
$patch = preg_replace('/[^0-9.]/', '', $_GET['patch'] ?? ($state['patch'] ?? ''));
$xdir = STORE . '/x_' . $patch;
$parts = $patch && is_dir($xdir) ? (glob($xdir . '/b_*.json') ?: []) : [];
if ($parts) {
    // statistiche divise per gruppo: le rimetto insieme come testo, senza decodificarle
    $latest = 0;
    foreach ($parts as $f) $latest = max($latest, filemtime($f));
    $body = '{"patch":' . json_encode($patch) . ',"total":' . (int)($state['total'] ?? 0) . ',"buckets":{';
    $first = true;
    foreach ($parts as $f) {
        $j = trim((string)file_get_contents($f));
        if (strlen($j) < 3 || $j[0] !== '{') continue;
        $body .= ($first ? '' : ',') . substr($j, 1, -1);
        $first = false;
    }
    $pro = is_file($xdir . '/pro.json') ? trim(file_get_contents($xdir . '/pro.json')) : '{}';
    $body .= '},"seen":{},"pro":' . ($pro !== '' ? $pro : '{}') . '}';
} else {
    $file = STORE . '/export_' . $patch . '.json';
    if (!$patch || !is_file($file)) { http_response_code(404); exit('nessun dato'); }
    $latest = filemtime($file);
    $body = file_get_contents($file);
}
header('Content-Type: application/json; charset=utf-8');
header('Last-Modified: ' . gmdate('D, d M Y H:i:s', $latest) . ' GMT');
if (!ini_get('zlib.output_compression') && strpos($_SERVER['HTTP_ACCEPT_ENCODING'] ?? '', 'gzip') !== false && function_exists('gzencode')) {
    header('Content-Encoding: gzip');
    echo gzencode($body, 6);
} else {
    echo $body;
}
