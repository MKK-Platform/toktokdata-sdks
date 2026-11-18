package toktokdata

import (
	"context"
	"net/url"
)

// AgenciesService wraps the /v1/agencies endpoints.
type AgenciesService struct {
	c *Client
}

// AgenciesListParams are the optional parameters of AgenciesService.List.
// Zero values are omitted.
type AgenciesListParams struct {
	// Q is part of the agency name, or its official handle.
	Q string

	// Region is a region bucket, for example "VN", "US+" or "MENA".
	Region string

	// PublicOnly keeps only agencies with a public profile. Nil omits it.
	PublicOnly *bool

	// Sort is "size" (API default), "revenue", "recent" or "name".
	Sort string

	// Limit is the maximum number of rows, up to 200 (API default 50).
	Limit int

	// Offset is the number of rows to skip.
	Offset int
}

// List lists TikTok LIVE agencies (GET /v1/agencies).
func (s *AgenciesService) List(ctx context.Context, params *AgenciesListParams) (*Response, error) {
	q := url.Values{}
	if params != nil {
		setString(q, "q", params.Q)
		setString(q, "region", params.Region)
		setBool(q, "publicOnly", params.PublicOnly)
		setString(q, "sort", params.Sort)
		setInt(q, "limit", params.Limit)
		setInt(q, "offset", params.Offset)
	}
	return s.c.get(ctx, "/v1/agencies", q)
}

// Get returns one agency by its TikTok agency id (GET /v1/agencies/{agency_id}).
func (s *AgenciesService) Get(ctx context.Context, agencyID string) (*Response, error) {
	if err := required("agencyID", agencyID); err != nil {
		return nil, err
	}
	return s.c.get(ctx, "/v1/agencies/"+segment(agencyID), nil)
}

// InsightsService wraps the /v1/insights endpoints.
type InsightsService struct {
	c *Client
}

// InsightsBestTimeParams are the optional parameters of
// InsightsService.BestTime. Zero values are omitted.
type InsightsBestTimeParams struct {
	// Region is an ISO 3166-1 alpha-2 code (API default VN).
	Region string
}

// BestTime returns the best time to go LIVE in a region
// (GET /v1/insights/best-time).
func (s *InsightsService) BestTime(ctx context.Context, params *InsightsBestTimeParams) (*Response, error) {
	q := url.Values{}
	if params != nil {
		setString(q, "region", params.Region)
	}
	return s.c.get(ctx, "/v1/insights/best-time", q)
}

// InsightsPresenceParams are the optional parameters of
// InsightsService.Presence. Zero values are omitted.
type InsightsPresenceParams struct {
	// Region is an ISO 3166-1 alpha-2 code (API default VN).
	Region string

	// Hours is how many hours back, 1-168 (API default 48).
	Hours int
}

// Presence returns hourly LIVE presence in a region
// (GET /v1/insights/presence).
func (s *InsightsService) Presence(ctx context.Context, params *InsightsPresenceParams) (*Response, error) {
	q := url.Values{}
	if params != nil {
		setString(q, "region", params.Region)
		setInt(q, "hours", params.Hours)
	}
	return s.c.get(ctx, "/v1/insights/presence", q)
}

// TrendsService wraps the /v1/trends endpoint.
type TrendsService struct {
	c *Client
}

// TrendsListParams are the optional parameters of TrendsService.List. Zero
// values are omitted.
type TrendsListParams struct {
	// Region is a region code (API default US).
	Region string

	// Type keeps one kind of trend: "hashtag" or "sound".
	Type string
}

// List lists the trends of a region (GET /v1/trends).
func (s *TrendsService) List(ctx context.Context, params *TrendsListParams) (*Response, error) {
	q := url.Values{}
	if params != nil {
		setString(q, "region", params.Region)
		setString(q, "type", params.Type)
	}
	return s.c.get(ctx, "/v1/trends", q)
}

// GiftsService wraps the /v1/gifts endpoint (the gift catalog).
type GiftsService struct {
	c *Client
}

// GiftsListParams are the optional parameters of GiftsService.List. Zero
// values are omitted, so MinCoins and MaxCoins cannot be sent as 0.
type GiftsListParams struct {
	// Q is part of the gift name.
	Q string

	// MinCoins is the minimum coins of one gift.
	MinCoins int

	// MaxCoins is the maximum coins of one gift.
	MaxCoins int

	// Region keeps only gifts seen in this region, for example "VN".
	Region string

	// Sort is "coins" (most first, API default), "name" or "recent".
	Sort string

	// Limit is the maximum number of rows, up to 500 (API default 100).
	Limit int
}

// List lists gifts (GET /v1/gifts).
func (s *GiftsService) List(ctx context.Context, params *GiftsListParams) (*Response, error) {
	q := url.Values{}
	if params != nil {
		setString(q, "q", params.Q)
		setInt(q, "minCoins", params.MinCoins)
		setInt(q, "maxCoins", params.MaxCoins)
		setString(q, "region", params.Region)
		setString(q, "sort", params.Sort)
		setInt(q, "limit", params.Limit)
	}
	return s.c.get(ctx, "/v1/gifts", q)
}

// GiftersService wraps the /v1/gifters endpoint.
type GiftersService struct {
	c *Client
}

// GiftersListParams are the optional parameters of GiftersService.List. Zero
// values are omitted.
type GiftersListParams struct {
	// Window is "7d" (API default) or "30d".
	Window string

	// Segment keeps one segment: "whale", "rising", "regular", "one_time" or
	// "occasional".
	Segment string

	// Region is the region of the creators gifted, for example "VN".
	Region string

	// Limit is the maximum number of rows, up to 500 (API default 50).
	Limit int
}

// List lists gifters (GET /v1/gifters).
func (s *GiftersService) List(ctx context.Context, params *GiftersListParams) (*Response, error) {
	q := url.Values{}
	if params != nil {
		setString(q, "window", params.Window)
		setString(q, "segment", params.Segment)
		setString(q, "region", params.Region)
		setInt(q, "limit", params.Limit)
	}
	return s.c.get(ctx, "/v1/gifters", q)
}

// AccountService wraps the account endpoints /v1/entitlements and /v1/usage.
type AccountService struct {
	c *Client
}

// AccountEntitlementsParams are the optional parameters of
// AccountService.Entitlements. Zero values are omitted.
type AccountEntitlementsParams struct {
	// Period is the billing period start, YYYY-MM-01 (API default: the
	// current one).
	Period string
}

// Entitlements returns the plan and its limits (GET /v1/entitlements).
func (s *AccountService) Entitlements(ctx context.Context, params *AccountEntitlementsParams) (*Response, error) {
	q := url.Values{}
	if params != nil {
		setString(q, "period", params.Period)
	}
	return s.c.get(ctx, "/v1/entitlements", q)
}

// AccountUsageParams are the optional parameters of AccountService.Usage.
// Zero values are omitted.
type AccountUsageParams struct {
	// Period is the billing period start, YYYY-MM-01 (API default: the
	// current one).
	Period string

	// Limit is the maximum number of rows.
	Limit int
}

// Usage returns credit usage (GET /v1/usage).
func (s *AccountService) Usage(ctx context.Context, params *AccountUsageParams) (*Response, error) {
	q := url.Values{}
	if params != nil {
		setString(q, "period", params.Period)
		setInt(q, "limit", params.Limit)
	}
	return s.c.get(ctx, "/v1/usage", q)
}
