<?php
// Receives toktokdata.live alert webhooks and verifies their signature.
// Run: TOKTOKDATA_WEBHOOK_SECRET=... php -S localhost:8080 webhook.php
// (or deploy this file behind any PHP web server). No API key needed here.

declare(strict_types=1);

require __DIR__ . '/TokTokData.php';

use function TokTokData\Examples\verifyWebhook;

$log = static fn (string $line) => file_put_contents('php://stderr', $line . "\n");

$secret = getenv('TOKTOKDATA_WEBHOOK_SECRET');
if ($secret === false || $secret === '') {
    $log('Set TOKTOKDATA_WEBHOOK_SECRET to the signing secret shown when you created the webhook channel.');
    http_response_code(500);
    exit;
}
if ($_SERVER['REQUEST_METHOD'] !== 'POST') {
    http_response_code(405);
    exit;
}

// Verify the exact bytes that were signed: read the raw body, never a re-encoded one.
$raw = (string) file_get_contents('php://input');
try {
    $payload = verifyWebhook(
        $raw,
        $_SERVER['HTTP_X_TOKTOKDATA_TIMESTAMP'] ?? null,
        $_SERVER['HTTP_X_TOKTOKDATA_SIGNATURE'] ?? null,
        $secret,
    );
} catch (UnexpectedValueException $e) {
    $log('rejected webhook: ' . $e->getMessage());
    http_response_code(401);
    exit;
}

$log(sprintf(
    'alert %s from rule "%s" at %s',
    $payload['alertId'] ?? '?',
    $payload['rule']['name'] ?? '?',
    $payload['occurredAt'] ?? $payload['deliveredAt'] ?? '?',
));
$log(json_encode($payload['event'] ?? null, JSON_PRETTY_PRINT | JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES));
// Answer 2xx quickly and do slow work after responding, so the delivery does not time out.
http_response_code(204);
