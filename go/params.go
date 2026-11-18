package toktokdata

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Params structs and query encoding.
//
// Every params struct is optional (pass nil for the API defaults). String and
// int fields are sent only when they are not the zero value ("" or 0); none
// of the API's integer parameters accepts 0 as a meaningful value. Booleans
// are *bool so that false can be sent: use Bool(false), leave nil to omit.
// Booleans go on the query string as true / false.

// Bool returns a pointer to v, for optional boolean fields such as
// RankingsOfficialParams.LiveOnly.
func Bool(v bool) *bool {
	return &v
}

// String returns a pointer to v, for optional string fields such as
// AlertRuleParams.CreatorUniqueID.
func String(v string) *string {
	return &v
}

// Int returns a pointer to v, for optional integer fields such as
// AlertRuleParams.MinDiamondCount.
func Int(v int) *int {
	return &v
}

func setString(q url.Values, key, value string) {
	if value != "" {
		q.Set(key, value)
	}
}

func setInt(q url.Values, key string, value int) {
	if value != 0 {
		q.Set(key, strconv.Itoa(value))
	}
}

func setBool(q url.Values, key string, value *bool) {
	if value != nil {
		q.Set(key, strconv.FormatBool(*value))
	}
}

// required fails before any request when a path parameter or a required
// query parameter is empty.
func required(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("toktokdata: %s is required", name)
	}
	return nil
}

// segment escapes one path parameter.
func segment(value string) string {
	return url.PathEscape(value)
}

// creatorSegment escapes a creator uid (handle or numeric id) for a path,
// dropping a leading @ so "@linh.live" and "linh.live" are the same.
func creatorSegment(uid string) (string, error) {
	uid = trimHandle(uid)
	if uid == "" {
		return "", fmt.Errorf("toktokdata: uid is required")
	}
	return url.PathEscape(uid), nil
}

// trimHandle drops surrounding spaces and a leading @ from a creator handle.
func trimHandle(handle string) string {
	return strings.TrimPrefix(strings.TrimSpace(handle), "@")
}
