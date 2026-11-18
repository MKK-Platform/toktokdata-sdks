<?php

declare(strict_types=1);

namespace TokTokData\Examples;

/** Thrown for any 4xx/5xx answer; properties mirror the error envelope. */
final class ApiException extends \RuntimeException
{
    public readonly string $errorCode;
    public readonly ?string $requestId;
    public readonly bool $retryable;

    /**
     * @param array<string, mixed> $details the whole envelope (402 carries resetAt)
     * @param int|null $retryAfter seconds, from the Retry-After header
     */
    public function __construct(
        public readonly int $status,
        public readonly array $details,
        public readonly ?int $retryAfter,
    ) {
        parent::__construct((string) ($details['message'] ?? "HTTP $status"), $status);
        $this->errorCode = (string) ($details['code'] ?? 'http_error');
        $this->requestId = isset($details['requestId']) ? (string) $details['requestId'] : null;
        $this->retryable = (bool) ($details['retryable'] ?? false);
    }
}

/** Thrown when a 202 "still collecting" answer did not turn into data within 180 s. */
final class PendingTimeoutException extends \RuntimeException
{
}

/** A minimal toktokdata.live client: auth, query encoding, 202 waits, retries and the error envelope. */
final class Client
{
    private const MAX_RETRIES = 2;
    private const PENDING_DEADLINE_SECONDS = 180;

    private string $baseUrl;
    private string $apiKey;

    public function __construct()
    {
        $key = getenv('TOKTOKDATA_API_KEY');
        if ($key === false || $key === '') {
            throw new \RuntimeException('Set TOKTOKDATA_API_KEY (dashboard Settings -> API keys at https://toktokdata.live).');
        }
        $this->apiKey = $key;
        $this->baseUrl = rtrim(getenv('TOKTOKDATA_BASE_URL') ?: 'https://api.toktokdata.live', '/');
    }

    /**
     * GET a JSON resource. Waits through 202, retries 429 and retryable 5xx.
     *
     * @param array<string, scalar|null> $query
     * @return array<string, mixed>|null
     */
    public function get(string $path, array $query = []): ?array
    {
        $query = array_map(
            static fn ($v) => is_bool($v) ? ($v ? 'true' : 'false') : $v,
            array_filter($query, static fn ($v) => $v !== null),
        );
        $url = $this->baseUrl . $path . ($query ? '?' . http_build_query($query, '', '&', PHP_QUERY_RFC3986) : '');
        $deadline = time() + self::PENDING_DEADLINE_SECONDS;

        for ($attempt = 0; ; ) {
            [$status, $headers, $body] = $this->request($url);
            $retryAfter = isset($headers['retry-after']) && is_numeric($headers['retry-after']) ? (int) $headers['retry-after'] : null;

            if ($status === 202) {
                // Still collecting: wait retryAfterSeconds (or Retry-After), clamped to 1..120 s, and ask again.
                $pending = json_decode($body, true) ?: [];
                $wait = max(1, min(120, (int) ($pending['retryAfterSeconds'] ?? $retryAfter ?? 5)));
                if (time() + $wait > $deadline) {
                    throw new PendingTimeoutException('Still collecting after 180 s (job ' . ($pending['jobId'] ?? '?') . ')');
                }
                fwrite(STDERR, sprintf("  202 %s, asking again in %d s\n", $pending['status'] ?? 'resolving', $wait));
                sleep($wait);
                continue;
            }
            if ($status >= 200 && $status < 300) {
                return $body === '' ? null : json_decode($body, true, 512, JSON_THROW_ON_ERROR);
            }

            $envelope = json_decode($body, true);
            if (!is_array($envelope) || !isset($envelope['code'])) {
                $envelope = ['code' => 'http_error', 'message' => substr($body, 0, 200)];
            }
            $error = new ApiException($status, $envelope, $retryAfter);
            $retry = ($status === 429 && $error->errorCode !== 'daily_quota_exceeded')
                || ($status >= 500 && (!array_key_exists('retryable', $envelope) || $error->retryable));
            if (!$retry || $attempt >= self::MAX_RETRIES) {
                throw $error;
            }
            usleep($retryAfter !== null ? min($retryAfter, 60) * 1_000_000 : (int) ((0.5 * 2 ** $attempt) * 1_000_000) + random_int(0, 250_000));
            $attempt++;
        }
    }

