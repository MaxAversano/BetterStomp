<?php
// Distribuzione di BetterStomp.exe per gli amici (aggiornamento automatico).
//   ?v=1                      versione pubblicata (JSON, pubblico)
//   ?get=1                    scarica l'exe (pubblico)
//   POST ?k=TOKEN&up=1&i=0&n=9&ver=X&sha=Y   carica l'exe a pezzi (solo con il codice del server)
require __DIR__ . '/config.php';
$dir = __DIR__ . '/app';
if (!is_dir($dir)) @mkdir($dir, 0755, true);
$verFile = $dir . '/version.json';
$exe = $dir . '/BetterStomp.exe';
header('Cache-Control: no-store');

if (isset($_GET['v'])) {
    header('Content-Type: application/json; charset=utf-8');
    echo is_file($verFile) ? file_get_contents($verFile) : '{"version":""}';
    exit;
}

if (isset($_GET['get'])) {
    if (!is_file($exe)) { http_response_code(404); exit('nessuna versione pubblicata'); }
    header('Content-Type: application/octet-stream');
    header('Content-Disposition: attachment; filename="BetterStomp.exe"');
    header('Content-Length: ' . filesize($exe));
    readfile($exe);
    exit;
}

if (isset($_GET['up'])) {
    if (!isset($_GET['k']) || !hash_equals($TOKEN, (string)$_GET['k'])) { http_response_code(403); exit('accesso negato'); }
    $i = (int)($_GET['i'] ?? 0);
    $n = (int)($_GET['n'] ?? 0);
    $ver = preg_replace('/[^0-9.]/', '', $_GET['ver'] ?? '');
    $sha = preg_replace('/[^0-9a-f]/', '', strtolower($_GET['sha'] ?? ''));
    if ($n < 1 || $n > 200 || $i < 0 || $i >= $n || !$ver || strlen($sha) !== 64) { http_response_code(400); exit('parametri non validi'); }
    $tmp = $dir . '/upload_' . $sha . '.part';
    if ($i === 0) @unlink($tmp);
    $data = file_get_contents('php://input');
    if ($data === false || $data === '') { http_response_code(400); exit('pezzo vuoto'); }
    if (($i === 0 ? 0 : (is_file($tmp) ? filesize($tmp) : -1)) < 0) { http_response_code(409); exit('pezzo fuori ordine'); }
    file_put_contents($tmp, $data, FILE_APPEND);
    if ($i < $n - 1) { echo 'ok'; exit; }
    if (hash_file('sha256', $tmp) !== $sha) { @unlink($tmp); http_response_code(422); exit('file corrotto'); }
    rename($tmp, $exe);
    file_put_contents($verFile, json_encode(['version' => $ver, 'size' => filesize($exe), 'sha256' => $sha, 'at' => time()]));
    echo 'pubblicato';
    exit;
}

http_response_code(400);
echo 'richiesta non valida';
