<?php
// A creator's recorded LIVE sessions, following data.nextCursor page by page.
// Usage: php sessions.php [HANDLE]     e.g. php sessions.php tiktok
// Scope: live:read

declare(strict_types=1);

require __DIR__ . '/TokTokData.php';

use TokTokData\Examples\Client;

use function TokTokData\Examples\exitWithError;

const MAX_SESSIONS = 50;

$handle = ltrim($argv[1] ?? 'tiktok', '@');
$n = static fn ($v): string => $v === null ? '-' : number_format((float) $v);
$client = new Client();
$cursor = null;
$shown = 0;

try {
    do {
        $data = $client->get('/v1/live/sessions', ['creatorUniqueId' => $handle, 'limit' => 20, 'cursor' => $cursor])['data'];
        foreach ($data['items'] as $s) {
            $minutes = $s['durationSeconds'] === null ? '-' : (string) round($s['durationSeconds'] / 60);
            printf(
                "  %s  %4s min  peak %7s viewers  %9s diamonds\n",
                $s['startedAt'],
                $minutes,
                $n($s['peakViewerCount']),
                $n($s['totalGiftDiamonds']),
            );
            if (++$shown >= MAX_SESSIONS) {
                break 2;
            }
        }
        $cursor = $data['nextCursor'] ?? null; // null on the last page
    } while ($cursor !== null);
} catch (Throwable $e) {
    exitWithError($e);
}

if ($shown === 0) {
    echo "  no recorded sessions\n";
}
