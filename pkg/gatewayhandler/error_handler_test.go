package gatewayhandler_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/gatewayhandler"
)

type DummyWriter struct {
	WriteErr           error
	CurrHeader         http.Header
	ExpectedStatusCode int
}

func (d DummyWriter) Header() http.Header {
	return d.CurrHeader
}

func (d DummyWriter) Write(_ []byte) (int, error) {
	return 0, d.WriteErr
}

func (d DummyWriter) WriteHeader(statusCode int) {
	if d.ExpectedStatusCode != statusCode {
		panic(fmt.Sprintf("unexpected status code: %d", statusCode))
	}
}

func TestBaseErrorHandler(t *testing.T) {
	tests := []struct {
		err                error
		name               string
		expectedErrMsg     string
		expectedStatusCode int
	}{
		{
			name:           "test base error handler should succeed when error is nil",
			err:            nil,
			expectedErrMsg: "",
		},
		{
			name:               "test base error handler should succeed when error is route not found",
			expectedStatusCode: http.StatusNotFound,
			err:                gatewayhandler.ErrRouteNotFound,
			expectedErrMsg:     "level=INFO msg=\"route not found\"",
		},
		{
			name:               "test base error handler should succeed when error is deadline exceeded",
			expectedStatusCode: http.StatusBadGateway,
			err:                context.DeadlineExceeded,
			expectedErrMsg:     "level=ERROR msg=\"request timeout\" error=\"context deadline exceeded\"",
		},
		{
			name:               "test base error handler should succeed when error is generic http error",
			expectedStatusCode: http.StatusBadGateway,
			err:                gateway.ErrHTTP,
			expectedErrMsg:     "level=ERROR msg=\"http request failed\" error=\"gateway http request to backend failed\"",
		},
		{
			name:               "test base error handler should succeed when error is rate limit exceeded",
			expectedStatusCode: http.StatusTooManyRequests,
			err:                filter.ErrRateLimitExceeded,
			expectedErrMsg:     "level=ERROR msg=\"rate limit exceeded\" error=\"rate limit exceeded",
		},
		{
			name:               "test base error handler should succeed when error is unhandled error",
			expectedStatusCode: http.StatusInternalServerError,
			err:                io.EOF,
			expectedErrMsg:     "level=ERROR msg=\"unexpected error\" error=EOF",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&buf, nil))
			writer := &DummyWriter{
				CurrHeader:         http.Header{},
				ExpectedStatusCode: tt.expectedStatusCode,
				WriteErr:           nil,
			}

			gatewayhandler.BaseErrorHandler().Handle(logger, tt.err, writer)

			if !strings.Contains(buf.String(), tt.expectedErrMsg) {
				t.Errorf("expected error message: %s actual: %s", tt.expectedErrMsg, buf.String())
			}
		})
	}
}
