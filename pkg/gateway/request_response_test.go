package gateway_test

import (
	"bytes"
	"errors"
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

type closeCountingBody struct {
	io.Reader

	closes int
}

func (c *closeCountingBody) Close() error {
	c.closes++
	return nil
}

func TestReplayableBody_Close_ClosesOriginalOnce(t *testing.T) {
	payload := []byte("payload")
	src := &closeCountingBody{Reader: bytes.NewReader(payload)}
	rb := gateway.NewReplayableBody(src, int64(len(payload)))

	if err := rb.Capture(); err != nil {
		t.Fatalf("capture failed: %v", err)
	}
	if err := rb.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if src.closes != 1 {
		t.Errorf("expected original closed once, actual %d", src.closes)
	}

	if err := rb.Close(); err != nil {
		t.Fatalf("second close failed: %v", err)
	}
	if src.closes != 1 {
		t.Errorf("expected close to be idempotent, original closed %d times", src.closes)
	}

	got, err := io.ReadAll(rb)
	if err != nil {
		t.Fatalf("read after close failed: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("expected captured body to remain replayable after close, got %q want %q", got, payload)
	}
}

func TestReplayableBody_CaptureWithLimit(t *testing.T) {
	payload := []byte("0123456789")

	t.Run("body within the limit is captured and replayable", func(t *testing.T) {
		rb := gateway.NewReplayableBody(io.NopCloser(bytes.NewReader(payload)), int64(len(payload)))

		if err := rb.CaptureWithLimit(int64(len(payload))); err != nil {
			t.Fatalf("capture failed: %v", err)
		}
		for range 2 {
			got, err := io.ReadAll(rb)
			if err != nil || !bytes.Equal(got, payload) {
				t.Fatalf("expected replayable body %q, actual %q (err %v)", payload, got, err)
			}
		}
	})

	t.Run("declared length over the limit is rejected without consuming", func(t *testing.T) {
		src := &closeCountingBody{Reader: bytes.NewReader(payload)}
		rb := gateway.NewReplayableBody(src, int64(len(payload)))

		err := rb.CaptureWithLimit(int64(len(payload)) - 1)
		if !errors.Is(err, gateway.ErrCaptureLimitExceeded) {
			t.Fatalf("expected ErrCaptureLimitExceeded, actual %v", err)
		}
		got, err := io.ReadAll(rb)
		if err != nil || !bytes.Equal(got, payload) {
			t.Errorf("expected body fully forwardable after rejection, actual %q (err %v)", got, err)
		}
	})

	t.Run("unknown length over the limit is rejected and the prefix is stitched back", func(t *testing.T) {
		rb := gateway.NewReplayableBody(io.NopCloser(bytes.NewReader(payload)), -1)

		err := rb.CaptureWithLimit(4)
		if !errors.Is(err, gateway.ErrCaptureLimitExceeded) {
			t.Fatalf("expected ErrCaptureLimitExceeded, actual %v", err)
		}
		got, readErr := io.ReadAll(rb)
		if readErr != nil || !bytes.Equal(got, payload) {
			t.Errorf("expected body fully forwardable after rejection, actual %q (err %v)", got, readErr)
		}
		if rb.Len() != -1 {
			t.Errorf("expected declared length untouched after rejection, actual %d", rb.Len())
		}
	})

	t.Run("negative limit means unlimited", func(t *testing.T) {
		rb := gateway.NewReplayableBody(io.NopCloser(bytes.NewReader(payload)), -1)

		if err := rb.CaptureWithLimit(-1); err != nil {
			t.Fatalf("capture failed: %v", err)
		}
		if rb.Len() != int64(len(payload)) {
			t.Errorf("expected length %d, actual %d", len(payload), rb.Len())
		}
	})

	t.Run("close after rejection closes the original once", func(t *testing.T) {
		src := &closeCountingBody{Reader: bytes.NewReader(payload)}
		rb := gateway.NewReplayableBody(src, -1)

		if err := rb.CaptureWithLimit(4); !errors.Is(err, gateway.ErrCaptureLimitExceeded) {
			t.Fatalf("expected ErrCaptureLimitExceeded, actual %v", err)
		}
		if err := rb.Close(); err != nil {
			t.Fatalf("close failed: %v", err)
		}
		if src.closes != 1 {
			t.Errorf("expected original closed once, actual %d", src.closes)
		}
	})
}

func TestReplayableBody_Capture_DoesNotAliasPooledBuffer(t *testing.T) {
	first := bytes.Repeat([]byte("A"), 1024)
	second := bytes.Repeat([]byte("B"), 1024)

	firstBody := gateway.NewReplayableBody(io.NopCloser(bytes.NewReader(first)), int64(len(first)))
	if err := firstBody.Capture(); err != nil {
		t.Fatalf("capture first body failed: %v", err)
	}

	// Capturing a second body reuses the pooled staging buffer that the first
	// capture just released; the first body must not observe its contents.
	secondBody := gateway.NewReplayableBody(io.NopCloser(bytes.NewReader(second)), int64(len(second)))
	if err := secondBody.Capture(); err != nil {
		t.Fatalf("capture second body failed: %v", err)
	}

	got, err := io.ReadAll(firstBody)
	if err != nil {
		t.Fatalf("read first body failed: %v", err)
	}
	if !bytes.Equal(got, first) {
		t.Errorf("first captured body was corrupted after pooled buffer reuse: got %q... want %q...", got[:8], first[:8])
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
