package api

import (
	"reflect"
	"testing"
	"time"
)

func TestTruncateToEnd(t *testing.T) {
	api := BaseApi{}
	now := time.Now()
	truncatedToHour := api.TruncateToEnd(now, time.Hour)
	truncatedToMinute := api.TruncateToEnd(now, time.Minute)
	truncatedToSecond := api.TruncateToEnd(now, time.Second)
	truncatedToMillisecond := api.TruncateToEnd(now, time.Millisecond)
	truncatedToMicrosecond := api.TruncateToEnd(now, time.Microsecond)
	truncatedToNanosecond := api.TruncateToEnd(now, time.Nanosecond)

	tests := []struct {
		name string
		got  int
		want int
	}{
		{"Truncated to hour - year", truncatedToHour.Year(), now.Year()},
		{"Truncated to hour - month", int(truncatedToHour.Month()), int(now.Month())},
		{"Truncated to hour - day", truncatedToHour.Day(), now.Day()},
		{"Truncated to hour - hours", truncatedToHour.Hour(), 23},
		{"Truncated to hour - minutes", truncatedToHour.Minute(), 59},
		{"Truncated to hour - seconds", truncatedToHour.Second(), 59},
		{"Truncated to hour - nanoseconds", truncatedToHour.Nanosecond(), 999999999},

		{"Truncated to minute - year", truncatedToMinute.Year(), now.Year()},
		{"Truncated to minute - month", int(truncatedToMinute.Month()), int(now.Month())},
		{"Truncated to minute - day", truncatedToMinute.Day(), now.Day()},
		{"Truncated to minute - hours", truncatedToMinute.Hour(), now.Hour()},
		{"Truncated to minute - minutes", truncatedToMinute.Minute(), 59},
		{"Truncated to minute - seconds", truncatedToMinute.Second(), 59},
		{"Truncated to minute - nanoseconds", truncatedToMinute.Nanosecond(), 999999999},

		{"Truncated to second - year", truncatedToSecond.Year(), now.Year()},
		{"Truncated to second - month", int(truncatedToSecond.Month()), int(now.Month())},
		{"Truncated to second - day", truncatedToSecond.Day(), now.Day()},
		{"Truncated to second - hours", truncatedToSecond.Hour(), now.Hour()},
		{"Truncated to second - minutes", truncatedToSecond.Minute(), now.Minute()},
		{"Truncated to second - seconds", truncatedToSecond.Second(), 59},
		{"Truncated to second - nanoseconds", truncatedToSecond.Nanosecond(), 999999999},

		{"Truncated to millisecond - year", truncatedToMillisecond.Year(), now.Year()},
		{"Truncated to millisecond - month", int(truncatedToMillisecond.Month()), int(now.Month())},
		{"Truncated to millisecond - day", truncatedToMillisecond.Day(), now.Day()},
		{"Truncated to millisecond - hours", truncatedToMillisecond.Hour(), now.Hour()},
		{"Truncated to millisecond - minutes", truncatedToMillisecond.Minute(), now.Minute()},
		{"Truncated to millisecond - seconds", truncatedToMillisecond.Second(), now.Second()},
		{"Truncated to millisecond - nanoseconds", truncatedToMillisecond.Nanosecond(), 999999999},

		{"Truncated to microsecond - year", truncatedToMicrosecond.Year(), now.Year()},
		{"Truncated to microsecond - month", int(truncatedToMicrosecond.Month()), int(now.Month())},
		{"Truncated to microsecond - day", truncatedToMicrosecond.Day(), now.Day()},
		{"Truncated to microsecond - hours", truncatedToMicrosecond.Hour(), now.Hour()},
		{"Truncated to microsecond - minutes", truncatedToMicrosecond.Minute(), now.Minute()},
		{"Truncated to microsecond - seconds", truncatedToMicrosecond.Second(), now.Second()},
		{"Truncated to microsecond - milliseconds", truncatedToMicrosecond.Nanosecond() / 1000000, now.Nanosecond() / 1000000},
		{"Truncated to microsecond - microseconds", truncatedToMicrosecond.Nanosecond() - (truncatedToMicrosecond.Nanosecond() / 1000000 * 1000000), 999999},

		{"Truncated to nanosecond - year", truncatedToNanosecond.Year(), now.Year()},
		{"Truncated to nanosecond - month", int(truncatedToNanosecond.Month()), int(now.Month())},
		{"Truncated to nanosecond - day", truncatedToNanosecond.Day(), now.Day()},
		{"Truncated to nanosecond - hours", truncatedToNanosecond.Hour(), now.Hour()},
		{"Truncated to nanosecond - minutes", truncatedToNanosecond.Minute(), now.Minute()},
		{"Truncated to nanosecond - seconds", truncatedToNanosecond.Second(), now.Second()},
		{"Truncated to nanosecond - milliseconds", truncatedToNanosecond.Nanosecond() / 1000000, now.Nanosecond() / 1000000},
		{"Truncated to nanosecond - microseconds", truncatedToNanosecond.Nanosecond() / 1000, now.Nanosecond() / 1000},
		{"Truncated to nanosecond - nanoseconds", truncatedToNanosecond.Nanosecond() - (truncatedToNanosecond.Nanosecond() / 1000 * 1000), 999},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !reflect.DeepEqual(tt.got, tt.want) {
				t.Errorf("got = %v, want %v", tt.got, tt.want)
			}
		})
	}
}
