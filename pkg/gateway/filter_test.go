package gateway_test

import (
	"errors"
	"io"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

type DummyFilter struct {
	PreProcessErr  error
	PostProcessErr error
	ID             string
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
		expectedErr    error
		name           string
		expectedErrMsg string
		filters        []gateway.Filter
	}{
		{
			name: "pre process should succeed when all filters succeed",
			filters: []gateway.Filter{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "DF1",
				},
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "DF2",
				},
			},
			expectedErr:    nil,
			expectedErrMsg: "",
		},
		{
			name: "pre process should fail when first filter fail",
			filters: []gateway.Filter{
				&DummyFilter{
					PreProcessErr:  io.EOF,
					PostProcessErr: nil,
					ID:             "DF1",
				},
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "DF2",
				},
			},
			expectedErr:    io.EOF,
			expectedErrMsg: "pre-process filters failed with filter DF1: EOF",
		},
		{
			name: "pre process should fail when last filter fail",
			filters: []gateway.Filter{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "DF1",
				},
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "DF2",
				},
				&DummyFilter{
					PreProcessErr:  io.EOF,
					PostProcessErr: nil,
					ID:             "DF3",
				},
			},
			expectedErr:    io.EOF,
			expectedErrMsg: "pre-process filters failed with filter DF3: EOF",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gateway.NewGatewayContext(&gateway.Route{}, nil)
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
		expectedErr    error
		name           string
		expectedErrMsg string
		filters        []gateway.Filter
	}{
		{
			name: "post process should succeed when all filters succeed",
			filters: []gateway.Filter{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "DF1",
				},
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "DF2",
				},
			},
			expectedErr:    nil,
			expectedErrMsg: "",
		},
		{
			name: "post process should fail when last filter fail",
			filters: []gateway.Filter{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "DF1",
				},
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: io.EOF,
					ID:             "DF2",
				},
			},
			expectedErr:    io.EOF,
			expectedErrMsg: "post-process filters failed with filter DF2: EOF",
		},
		{
			name: "post process should fail when first filter fail",
			filters: []gateway.Filter{
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: io.EOF,
					ID:             "DF1",
				},
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "DF2",
				},
				&DummyFilter{
					PreProcessErr:  nil,
					PostProcessErr: nil,
					ID:             "DF3",
				},
			},
			expectedErr:    io.EOF,
			expectedErrMsg: "post-process filters failed with filter DF1: EOF",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gateway.NewGatewayContext(&gateway.Route{}, nil)
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
