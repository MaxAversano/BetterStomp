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
