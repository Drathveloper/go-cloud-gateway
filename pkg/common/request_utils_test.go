package common_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/common"
)

type DummyReadCloser struct {
	ReadErr  error
	CloseErr error
}

func (d *DummyReadCloser) Read(_ []byte) (n int, err error) {
	return 1, d.ReadErr
}

func (d *DummyReadCloser) Close() error {
	return d.CloseErr
}

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
		{
			name:        "read when reader failed should return expected error",
			readCloser:  &DummyReadCloser{ReadErr: errors.New("read failed")},
			expected:    nil,
			expectedErr: errors.New("read failed"),
		},
		{
			name:        "read when close failed should return expected error",
			readCloser:  &DummyReadCloser{ReadErr: io.EOF, CloseErr: errors.New("close failed")},
			expected:    nil,
			expectedErr: errors.New("close failed"),
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
