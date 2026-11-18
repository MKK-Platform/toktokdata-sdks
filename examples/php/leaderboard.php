<?php
// Top 10 of an official TikTok LIVE board for one region.
// Usage: php leaderboard.php [REGION] [BOARD]     e.g. php leaderboard.php VN hourly
// Scope: rank:read

declare(strict_types=1);

require __DIR__ . '/TokTokData.php';

use TokTokData\Examples\Client;

use function TokTokData\Examples\exitWithError;

$region = $argv[1] ?? 'VN';
$board = $argv[2] ?? 'hourly';

try {
    $res = (new Client())->get('/v1/rankings/official', ['board' => $board, 'region' => $region, 'limit' => 10]);
} catch (Throwable $e) {
    exitWithError($e);
}

$b = $res['data']['board'];
printf("%s board, %s, period %s\n", $b['board'], $b['region'], $b['periodKey']);
if (!$b['publishable']) {
    // Coverage below the gate: the order may still change, so label it as partial.
    printf("(partial: coverage %.0f%% of the %.0f%% gate)\n", $b['coverage']['ratio'] * 100, $b['coverage']['threshold'] * 100);
}
foreach ($b['entries'] as $e) {
    $who = $e['masked'] ? "(masked, plan {$e['unmaskTier']})" : trim(($e['nickname'] ?? '') . ' @' . $e['uniqueId']);
    printf("%3d  %-40s %12s%s\n", $e['rank'], $who, number_format((float) $e['score']), $e['isLive'] ? '  LIVE' : '');
}
