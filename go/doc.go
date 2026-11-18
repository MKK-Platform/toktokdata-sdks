// Package toktokdata is the official Go client for the toktokdata.live API
// (TikTok LIVE intelligence: official rankings, creators, recorded LIVE
// sessions, alerts and exports).
//
// Create a client with NewClient. The API key comes from WithAPIKey or, when
// that option is absent, from the TOKTOKDATA_API_KEY environment variable:
//
//	c, err := toktokdata.NewClient()
//	if err != nil {
//		log.Fatal(err)
//	}
//	resp, err := c.Rankings.Official(ctx, "hourly", &toktokdata.RankingsOfficialParams{Region: "VN"})
//
// Every method takes a context.Context first and returns a *Response that
// keeps the parsed JSON body unchanged: Response.Data holds the "data" member
// of read responses, Response.Provenance the "provenance" object and
// Response.Body the raw bytes. Response.Decode unmarshals into your own types.
//
// Failed calls return an *APIError (use errors.As, or the helpers
// IsRateLimited, IsPaymentRequired, IsForbidden, IsUnauthorized and
// IsNotFound). Requests are retried on network errors (GET only), on 429
// (except daily_quota_exceeded) and on retryable 5xx answers (GET only),
// honouring Retry-After.
//
// Reads that collect fresh data (Creators.Get, Creators.Followers,
// Creators.Following, Videos.Comments, Live.Playback) wait through the API's
// 202 "not ready yet" answers by default; a wait that exceeds the client's
// pending timeout returns an error matching ErrPendingTimeout.
//
// Cursor-paged lists have an Iter variant returning a *PageIterator, and
// VerifyWebhook checks the signature of alert webhooks.
package toktokdata
