"""Shared error printing for the examples."""

import sys
from typing import NoReturn

from toktokdata import PendingTimeoutError, TokTokDataError


def exit_with_error(err: Exception) -> NoReturn:
    """Print an API error the way a user can act on it, then exit non-zero."""
    if isinstance(err, PendingTimeoutError):
        print("TikTok data is still being collected; try again in a minute.", file=sys.stderr)
    elif isinstance(err, TokTokDataError):
        # The SDK already retried 429 and retryable 5xx before raising.
        print(
            f"HTTP {err.status} {err.code}: {err.message} (requestId {err.request_id or '-'})",
            file=sys.stderr,
        )
        if err.status == 402:
            reset_at = (err.details or {}).get("resetAt")
            print(f"Credits are spent{f'; they reset at {reset_at}' if reset_at else ''}.", file=sys.stderr)
        elif err.status == 429 and err.retry_after:
            print(f"Rate limited; retry after {err.retry_after} s.", file=sys.stderr)
        elif err.status == 403:
            print(
                "The key lacks the scope (insufficient_scope) "
                "or the plan lacks the feature (module_not_entitled).",
                file=sys.stderr,
            )
    else:
        raise err
    sys.exit(1)
