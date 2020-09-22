// Package roughtime is a soon to be deprecated wrapper for the local clock time.
package roughtime

import (
	"time"
)

// Since returns the duration since t.
func Since(t time.Time) time.Duration {
	return Now().Sub(t)
}

// Until returns the duration until t.
func Until(t time.Time) time.Duration {
	return t.Sub(Now())
}

// Now returns the current local time.
func Now() time.Time {
	return time.Now()
}
