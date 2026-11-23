<?php
// Statistiche per BetterStomp: ?k=TOKEN (dati) oppure ?k=TOKEN&status=1 (stato del motore)
require __DIR__ . '/config.php';
define('STORE', __DIR__ . '/store');
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
$file = STORE . '/export_' . $patch . '.json';
if (!$patch || !is_file($file)) { http_response_code(404); exit('nessun dato'); }
header('Content-Type: application/json; charset=utf-8');
header('Last-Modified: ' . gmdate('D, d M Y H:i:s', filemtime($file)) . ' GMT');
if (!ini_get('zlib.output_compression') && strpos($_SERVER['HTTP_ACCEPT_ENCODING'] ?? '', 'gzip') !== false && function_exists('gzencode')) {
    header('Content-Encoding: gzip');
    echo gzencode(file_get_contents($file), 6);
} else {
    readfile($file);
}
