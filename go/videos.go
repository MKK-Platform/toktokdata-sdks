package toktokdata

import (
	"context"
	"net/url"
)

// VideosService wraps the /v1/videos endpoints.
type VideosService struct {
	c *Client
}

// Get returns a video by its TikTok video id (GET /v1/videos/{video_id}).
func (s *VideosService) Get(ctx context.Context, videoID string) (*Response, error) {
	if err := required("videoID", videoID); err != nil {
		return nil, err
	}
	return s.c.get(ctx, "/v1/videos/"+segment(videoID), nil)
}

// VideoCommentsParams are the optional parameters of VideosService.Comments.
// Zero values are omitted.
type VideoCommentsParams struct {
	// ParentCommentID returns the replies of this comment instead of the
	// top-level comments.
	ParentCommentID string

	// Cursor is the nextCursor of the previous page.
	Cursor string

	// Limit is the number of comments per page.
	Limit int

	// Wait controls the 202 handling. Nil or true waits while the comments
	// are collected; false returns the 202 body at once.
	Wait *bool
}

// Comments lists the comments of a video (GET /v1/videos/{video_id}/comments,
// cursor-paged, 202 aware).
func (s *VideosService) Comments(ctx context.Context, videoID string, params *VideoCommentsParams) (*Response, error) {
	if err := required("videoID", videoID); err != nil {
		return nil, err
	}
	q := url.Values{}
	var wait *bool
	if params != nil {
		setString(q, "parentCommentId", params.ParentCommentID)
		setString(q, "cursor", params.Cursor)
		setInt(q, "limit", params.Limit)
		wait = params.Wait
	}
	return s.c.getPending(ctx, "/v1/videos/"+segment(videoID)+"/comments", q, wait)
}

// CommentsIter iterates over the pages of Comments, starting at
// params.Cursor.
func (s *VideosService) CommentsIter(ctx context.Context, videoID string, params *VideoCommentsParams) *PageIterator {
	var p VideoCommentsParams
	if params != nil {
		p = *params
	}
	return newPageIterator(ctx, p.Cursor, func(ctx context.Context, cursor string) (*Response, error) {
		p.Cursor = cursor
		return s.Comments(ctx, videoID, &p)
	})
}
