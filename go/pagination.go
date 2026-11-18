package toktokdata

import "context"

// PageIterator walks a cursor-paged list one page at a time. It is returned
// by the Iter methods (Live.SessionsIter, Creators.FollowersIter, ...):
//
//	it := c.Live.SessionsIter(ctx, &toktokdata.LiveSessionsParams{Limit: 50})
//	for it.Next() {
//		page := it.Page()
//		for _, item := range page.Items() {
//			_ = item
//		}
//	}
//	if err := it.Err(); err != nil {
//		// handle err
//	}
//
// Each page is a whole *Response, so the iterator works for every paged route
// whatever the member holding the items (entries, items, sessions, comments).
// Iteration stops when data.nextCursor is null, when the cursor repeats, or
// at the first error. A PageIterator is not safe for concurrent use.
type PageIterator struct {
	ctx    context.Context
	fetch  func(ctx context.Context, cursor string) (*Response, error)
	cursor string
	page   *Response
	err    error
	done   bool
}

func newPageIterator(ctx context.Context, cursor string, fetch func(ctx context.Context, cursor string) (*Response, error)) *PageIterator {
	if ctx == nil {
		ctx = context.Background()
	}
	return &PageIterator{ctx: ctx, fetch: fetch, cursor: cursor}
}

// Next fetches the next page and reports whether there is one. After it
// returns false, check Err.
func (it *PageIterator) Next() bool {
	if it.done || it.err != nil {
		return false
	}
	page, err := it.fetch(it.ctx, it.cursor)
	if err != nil {
		it.err = err
		it.page = nil
		return false
	}
	it.page = page
	next := page.NextCursor()
	if next == "" || next == it.cursor {
		it.done = true
	} else {
		it.cursor = next
	}
	return true
}

// Page returns the page fetched by the last successful call to Next.
func (it *PageIterator) Page() *Response {
	return it.page
}

// Err returns the error that stopped the iteration, or nil.
func (it *PageIterator) Err() error {
	return it.err
}

// Cursor returns the cursor the next call to Next will send ("" before the
// first page when no starting cursor was given). Save it to resume later.
func (it *PageIterator) Cursor() string {
	return it.cursor
}
