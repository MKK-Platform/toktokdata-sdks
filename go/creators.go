package toktokdata

import (
	"context"
	"net/url"
)

// CreatorsService wraps the /v1/creators endpoints. uid is a creator's
// handle ("linh.live"; a leading @ is dropped) or numeric TikTok user id.
type CreatorsService struct {
	c *Client
}

// CreatorsSearchParams are the optional parameters of CreatorsService.Search.
// Zero values are omitted.
type CreatorsSearchParams struct {
	// Q is the search text (handle or display name).
	Q string

	// Limit is the maximum number of results, 1-100 (API default 20).
	Limit int
}

// Search finds creators (GET /v1/creators).
func (s *CreatorsService) Search(ctx context.Context, params *CreatorsSearchParams) (*Response, error) {
	q := url.Values{}
	if params != nil {
		setString(q, "q", params.Q)
		setInt(q, "limit", params.Limit)
	}
	return s.c.get(ctx, "/v1/creators", q)
}

// CreatorGetParams are the optional parameters of CreatorsService.Get.
type CreatorGetParams struct {
	// Wait controls the 202 handling. Nil or true waits until the profile is
	// collected; false returns the 202 body at once.
	Wait *bool
}

// Get returns a creator profile (GET /v1/creators/{uid}). When the profile is
// being collected the API answers 202; Get waits for it unless
// params.Wait is false.
func (s *CreatorsService) Get(ctx context.Context, uid string, params *CreatorGetParams) (*Response, error) {
	id, err := creatorSegment(uid)
	if err != nil {
		return nil, err
	}
	var wait *bool
	if params != nil {
		wait = params.Wait
	}
	return s.c.getPending(ctx, "/v1/creators/"+id, nil, wait)
}

// Analysis analyses a creator's videos (GET /v1/creators/{uid}/analysis).
func (s *CreatorsService) Analysis(ctx context.Context, uid string) (*Response, error) {
	id, err := creatorSegment(uid)
	if err != nil {
		return nil, err
	}
	return s.c.get(ctx, "/v1/creators/"+id+"/analysis", nil)
}

// CreatorChangesParams are the optional parameters of
// CreatorsService.Changes. Zero values are omitted.
type CreatorChangesParams struct {
	// Limit is the maximum number of changes, 1-200 (API default 50).
	Limit int
}

// Changes lists a creator's profile changes (GET /v1/creators/{uid}/changes).
func (s *CreatorsService) Changes(ctx context.Context, uid string, params *CreatorChangesParams) (*Response, error) {
	id, err := creatorSegment(uid)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if params != nil {
		setInt(q, "limit", params.Limit)
	}
	return s.c.get(ctx, "/v1/creators/"+id+"/changes", q)
}

// CreatorFollowParams are the optional parameters of
// CreatorsService.Followers and CreatorsService.Following. Zero values are
// omitted.
type CreatorFollowParams struct {
	// Cursor is the nextCursor of the previous page.
	Cursor string

	// Limit is the page size.
	Limit int

	// Wait controls the 202 handling. Nil or true waits while the list is
	// collected; false returns the 202 body at once.
	Wait *bool
}

// Followers lists a creator's followers (GET /v1/creators/{uid}/followers,
// cursor-paged, 202 aware).
func (s *CreatorsService) Followers(ctx context.Context, uid string, params *CreatorFollowParams) (*Response, error) {
	return s.follow(ctx, uid, "followers", params)
}

// FollowersIter iterates over the pages of Followers, starting at
// params.Cursor.
func (s *CreatorsService) FollowersIter(ctx context.Context, uid string, params *CreatorFollowParams) *PageIterator {
	return s.followIter(ctx, uid, "followers", params)
}

// Following lists the accounts a creator follows
// (GET /v1/creators/{uid}/following, cursor-paged, 202 aware).
func (s *CreatorsService) Following(ctx context.Context, uid string, params *CreatorFollowParams) (*Response, error) {
	return s.follow(ctx, uid, "following", params)
}

// FollowingIter iterates over the pages of Following, starting at
// params.Cursor.
func (s *CreatorsService) FollowingIter(ctx context.Context, uid string, params *CreatorFollowParams) *PageIterator {
	return s.followIter(ctx, uid, "following", params)
}

func (s *CreatorsService) follow(ctx context.Context, uid, direction string, params *CreatorFollowParams) (*Response, error) {
	id, err := creatorSegment(uid)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	var wait *bool
	if params != nil {
		setString(q, "cursor", params.Cursor)
		setInt(q, "limit", params.Limit)
		wait = params.Wait
	}
	return s.c.getPending(ctx, "/v1/creators/"+id+"/"+direction, q, wait)
}

func (s *CreatorsService) followIter(ctx context.Context, uid, direction string, params *CreatorFollowParams) *PageIterator {
	var p CreatorFollowParams
	if params != nil {
		p = *params
	}
	return newPageIterator(ctx, p.Cursor, func(ctx context.Context, cursor string) (*Response, error) {
		p.Cursor = cursor
		return s.follow(ctx, uid, direction, &p)
	})
}

// CreatorGiftersParams are the optional parameters of
// CreatorsService.Gifters. Zero values are omitted.
type CreatorGiftersParams struct {
	// Window is "7d" (API default) or "30d".
	Window string

	// Limit is the maximum number of gifters, up to 100 (API default 20).
	Limit int
}

// Gifters returns a creator's top gifters (GET /v1/creators/{uid}/gifters).
func (s *CreatorsService) Gifters(ctx context.Context, uid string, params *CreatorGiftersParams) (*Response, error) {
	id, err := creatorSegment(uid)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if params != nil {
		setString(q, "window", params.Window)
		setInt(q, "limit", params.Limit)
	}
	return s.c.get(ctx, "/v1/creators/"+id+"/gifters", q)
}

// Insights returns a creator's insights (GET /v1/creators/{uid}/insights).
func (s *CreatorsService) Insights(ctx context.Context, uid string) (*Response, error) {
	id, err := creatorSegment(uid)
	if err != nil {
		return nil, err
	}
	return s.c.get(ctx, "/v1/creators/"+id+"/insights", nil)
}

// Videos lists a creator's videos (GET /v1/creators/{uid}/videos).
func (s *CreatorsService) Videos(ctx context.Context, uid string) (*Response, error) {
	id, err := creatorSegment(uid)
	if err != nil {
		return nil, err
	}
	return s.c.get(ctx, "/v1/creators/"+id+"/videos", nil)
}
