<?php
// A creator profile. A creator not stored yet answers 202 "resolving"; the client waits and asks again.
// Usage: php creator.php [HANDLE]     e.g. php creator.php tiktok
// Scope: creator:read

declare(strict_types=1);

require __DIR__ . '/TokTokData.php';

use TokTokData\Examples\Client;

use function TokTokData\Examples\exitWithError;

$handle = ltrim($argv[1] ?? 'tiktok', '@');
$n = static fn ($v): string => $v === null ? '-' : number_format((float) $v);

try {
    $c = (new Client())->get('/v1/creators/' . rawurlencode($handle))['data']['creator'];
} catch (Throwable $e) {
    exitWithError($e);
}

printf("%s (@%s)%s\n", $c['displayName'] ?? $c['uniqueId'], $c['uniqueId'], $c['verified'] ? ' [verified]' : '');
printf(
    "  region %s, %s followers, %s videos, %s likes\n",
    $c['region'] ?? '?',
    $n($c['followerCount']),
    $n($c['awemeCount']),
    $n($c['totalFavorited']),
);
printf("  last refreshed %s\n", $c['lastRefreshedAt'] ?? '-');
