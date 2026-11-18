package toktokdata

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// LiveService wraps the /v1/live endpoints: recorded LIVE sessions, a
// creator's LIVE performance, playback, recent chat and refreshes.
type LiveService struct {
	c *Client
}

// LiveSessionsParams are the optional parameters of LiveService.Sessions.
// Zero values are omitted.
type LiveSessionsParams struct {
	// CreatorUniqueID keeps one creator's sessions (handle; a leading @ is
	// dropped).
	CreatorUniqueID string

	// Cursor is the nextCursor of the previous page.
	Cursor string

	// Limit is the number of sessions per page, 1-100 (API default 50).
	Limit int
}

// Sessions lists recorded LIVE sessions (GET /v1/live/sessions, cursor-paged).
func (s *LiveService) Sessions(ctx context.Context, params *LiveSessionsParams) (*Response, error) {
	q := url.Values{}
	if params != nil {
		setString(q, "creatorUniqueId", trimHandle(params.CreatorUniqueID))
		setString(q, "cursor", params.Cursor)
		setInt(q, "limit", params.Limit)
	}
	return s.c.get(ctx, "/v1/live/sessions", q)
}

// SessionsIter iterates over the pages of Sessions, starting at
// params.Cursor.
func (s *LiveService) SessionsIter(ctx context.Context, params *LiveSessionsParams) *PageIterator {
	var p LiveSessionsParams
	if params != nil {
		p = *params
	}
	return newPageIterator(ctx, p.Cursor, func(ctx context.Context, cursor string) (*Response, error) {
		p.Cursor = cursor
		return s.Sessions(ctx, &p)
	})
}

// Session returns one LIVE session (GET /v1/live/sessions/{session_id}).
func (s *LiveService) Session(ctx context.Context, sessionID string) (*Response, error) {
	if err := required("sessionID", sessionID); err != nil {
		return nil, err
	}
	return s.c.get(ctx, "/v1/live/sessions/"+segment(sessionID), nil)
}

// Timeline returns a LIVE session minute by minute
// (GET /v1/live/sessions/{session_id}/timeline).
func (s *LiveService) Timeline(ctx context.Context, sessionID string) (*Response, error) {
	if err := required("sessionID", sessionID); err != nil {
		return nil, err
	}
	return s.c.get(ctx, "/v1/live/sessions/"+segment(sessionID)+"/timeline", nil)
}

// Compare compares 2 to 4 LIVE sessions (GET /v1/live/sessions/compare). The
// ids are sent comma-joined; another count fails before any request.
func (s *LiveService) Compare(ctx context.Context, ids []string) (*Response, error) {
	if len(ids) < 2 || len(ids) > 4 {
		return nil, fmt.Errorf("toktokdata: Compare needs 2 to 4 session ids, got %d", len(ids))
	}
	for _, id := range ids {
		if err := required("session id", id); err != nil {
			return nil, err
		}
	}
	q := url.Values{}
	q.Set("ids", strings.Join(ids, ","))
	return s.c.get(ctx, "/v1/live/sessions/compare", q)
}

// LivePerformanceParams are the optional parameters of
// LiveService.Performance. Zero values are omitted.
type LivePerformanceParams struct {
	// Cursor is the nextCursor of the previous page.
	Cursor string

	// Limit is the number of rows per page, 1-100 (API default 50).
	Limit int
}

// Performance returns a creator's LIVE performance
// (GET /v1/live/creators/{uid}/performance, cursor-paged).
func (s *LiveService) Performance(ctx context.Context, uid string, params *LivePerformanceParams) (*Response, error) {
	id, err := creatorSegment(uid)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if params != nil {
		setString(q, "cursor", params.Cursor)
		setInt(q, "limit", params.Limit)
	}
	return s.c.get(ctx, "/v1/live/creators/"+id+"/performance", q)
}

// PerformanceIter iterates over the pages of Performance, starting at
// params.Cursor.
func (s *LiveService) PerformanceIter(ctx context.Context, uid string, params *LivePerformanceParams) *PageIterator {
	var p LivePerformanceParams
	if params != nil {
		p = *params
	}
	return newPageIterator(ctx, p.Cursor, func(ctx context.Context, cursor string) (*Response, error) {
		p.Cursor = cursor
		return s.Performance(ctx, uid, &p)
	})
}

// Refresh queues a live-status refresh of a creator
// (POST /v1/live/creators/{uid}/refresh). The API answers 202 (queued) and
// the body is returned as is. A POST is never retried after it reached the
// server, except on 429.
func (s *LiveService) Refresh(ctx context.Context, uid string) (*Response, error) {
	id, err := creatorSegment(uid)
	if err != nil {
		return nil, err
	}
	return s.c.do(ctx, http.MethodPost, "/v1/live/creators/"+id+"/refresh", nil, nil)
}

// LivePlaybackParams are the optional parameters of LiveService.Playback.
type LivePlaybackParams struct {
	// Wait controls the 202 handling. Nil or true waits while the source is
	// resolved; false returns the 202 body at once.
	Wait *bool
}

// Playback returns a creator's LIVE video source
// (GET /v1/live/creators/{uid}/playback, 202 aware; needs live:stream).
func (s *LiveService) Playback(ctx context.Context, uid string, params *LivePlaybackParams) (*Response, error) {
	id, err := creatorSegment(uid)
	if err != nil {
		return nil, err
	}
	var wait *bool
	if params != nil {
		wait = params.Wait
	}
	return s.c.getPending(ctx, "/v1/live/creators/"+id+"/playback", nil, wait)
}

// LiveCommentsParams are the optional parameters of LiveService.Comments.
// Zero values are omitted.
type LiveCommentsParams struct {
	// After is the cursor of the previous answer: only newer comments.
	After string
}

// Comments returns a creator's recent LIVE chat
// (GET /v1/live/creators/{uid}/comments).
func (s *LiveService) Comments(ctx context.Context, uid string, params *LiveCommentsParams) (*Response, error) {
	id, err := creatorSegment(uid)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	if params != nil {
		setString(q, "after", params.After)
	}
	return s.c.get(ctx, "/v1/live/creators/"+id+"/comments", q)
}
