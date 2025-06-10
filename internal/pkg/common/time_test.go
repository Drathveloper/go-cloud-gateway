package common_test

import (
	"testing"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
)

func TestRealTime_Now(t *testing.T) {
	realTime := common.RealTime{}
	if realTime.Now().Unix() == 0 {
		t.Fatal("now is 0")
	}
}
