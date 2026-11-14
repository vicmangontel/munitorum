package app

import "time"

// Clock is the source of "now". Tests replace it; everything else uses SystemClock.
type Clock interface {
	Now() time.Time
}

// SystemClock reads the local time.
type SystemClock struct{}

func (SystemClock) Now() time.Time { return time.Now() }
