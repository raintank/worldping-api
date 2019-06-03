package util

import "time"

// Since returns the number of milliseconds since t.
func Since(t time.Time) int {
	return int(time.Since(t) / time.Millisecond)
}
