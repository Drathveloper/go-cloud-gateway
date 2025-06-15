package circuitbreaker_test

import (
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/circuitbreaker"
)

func TestState_String(t *testing.T) {
	tests := []struct {
		name        string
		expectedVal string
		state       circuitbreaker.State
	}{
		{
			name:        "open state should return open",
			state:       circuitbreaker.StateOpen,
			expectedVal: "open",
		},
		{
			name:        "open state should return closed",
			state:       circuitbreaker.StateClosed,
			expectedVal: "closed",
		},
		{
			name:        "open state should return half-open",
			state:       circuitbreaker.StateHalfOpen,
			expectedVal: "half-open",
		},
		{
			name:        "unknown state should return unknown",
			state:       -1,
			expectedVal: "unknown",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := tt.state.String()
			if tt.expectedVal != val {
				t.Errorf("expected %s actual %s", tt.expectedVal, val)
			}
		})
	}
}
