package common_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/common"
)

type DummyReadCloser struct {
	ReadErr  error
	CloseErr error
}

func (d *DummyReadCloser) Read(_ []byte) (int, error) {
	return 1, d.ReadErr
}

func (d *DummyReadCloser) Close() error {
	return d.CloseErr
}

func TestReadBody(t *testing.T) {
	tests := []struct {
		readCloser  io.ReadCloser
		expectedErr error
		name        string
		expected    []byte
	}{
		{
			name:        "read when reader is nil should return zero length byte array",
			readCloser:  nil,
			expected:    make([]byte, 0),
			expectedErr: nil,
		},
		{
			name:        "read when reader is empty should return zero length byte array",
			readCloser:  io.NopCloser(bytes.NewBuffer(make([]byte, 0))),
			expected:    make([]byte, 0),
			expectedErr: nil,
		},
		{
			name:        "read when reader has content should return content byte array",
			readCloser:  io.NopCloser(bytes.NewBuffer([]byte("{\"x\":\"y\"}"))),
			expected:    []byte("{\"x\":\"y\"}"),
			expectedErr: nil,
		},
		{
			name:        "read when reader failed should return expected error",
			readCloser:  &DummyReadCloser{ReadErr: errors.New("read failed")},
			expected:    nil,
			expectedErr: errors.New("read failed"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := common.ReadBody(tt.readCloser)
			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if string(content) != string(tt.expected) {
				t.Errorf("expected %s actual %s", string(tt.expected), string(content))
			}
		})
	}
}

func TestWriteHeader(t *testing.T) {
	tests := []struct {
		initialHeaders http.Header
		inputHeaders   http.Header
		expected       http.Header
		name           string
	}{
		{
			name:           "copy new headers",
			initialHeaders: http.Header{},
			inputHeaders:   http.Header{"X-Test": {"value1"}},
			expected:       http.Header{"X-Test": {"value1"}},
		},
		{
			name:           "overwrite existing header",
			initialHeaders: http.Header{"X-Test": {"old"}},
			inputHeaders:   http.Header{"X-Test": {"new1", "new2"}},
			expected:       http.Header{"X-Test": {"new1", "new2"}},
		},
		{
			name:           "preserve unrelated header",
			initialHeaders: http.Header{"X-Other": {"value"}},
			inputHeaders:   http.Header{"X-Test": {"a"}},
			expected:       http.Header{"X-Other": {"value"}, "X-Test": {"a"}},
		},
		{
			name:           "replace multiple headers",
			initialHeaders: http.Header{"X-A": {"1"}, "X-B": {"2"}},
			inputHeaders:   http.Header{"X-A": {"A1"}, "X-B": {"B2"}},
			expected:       http.Header{"X-A": {"A1"}, "X-B": {"B2"}},
		},
		{
			name:           "empty input should not remove existing headers",
			initialHeaders: http.Header{"X-Exists": {"keep"}},
			inputHeaders:   http.Header{},
			expected:       http.Header{"X-Exists": {"keep"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()

			// Set initial headers if any
			for k, v := range tt.initialHeaders {
				rr.Header()[k] = append([]string(nil), v...)
			}

			common.WriteHeader(rr, tt.inputHeaders)

			got := rr.Header()
			if len(got) != len(tt.expected) {
				t.Errorf("unexpected header count: got %d, want %d", len(got), len(tt.expected))
			}
			for k, v := range tt.expected {
				if gotVals, ok := got[k]; !ok {
					t.Errorf("missing header %q", k)
				} else if !reflect.DeepEqual(gotVals, v) {
					t.Errorf("header %q mismatch: got %v, want %v", k, gotVals, v)
				}
			}
		})
	}
}
