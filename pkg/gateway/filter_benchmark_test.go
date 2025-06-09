package gateway_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

type DummyBenchFilter struct {
	ID   string
	Fail bool
}

func (f *DummyBenchFilter) Name() string {
	return "filter-" + f.ID
}

func (f *DummyBenchFilter) PreProcess(_ *gateway.Context) error {
	if f.Fail {
		return errors.New("forced error")
	}
	return nil
}

func (f *DummyBenchFilter) PostProcess(_ *gateway.Context) error {
	if f.Fail {
		return errors.New("forced error")
	}
	return nil
}

var dummyCtx = &gateway.Context{}

func BenchmarkPreProcessAll_NoError(b *testing.B) {
	filters := make(gateway.Filters, 10)
	for i := range filters {
		filters[i] = &DummyFilter{ID: strconv.Itoa(i)}
	}
	b.ResetTimer()
	for range b.N {
		_ = filters.PreProcessAll(dummyCtx)
	}
}

func BenchmarkPostProcessAll_NoError(b *testing.B) {
	filters := make(gateway.Filters, 10)
	for i := range filters {
		filters[i] = &DummyBenchFilter{ID: strconv.Itoa(i)}
	}
	b.ResetTimer()
	for range b.N {
		_ = filters.PostProcessAll(dummyCtx)
	}
}

func BenchmarkPreProcessAll_WithError(b *testing.B) {
	filters := make(gateway.Filters, 10)
	for i := range filters {
		fail := i == 5 // solo el 6° da error
		filters[i] = &DummyBenchFilter{ID: strconv.Itoa(i), Fail: fail}
	}
	b.ResetTimer()
	for range b.N {
		_ = filters.PreProcessAll(dummyCtx)
	}
}

func BenchmarkPostProcessAll_WithError(b *testing.B) {
	filters := make(gateway.Filters, 10)
	for i := range filters {
		fail := i == 5
		filters[i] = &DummyBenchFilter{ID: strconv.Itoa(i), Fail: fail}
	}
	b.ResetTimer()
	for range b.N {
		_ = filters.PostProcessAll(dummyCtx)
	}
}
