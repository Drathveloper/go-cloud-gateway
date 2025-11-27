package shared_test

import (
	"testing"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
)

func TestRealTime_Now(t *testing.T) {
	realTime := shared.RealTime{}
	if realTime.Now().Unix() == 0 {
		t.Fatal("now is 0")
	}
}
