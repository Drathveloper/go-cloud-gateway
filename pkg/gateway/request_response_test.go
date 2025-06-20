package gateway_test

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

func TestNewGatewayRequest(t *testing.T) {
	tests := []struct {
		expectedErr error
		request     *http.Request
		expected    *gateway.Request
		name        string
	}{
		{
			name: "new gateway should succeed",
			request: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Header: map[string][]string{
					"h1": {"value1"},
				},
				ContentLength: int64(len(`{"p1":"v1"}`)),
				Body:          io.NopCloser(bytes.NewBuffer([]byte("{\"p1\":\"v1\"}"))),
			},
			expected: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method: http.MethodGet,
				Headers: map[string][]string{
					"h1": {"value1"},
				},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer([]byte("{\"p1\":\"v1\"}"))), int64(len(`{"p1":"v1"}`))),
			},
			expectedErr: nil,
		},
		{
			name: "new gateway should succeed when body is nil",
			request: &http.Request{
				Method: http.MethodGet,
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Header: map[string][]string{
					"h1": {"value1"},
				},
				Body: nil,
			},
			expected: &gateway.Request{
				URL: &url.URL{
					Scheme:   "https",
					Host:     "example.org",
					Path:     "/server/test",
					RawQuery: "key=value",
				},
				Method: http.MethodGet,
				Headers: map[string][]string{
					"h1": {"value1"},
				},
				BodyReader: gateway.NewReplayableBody(nil, 0),
			},
			expectedErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gwReq := gateway.NewGatewayRequest(tt.request)
			if !reflect.DeepEqual(tt.expected, gwReq) {
				t.Errorf("expected response %v actual %v", tt.expected, gwReq)
			}
		})
	}
}

func TestNewGatewayResponse(t *testing.T) {
	tests := []struct {
		expectedErr error
		response    *http.Response
		expected    *gateway.Response
		name        string
	}{
		{
			name: "new gateway should succeed",
			response: &http.Response{
				ContentLength: int64(len(`{"p1":"v1"}`)),
				StatusCode:    http.StatusOK,
				Header: map[string][]string{
					"h1": {"value1"},
				},
				Body: io.NopCloser(bytes.NewBuffer([]byte("{\"p1\":\"v1\"}"))),
			},
			expected: &gateway.Response{
				Status: http.StatusOK,
				Headers: map[string][]string{
					"h1": {"value1"},
				},
				BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer([]byte(`{"p1":"v1"}`))), int64(len(`{"p1":"v1"}`))),
			},
			expectedErr: nil,
		},
		{
			name: "new gateway should succeed when body is nil",
			response: &http.Response{
				StatusCode: http.StatusOK,
				Header: map[string][]string{
					"h1": {"value1"},
				},
				Body: nil,
			},
			expected: &gateway.Response{
				Status: http.StatusOK,
				Headers: map[string][]string{
					"h1": {"value1"},
				},
				BodyReader: gateway.NewReplayableBody(nil, 0),
			},
			expectedErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gwRes := gateway.NewGatewayResponse(tt.response)
			if !reflect.DeepEqual(tt.expected, gwRes) {
				t.Errorf("expected response %v actual %v", tt.expected, gwRes)
			}
		})
	}
}

func TestReplayableBody_Read(t *testing.T) {
	tests := []struct {
		reader        io.ReadCloser
		expectedErr   error
		name          string
		expected      []byte
		readTimes     int
		readerLen     int64
		shouldCapture bool
	}{
		{
			name:          "read replayable body one time without capture should succeed",
			readTimes:     1,
			shouldCapture: false,
			reader:        io.NopCloser(bytes.NewBuffer([]byte("{\"p1\":\"v1\"}"))),
			expected:      []byte("{\"p1\":\"v1\"}"),
			expectedErr:   nil,
		},
		{
			name:          "read replayable body many times without capture should return empty bytes slice",
			readTimes:     rand.Intn(10) + 2, //nolint:gosec
			shouldCapture: false,
			reader:        io.NopCloser(bytes.NewBuffer([]byte("{\"p1\":\"v1\"}"))),
			expected:      make([]byte, 0),
			expectedErr:   nil,
		},
		{
			name:          "read replayable body one time with capture should succeed",
			readTimes:     1,
			shouldCapture: true,
			reader:        io.NopCloser(bytes.NewBuffer([]byte("{\"p1\":\"v1\"}"))),
			expected:      []byte("{\"p1\":\"v1\"}"),
			expectedErr:   nil,
		},
		{
			name:          "read replayable body many times with capture should succeed",
			readTimes:     rand.Intn(10) + 2, //nolint:gosec
			shouldCapture: true,
			reader:        io.NopCloser(bytes.NewBuffer([]byte("{\"p1\":\"v1\"}"))),
			expected:      []byte("{\"p1\":\"v1\"}"),
			expectedErr:   nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replayableBody := gateway.NewReplayableBody(tt.reader, tt.readerLen)

			var readBytes []byte
			var err error
			if tt.shouldCapture {
				err = replayableBody.Capture()
				if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
					t.Errorf("expected err %s actual %s", tt.expectedErr, err)
				}
			}
			for range tt.readTimes {
				readBytes, err = io.ReadAll(replayableBody)
				t.Logf("read bytes %v", readBytes)
			}
			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
			if !reflect.DeepEqual(readBytes, tt.expected) {
				t.Errorf("expected body %v actual %v", tt.expected, readBytes)
			}
		})
	}
}

