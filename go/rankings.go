package toktokdata

import (
	"context"
	"net/url"
)

// RankingsService wraps the /v1/rankings endpoints. Board keys: daily,
// hourly, popular_live, gaming_daily, gaming_weekly, shopping_daily,
// shopping_weekly (needs shop:read), or a league board league_a1 ...
// league_d5.
type RankingsService struct {
	c *Client
}

// RankingsOfficialParams are the optional parameters of
// RankingsService.Official. Zero values are omitted.
type RankingsOfficialParams struct {
	// Region is an ISO 3166-1 alpha-2 code (API default VN).
	Region string

	// Period is "current" (the running period, default) or "history" (the
	// last completed one).
	Period string

	// Limit is the maximum number of entries, 1-200 (API default 200).
	Limit int

	// LiveOnly keeps only creators live right now. Nil omits it.
	LiveOnly *bool

	// LeagueTier filters by league tier, A1 ... D5.
	LeagueTier string
}

// Official returns an official TikTok ranking board
// (GET /v1/rankings/official). board is required.
func (s *RankingsService) Official(ctx context.Context, board string, params *RankingsOfficialParams) (*Response, error) {
	if err := required("board", board); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("board", board)
	if params != nil {
		setString(q, "region", params.Region)
		setString(q, "period", params.Period)
		setInt(q, "limit", params.Limit)
		setBool(q, "liveOnly", params.LiveOnly)
		setString(q, "leagueTier", params.LeagueTier)
	}
	return s.c.get(ctx, "/v1/rankings/official", q)
}

// Regions lists the regions that have ranking boards (GET /v1/rankings/regions).
func (s *RankingsService) Regions(ctx context.Context) (*Response, error) {
	return s.c.get(ctx, "/v1/rankings/regions", nil)
}

// RankingsHistoryParams are the optional parameters of
// RankingsService.History. Zero values are omitted.
type RankingsHistoryParams struct {
	// Region is an ISO 3166-1 alpha-2 code (API default VN).
	Region string

	// Cursor is the nextCursor of the previous page.
	Cursor string

	// Limit is the number of periods per page, 1-24 (API default 24).
	Limit int

	// LeagueTier filters by league tier, A1 ... D5.
	LeagueTier string
}

// History returns past periods of a board, newest first
// (GET /v1/rankings/history, cursor-paged). board is required.
func (s *RankingsService) History(ctx context.Context, board string, params *RankingsHistoryParams) (*Response, error) {
	if err := required("board", board); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("board", board)
	if params != nil {
		setString(q, "region", params.Region)
		setString(q, "cursor", params.Cursor)
		setInt(q, "limit", params.Limit)
		setString(q, "leagueTier", params.LeagueTier)
	}
	return s.c.get(ctx, "/v1/rankings/history", q)
}

// HistoryIter iterates over the pages of History, starting at params.Cursor.
func (s *RankingsService) HistoryIter(ctx context.Context, board string, params *RankingsHistoryParams) *PageIterator {
	var p RankingsHistoryParams
	if params != nil {
		p = *params
	}
	return newPageIterator(ctx, p.Cursor, func(ctx context.Context, cursor string) (*Response, error) {
		p.Cursor = cursor
		return s.History(ctx, board, &p)
	})
}

// RankingsDayBarsParams are the optional parameters of
// RankingsService.DayBars. Zero values are omitted.
type RankingsDayBarsParams struct {
	// Region is an ISO 3166-1 alpha-2 code (API default VN).
	Region string

	// Board is "daily" (API default) or a league board.
	Board string

	// Days is the number of days, up to 90 (API default 30).
	Days int
}

// DayBars returns a board's daily bar over the last days
// (GET /v1/rankings/day-bars).
func (s *RankingsService) DayBars(ctx context.Context, params *RankingsDayBarsParams) (*Response, error) {
	q := url.Values{}
	if params != nil {
		setString(q, "region", params.Region)
		setString(q, "board", params.Board)
		setInt(q, "days", params.Days)
	}
	return s.c.get(ctx, "/v1/rankings/day-bars", q)
}

// Masked returns the LIVE totals of a masked board row, by the ref the board
// gave it (GET /v1/rankings/masked/{ref}).
func (s *RankingsService) Masked(ctx context.Context, ref string) (*Response, error) {
	if err := required("ref", ref); err != nil {
		return nil, err
	}
	return s.c.get(ctx, "/v1/rankings/masked/"+segment(ref), nil)
}