    /** @return array{0: int, 1: array<string, string>, 2: string} */
    private function request(string $url): array
    {
        $headers = [];
        $ch = curl_init($url);
        curl_setopt_array($ch, [
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_TIMEOUT => 30,
            CURLOPT_HTTPHEADER => [
                'Authorization: Bearer ' . $this->apiKey,
                'Accept: application/json',
                'User-Agent: toktokdata-php-example/0.1.0',
            ],
            CURLOPT_HEADERFUNCTION => static function ($ch, string $line) use (&$headers): int {
                $parts = explode(':', $line, 2);
                if (count($parts) === 2) {
                    $headers[strtolower(trim($parts[0]))] = trim($parts[1]);
                }
                return strlen($line);
            },
        ]);
        $body = curl_exec($ch);
        if ($body === false) {
            throw new \RuntimeException('Network error: ' . curl_error($ch));
        }
        return [(int) curl_getinfo($ch, CURLINFO_RESPONSE_CODE), $headers, (string) $body];
    }
}

/**
 * Verifies an alert webhook: x-toktokdata-signature must be
 * "sha256=" . hex(HMAC-SHA256(secret, "<x-toktokdata-timestamp>.<raw body>")) and the timestamp
 * within the tolerance. Returns the decoded payload, or throws \UnexpectedValueException.
 *
 * @return array<string, mixed>
 */
function verifyWebhook(string $rawBody, ?string $timestamp, ?string $signature, string $secret, int $toleranceSeconds = 300): array
{
    if ($timestamp === null || $signature === null) {
        throw new \UnexpectedValueException('missing signature headers');
    }
    $timestamp = trim($timestamp);
    if (!ctype_digit($timestamp)) {
        throw new \UnexpectedValueException('bad timestamp');
    }
    if (abs(time() - (int) $timestamp) > $toleranceSeconds) {
        throw new \UnexpectedValueException('timestamp outside the tolerance');
    }
    $expected = 'sha256=' . hash_hmac('sha256', $timestamp . '.' . $rawBody, $secret);
    // hash_equals is constant-time.
    if (!hash_equals($expected, trim($signature))) {
        throw new \UnexpectedValueException('signature mismatch');
    }
    $payload = json_decode($rawBody, true);
    if (!is_array($payload)) {
        throw new \UnexpectedValueException('body is not JSON');
    }
    return $payload;
}

/** Prints an API error the way a user can act on it and exits non-zero. */
function exitWithError(\Throwable $e): never
{
    if ($e instanceof ApiException) {
        fwrite(STDERR, sprintf("HTTP %d %s: %s (requestId %s)\n", $e->status, $e->errorCode, $e->getMessage(), $e->requestId ?? '-'));
        match ($e->status) {
            402 => fwrite(STDERR, 'Credits are spent; they reset at ' . ($e->details['resetAt'] ?? '?') . ".\n"),
            429 => fwrite(STDERR, 'Rate limited; retry after ' . ($e->retryAfter ?? 'a few') . " s.\n"),
            403 => fwrite(STDERR, "The key lacks the scope (insufficient_scope) or the plan lacks the feature (module_not_entitled).\n"),
            default => null,
        };
    } elseif ($e instanceof PendingTimeoutException) {
        fwrite(STDERR, $e->getMessage() . "; try again in a minute.\n");
    } else {
        fwrite(STDERR, $e->getMessage() . "\n");
    }
    exit(1);
}