func TestReplayableBody_Capture(t *testing.T) {
	tests := []struct {
		reader       io.ReadCloser
		expectedErr  error
		name         string
		expected     []byte
		captureTimes int
		readerLen    int64
	}{
		{
			name:         "capture replayable body should succeed when executed one time",
			captureTimes: 1,
			reader:       io.NopCloser(bytes.NewBuffer([]byte("{\"p1\":\"v1\"}"))),
			expectedErr:  nil,
			readerLen:    int64(len(`{"p1":"v1"}`)),
		},
		{
			name:         "capture replayable body should succeed when executed multiple times",
			captureTimes: rand.Intn(10) + 2, //nolint:gosec
			reader:       io.NopCloser(bytes.NewBuffer([]byte("{\"p1\":\"v1\"}"))),
			expectedErr:  nil,
			readerLen:    int64(len(`{"p1":"v1"}`)),
		},
		{
			name:         "capture replayable body should succeed when original body is nil",
			captureTimes: rand.Intn(10) + 2, //nolint:gosec
			reader:       nil,
			expectedErr:  nil,
			readerLen:    0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replayableBody := gateway.NewReplayableBody(tt.reader, tt.readerLen)

			var err error
			for range tt.captureTimes {
				err = replayableBody.Capture()
			}

			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}

func TestReplayableBody_Close(t *testing.T) {
	tests := []struct {
		reader       io.ReadCloser
		expectedErr  error
		name         string
		expected     []byte
		captureTimes int
		readerLen    int64
	}{
		{
			name:         "close replayable body should succeed when body read zero times",
			captureTimes: 0,
			reader:       io.NopCloser(bytes.NewBuffer([]byte("{\"p1\":\"v1\"}"))),
			expectedErr:  nil,
			readerLen:    int64(len(`{"p1":"v1"}`)),
		},
		{
			name:         "close replayable body should succeed when body read one time",
			captureTimes: 1,
			reader:       io.NopCloser(bytes.NewBuffer([]byte("{\"p1\":\"v1\"}"))),
			expectedErr:  nil,
			readerLen:    int64(len(`{"p1":"v1"}`)),
		},
		{
			name:         "close replayable body should succeed when body read multiple times",
			captureTimes: rand.Intn(10) + 2, //nolint:gosec
			reader:       io.NopCloser(bytes.NewBuffer([]byte("{\"p1\":\"v1\"}"))),
			expectedErr:  nil,
			readerLen:    int64(len(`{"p1":"v1"}`)),
		},
		{
			name:         "capture replayable body should succeed when original body is nil",
			captureTimes: rand.Intn(10) + 2, //nolint:gosec
			reader:       nil,
			expectedErr:  nil,
			readerLen:    0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replayableBody := gateway.NewReplayableBody(tt.reader, tt.readerLen)

			for range tt.captureTimes {
				_ = replayableBody.Capture()
			}

			err := replayableBody.Close()
			if fmt.Sprintf("%s", err) != fmt.Sprintf("%s", tt.expectedErr) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}

func TestReplayableBody_Len(t *testing.T) {
	originalLength := int64(rand.Intn(1000000000)) //nolint:gosec

	body := gateway.NewReplayableBody(nil, originalLength)
	if body.Len() != originalLength {
		t.Errorf("expected body length %d actual %d", originalLength, body.Len())
	}
}
