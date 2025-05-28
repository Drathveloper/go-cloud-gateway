package gateway_test

import (
	"errors"
	"io"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

type DummyFilter struct {
	ID             string
	PreProcessErr  error
	PostProcessErr error
}

func (d *DummyFilter) PreProcess(_ *gateway.Context) error {
	return d.PreProcessErr
}

func (d *DummyFilter) PostProcess(_ *gateway.Context) error {
	return d.PostProcessErr
}

func (d *DummyFilter) Name() string {
	return d.ID
}

func TestFilters_PreProcessAll(t *testing.T) {
	tests := []struct {
		name           string
		filters        []gateway.Filter
		expectedErr    error
		expectedErrMsg string
	}{
		{
			name: "pre process should succeed when all filters succeed",
			filters: []gateway.Filter{
				&DummyFilter{"DF1", nil, nil},
				&DummyFilter{"DF2", nil, nil},
			},
			expectedErr:    nil,
			expectedErrMsg: "",
		},
		{
			name: "pre process should fail when first filter fail",
			filters: []gateway.Filter{
				&DummyFilter{"DF1", io.EOF, nil},
				&DummyFilter{"DF2", nil, nil},
			},
			expectedErr:    io.EOF,
			expectedErrMsg: "pre-process filters failed with filter DF1: EOF",
		},
		{
			name: "pre process should fail when last filter fail",
			filters: []gateway.Filter{
				&DummyFilter{"DF1", nil, nil},
				&DummyFilter{"DF2", nil, nil},
				&DummyFilter{"DF3", io.EOF, nil},
			},
			expectedErr:    io.EOF,
			expectedErrMsg: "pre-process filters failed with filter DF3: EOF",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gateway.NewGatewayContext(&gateway.Route{}, nil, nil)
			f := gateway.Filters(tt.filters)

			err := f.PreProcessAll(ctx)

			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if tt.expectedErr != nil && tt.expectedErrMsg != err.Error() {
				t.Errorf("expected err message %s actual %s", tt.expectedErr, err)
			}
		})
	}
}

func TestFilters_PostProcessAll(t *testing.T) {
	tests := []struct {
		name           string
		filters        []gateway.Filter
		expectedErr    error
		expectedErrMsg string
	}{
		{
			name: "post process should succeed when all filters succeed",
			filters: []gateway.Filter{
				&DummyFilter{"DF1", nil, nil},
				&DummyFilter{"DF2", nil, nil},
			},
			expectedErr:    nil,
			expectedErrMsg: "",
		},
		{
			name: "post process should fail when last filter fail",
			filters: []gateway.Filter{
				&DummyFilter{"DF1", nil, nil},
				&DummyFilter{"DF2", nil, io.EOF},
			},
			expectedErr:    io.EOF,
			expectedErrMsg: "post-process filters failed with filter DF2: EOF",
		},
		{
			name: "post process should fail when first filter fail",
			filters: []gateway.Filter{
				&DummyFilter{"DF1", nil, io.EOF},
				&DummyFilter{"DF2", nil, nil},
				&DummyFilter{"DF3", nil, nil},
			},
			expectedErr:    io.EOF,
			expectedErrMsg: "post-process filters failed with filter DF1: EOF",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gateway.NewGatewayContext(&gateway.Route{}, nil, nil)
			f := gateway.Filters(tt.filters)

			err := f.PostProcessAll(ctx)

			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if tt.expectedErr != nil && tt.expectedErrMsg != err.Error() {
				t.Errorf("expected err message %s actual %s", tt.expectedErr, err)
			}
		})
	}
}
