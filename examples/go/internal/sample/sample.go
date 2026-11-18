// Package sample holds the bits the example commands share.
package sample

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"

	toktokdata "github.com/MKK-Platform/toktokdata-sdks/go"
)

// Decode copies a loose value (such as the map VerifyWebhook returns) into a typed struct.
func Decode(v any, out any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// Exit prints err the way a user can act on it and exits non-zero.
func Exit(err error) {
	var apiErr *toktokdata.APIError
	var pending *toktokdata.PendingTimeoutError
	switch {
	case errors.As(err, &pending):
		fmt.Fprintln(os.Stderr, "TikTok data is still being collected; try again in a minute.")
	case errors.As(err, &apiErr):
		// The SDK already retried 429 and retryable 5xx before returning.
		fmt.Fprintf(os.Stderr, "HTTP %d %s: %s (requestId %s)\n", apiErr.Status, apiErr.Code, apiErr.Message, apiErr.RequestID)
		switch apiErr.Status {
		case 402:
			if resetAt, ok := apiErr.Details["resetAt"]; ok {
				fmt.Fprintf(os.Stderr, "Credits are spent; they reset at %v.\n", resetAt)
			}
		case 429:
			if apiErr.RetryAfter > 0 {
				fmt.Fprintf(os.Stderr, "Rate limited; retry after %s.\n", apiErr.RetryAfter)
			}
		case 403:
			fmt.Fprintln(os.Stderr, "The key lacks the scope (insufficient_scope) or the plan lacks the feature (module_not_entitled).")
		}
	default:
		fmt.Fprintln(os.Stderr, err)
	}
	os.Exit(1)
}

// Arg returns os.Args[i] or def when it is missing.
func Arg(i int, def string) string {
	if len(os.Args) > i && os.Args[i] != "" {
		return os.Args[i]
	}
	return def
}
