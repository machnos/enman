// Package buckets provides time-bucket helpers used across the forecasting,
// optimization and scheduling pipeline. A bucket is a fixed-size, half-open
// interval [Start, Start+Size).
package buckets

import (
	"time"
)

// DefaultBucketSize is the bucket size used when nothing is configured.
// It matches the typical 15-minute granularity of European day-ahead price feeds.
const DefaultBucketSize = 15 * time.Minute

// Bucket represents a single half-open time interval [Start, Start+Size).
type Bucket struct {
	Start time.Time
	Size  time.Duration
}

// End returns the (exclusive) end of the bucket.
func (b Bucket) End() time.Time {
	return b.Start.Add(b.Size)
}

// Contains reports whether t falls inside the bucket.
func (b Bucket) Contains(t time.Time) bool {
	return !t.Before(b.Start) && t.Before(b.End())
}

// Align rounds t down to the start of the bucket of the given size.
// Alignment is performed in UTC so daylight-saving transitions don't shift bucket boundaries.
func Align(t time.Time, size time.Duration) time.Time {
	if size <= 0 {
		return t
	}
	loc := t.Location()
	utc := t.UTC()
	truncated := utc.Truncate(size)
	return truncated.In(loc)
}

// Range returns aligned buckets that cover [from, till). The first bucket starts
// at Align(from, size); the last bucket starts strictly before till.
func Range(from, till time.Time, size time.Duration) []Bucket {
	if size <= 0 || !till.After(from) {
		return nil
	}
	start := Align(from, size)
	out := make([]Bucket, 0, int(till.Sub(start)/size)+1)
	for cur := start; cur.Before(till); cur = cur.Add(size) {
		out = append(out, Bucket{Start: cur, Size: size})
	}
	return out
}
