import { PendingTimeoutError, TokTokDataError } from '@toktokdata/sdk';

/** Print an API error the way a user can act on it, then exit non-zero. */
export function exitWithError(err: unknown): never {
  if (err instanceof PendingTimeoutError) {
    console.error('TikTok data is still being collected; try again in a minute.');
  } else if (err instanceof TokTokDataError) {
    // The SDK already retried 429 and retryable 5xx before throwing.
    console.error(`HTTP ${err.status} ${err.code}: ${err.message} (requestId ${err.requestId ?? '-'})`);
    if (err.status === 402) {
      const resetAt = (err.details as { resetAt?: string } | undefined)?.resetAt;
      console.error(`Credits are spent${resetAt ? `; they reset at ${resetAt}` : ''}.`);
    } else if (err.status === 429 && err.retryAfter) {
      console.error(`Rate limited; retry after ${err.retryAfter} s.`);
    } else if (err.status === 403) {
      console.error('The key lacks the scope (insufficient_scope) or the plan lacks the feature (module_not_entitled).');
    }
  } else {
    console.error(err);
  }
  process.exit(1);
}
