package toktokdata

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// Response is a successful (2xx) answer. The JSON body is kept unchanged:
// reads look like {"data": ..., "provenance": {...}}, while exports and alert
// rules are flat objects.
type Response struct {
	// StatusCode is the HTTP status: 200, 201, 202 (pending or queued) or 204.
	StatusCode int

	// Header holds the response headers.
	Header http.Header

	// Body is the raw response body (the file itself for Exports.Download).
	Body []byte

	// Data is the "data" member of the body, or nil when the body has none
	// (flat objects, 202 answers, 204, file downloads).
	Data json.RawMessage

	// Provenance is the "provenance" object of the body (how fresh and
	// complete the data is, official or derived, the time window), or nil.
	Provenance map[string]any
}

func newResponse(status int, header http.Header, body []byte) *Response {
	r := &Response{StatusCode: status, Header: header, Body: body}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return r
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &top); err != nil {
		return r
	}
	if data, ok := top["data"]; ok {
		r.Data = data
	}
	if raw, ok := top["provenance"]; ok {
		var provenance map[string]any
		if err := json.Unmarshal(raw, &provenance); err == nil {
			r.Provenance = provenance
		}
	}
	return r
}

// Decode unmarshals the "data" member into v when the body has one, else the
// whole body (flat objects such as exports and alert rules). An empty body
// (204) leaves v untouched and returns nil.
func (r *Response) Decode(v any) error {
	src := []byte(r.Data)
	if len(src) == 0 {
		src = r.Body
	}
	return decodeJSON(src, v)
}

// DecodeBody unmarshals the whole body into v, envelope included.
func (r *Response) DecodeBody(v any) error {
	return decodeJSON(r.Body, v)
}

func decodeJSON(src []byte, v any) error {
	if len(bytes.TrimSpace(src)) == 0 {
		return nil
	}
	return json.Unmarshal(src, v)
}

// IsPending reports whether the answer is a 202 (data still being collected,
// a queued refresh or a pending export).
func (r *Response) IsPending() bool {
	return r.StatusCode == http.StatusAccepted
}

// PendingInfo returns the parsed body of a 202 answer, or nil for any other
// status.
func (r *Response) PendingInfo() *PendingBody {
	if !r.IsPending() {
		return nil
	}
	var body PendingBody
	if err := json.Unmarshal(r.Body, &body); err != nil {
		return nil
	}
	return &body
}

// NextCursor returns data.nextCursor of a cursor-paged list, or "" on the
// last page (null) and for responses that are not paged.
func (r *Response) NextCursor() string {
	if len(r.Data) == 0 {
		return ""
	}
	var page struct {
		NextCursor json.RawMessage `json:"nextCursor"`
	}
	if err := json.Unmarshal(r.Data, &page); err != nil {
		return ""
	}
	raw := bytes.TrimSpace(page.NextCursor)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ""
	}
	var cursor string
	if err := json.Unmarshal(raw, &cursor); err == nil {
		return cursor
	}
	return string(raw)
}

// pageItemKeys are the members that hold the items of a paged list.
var pageItemKeys = []string{"entries", "items", "sessions", "comments"}

// Items returns the items of a list page: data itself when it is an array,
// else the first of data.entries, data.items, data.sessions or
// data.comments that is an array. It returns nil when there is none.
func (r *Response) Items() []json.RawMessage {
	if len(r.Data) == 0 {
		return nil
	}
	var list []json.RawMessage
	if err := json.Unmarshal(r.Data, &list); err == nil {
		return list
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(r.Data, &fields); err != nil {
		return nil
	}
	for _, key := range pageItemKeys {
		raw, ok := fields[key]
		if !ok {
			continue
		}
		var items []json.RawMessage
		if err := json.Unmarshal(raw, &items); err == nil {
			return items
		}
	}
	return nil
}

// PendingBody is the body of a 202 "not ready yet" answer: Status is
// "resolving", "saturated" or "collecting", JobID the collection job and
// RetryAfterSeconds when to ask again.
type PendingBody struct {
	Status            string  `json:"status"`
	JobID             string  `json:"jobId"`
	Handle            string  `json:"handle"`
	RetryAfterSeconds float64 `json:"retryAfterSeconds"`
}
