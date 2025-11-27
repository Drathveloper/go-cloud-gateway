package ratelimit_test

import (
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
	"github.com/drathveloper/go-cloud-gateway/pkg/ratelimit"
)

type MockTimeProvider struct {
	WantedTime time.Time
	Increment  int64
}

func (m *MockTimeProvider) Now() time.Time {
	m.WantedTime = m.WantedTime.Add(time.Duration(m.Increment) * time.Millisecond)
	return m.WantedTime
}

func TestTokenBucket_Allow(t *testing.T) {
	tests := []struct {
		timeProvider      shared.TimeProvider
		name              string
		timesBeforeCheck  int
		rate              int
		burst             int
		expectedRemaining int
		expectedAllow     bool
	}{
		{
			name: "allow should return true when bucket is not full",
			timeProvider: &MockTimeProvider{
				WantedTime: time.Now(),
				Increment:  0,
			},
			timesBeforeCheck:  0,
			rate:              1,
			burst:             2,
			expectedRemaining: 1,
			expectedAllow:     true,
		},
		{
			name: "allow should return false when bucket is full",
			timeProvider: &MockTimeProvider{
				WantedTime: time.Now(),
				Increment:  0,
			},
			timesBeforeCheck:  2,
			rate:              1,
			burst:             2,
			expectedRemaining: 0,
			expectedAllow:     false,
		},
		{
			name: "allow should return false when tokens are greater than burst",
			timeProvider: &MockTimeProvider{
				WantedTime: time.Now(),
				Increment:  500,
			},
			timesBeforeCheck:  3,
			rate:              1,
			burst:             2,
			expectedRemaining: 0,
			expectedAllow:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket := ratelimit.NewTokenBucket(tt.timeProvider, tt.rate, tt.burst)

			for range tt.timesBeforeCheck {
				_, _ = bucket.Allow()
			}

			allow, remaining := bucket.Allow()

			if tt.expectedRemaining != remaining {
				t.Errorf("expected %d actual %d", tt.expectedRemaining, remaining)
			}
			if tt.expectedAllow != allow {
				t.Errorf("expected %t actual %t", tt.expectedAllow, allow)
			}
		})
	}
}
