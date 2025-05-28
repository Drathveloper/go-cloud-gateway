package common

import "time"

type TimeProvider interface {
	Now() time.Time
}

type RealTime struct{}

func (t *RealTime) Now() time.Time {
	return time.Now()
}
