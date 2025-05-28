package common_test

import (
	"bytes"
	"errors"
	"gateway/pkg/common"
	"io"
	"testing"
)

func TestReadBody(t *testing.T) {
	tests := []struct {
		name        string
		readCloser  io.ReadCloser
		expected    []byte
		expectedErr error
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := common.ReadBody(tt.readCloser)
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if string(content) != string(tt.expected) {
				t.Errorf("expected %s actual %s", string(tt.expected), string(content))
			}
		})
	}
}
