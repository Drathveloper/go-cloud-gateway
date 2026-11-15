package gatewayhandler_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/gatewayhandler"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
	"github.com/drathveloper/go-cloud-gateway/pkg/ratelimit"
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
			name:               "test base error handler should succeed when error is deadline exceeded",
			expectedStatusCode: http.StatusGatewayTimeout,
			err:                context.DeadlineExceeded,
			expectedErrMsg:     "level=ERROR msg=\"request timeout\" error=\"context deadline exceeded\"",
		},
		{
			name:               "test base error handler should succeed when error is context canceled",
			expectedStatusCode: 499,
			err:                context.Canceled,
			expectedErrMsg:     "level=WARN msg=\"client closed request\" error=\"context canceled\"",
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
			expectedErrMsg:     "level=WARN msg=\"rate limit exceeded\" error=\"rate limit exceeded",
		},
		{
			name:               "test base error handler should succeed when error is rate limit exceeded",
			expectedStatusCode: http.StatusServiceUnavailable,
			err:                gateway.ErrCircuitBreaker,
			expectedErrMsg:     "level=ERROR msg=\"circuit breaker is open\" error=\"circuit breaker failed",
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
			ctx := &gateway.Context{
				Logger: logger,
			}
			writer := &DummyWriter{
				CurrHeader:         http.Header{},
				ExpectedStatusCode: tt.expectedStatusCode,
				WriteErr:           nil,
			}

			gatewayhandler.BaseErrorHandler().Handle(ctx, tt.err, writer)

			if !strings.Contains(buf.String(), tt.expectedErrMsg) {
				t.Errorf("expected error message: %s actual: %s", tt.expectedErrMsg, buf.String())
			}
		})
	}
}

func TestBaseErrorHandler_RetryAfter(t *testing.T) {
	tests := []struct {
		err                error
		name               string
		expectedRetryAfter []string
		expectedStatusCode int
	}{
		{
			name:               "base error handler should set retry after when the wait is a whole number of seconds",
			err:                &filter.RateLimitExceededError{RetryAfter: 2 * time.Second},
			expectedStatusCode: http.StatusTooManyRequests,
			expectedRetryAfter: []string{"2"},
		},
		{
			name:               "base error handler should round retry after up when the wait has a fraction of a second",
			err:                &filter.RateLimitExceededError{RetryAfter: 1500 * time.Millisecond},
			expectedStatusCode: http.StatusTooManyRequests,
			expectedRetryAfter: []string{"2"},
		},
		{
			name:               "base error handler should round retry after up when the wait is just over a second",
			err:                &filter.RateLimitExceededError{RetryAfter: time.Second + time.Nanosecond},
			expectedStatusCode: http.StatusTooManyRequests,
			expectedRetryAfter: []string{"2"},
		},
		{
			name:               "base error handler should set retry after to one second when the wait is shorter",
			err:                &filter.RateLimitExceededError{RetryAfter: time.Nanosecond},
			expectedStatusCode: http.StatusTooManyRequests,
			expectedRetryAfter: []string{"1"},
		},
		{
			name:               "base error handler should set retry after when the wait is long",
			err:                &filter.RateLimitExceededError{RetryAfter: 90 * time.Minute},
			expectedStatusCode: http.StatusTooManyRequests,
			expectedRetryAfter: []string{"5400"},
		},
		{
			// The gateway wraps the filter error twice before handing it over.
			name: "base error handler should set retry after when the rate limit error is wrapped",
			err: fmt.Errorf("gateway request for route r1 failed: %w",
				fmt.Errorf("pre-process filters failed with filter RateLimit: %w",
					&filter.RateLimitExceededError{RetryAfter: 300 * time.Millisecond})),
			expectedStatusCode: http.StatusTooManyRequests,
			expectedRetryAfter: []string{"1"},
		},
		{
			name:               "base error handler should not set retry after when the wait is not known",
			err:                &filter.RateLimitExceededError{RetryAfter: 0},
			expectedStatusCode: http.StatusTooManyRequests,
			expectedRetryAfter: nil,
		},
		{
			name:               "base error handler should not set retry after when the wait is negative",
			err:                &filter.RateLimitExceededError{RetryAfter: -time.Second},
			expectedStatusCode: http.StatusTooManyRequests,
			expectedRetryAfter: nil,
		},
		{
			name:               "base error handler should not set retry after when the error is the bare rate limit error",
			err:                fmt.Errorf("custom limiter: %w", filter.ErrRateLimitExceeded),
			expectedStatusCode: http.StatusTooManyRequests,
			expectedRetryAfter: nil,
		},
		{
			name:               "base error handler should not set retry after when the error is not a rate limit error",
			err:                gateway.ErrCircuitBreaker,
			expectedStatusCode: http.StatusServiceUnavailable,
			expectedRetryAfter: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &gateway.Context{Logger: slog.New(slog.DiscardHandler)}
			recorder := httptest.NewRecorder()

			gatewayhandler.BaseErrorHandler().Handle(ctx, tt.err, recorder)

			if tt.expectedStatusCode != recorder.Code {
				t.Errorf("expected status %d actual %d", tt.expectedStatusCode, recorder.Code)
			}
			actual := recorder.Header().Values("Retry-After")
			if fmt.Sprint(tt.expectedRetryAfter) != fmt.Sprint(actual) {
				t.Errorf("expected retry after %v actual %v", tt.expectedRetryAfter, actual)
			}
		})
	}
}

// A rejected request is the limiter working as intended: it must not reach the error log,
// which a client hammering the gateway would otherwise flood.
func TestBaseErrorHandler_LogsRateLimitRejectionsAtWarnLevel(t *testing.T) {
	tests := []struct {
		err  error
		name string
	}{
		{
			name: "base error handler should log at warn level when the error is the bare rate limit error",
			err:  filter.ErrRateLimitExceeded,
		},
		{
			name: "base error handler should log at warn level when the error carries the wait",
			err:  &filter.RateLimitExceededError{RetryAfter: time.Second},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			ctx := &gateway.Context{Logger: slog.New(slog.NewJSONHandler(&buf, nil))}

			gatewayhandler.BaseErrorHandler().Handle(ctx, tt.err, httptest.NewRecorder())

			if strings.Count(buf.String(), "\n") != 1 {
				t.Fatalf("expected %d log record actual %s", 1, buf.String())
			}
			if !strings.Contains(buf.String(), `"level":"WARN"`) {
				t.Errorf("expected level %s actual %s", slog.LevelWarn, buf.String())
			}
		})
	}
}

// okClient is a backend that answers every request with an empty 200.
type okClient struct{}

func (okClient) Do(_ *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: http.NoBody}, nil
}

// frozenClock is a time provider that never moves, so a bucket never refills.
type frozenClock struct {
	now time.Time
}

func (c *frozenClock) Now() time.Time {
	return c.now
}

// The whole path a rejected request takes: the limiter reports the wait, the filter
// carries it in its error, the gateway wraps that error and the handler answers with it.
func TestBaseErrorHandler_RateLimitedRouteAnswersWithRetryAfter(t *testing.T) {
	var logs bytes.Buffer
	limiter := ratelimit.NewInMemoryRateLimiter(&frozenClock{now: time.Now()}, 1, 1)
	route, err := gateway.NewRoute(
		"r1",
		"http://backend.internal",
		gateway.Predicates{predicate.NewMethodPredicate(http.MethodGet)},
		nil,
		gateway.Filters{filter.NewRateLimitFilter(limiter, ratelimit.NewPathKeyFunc())},
		time.Minute,
		nil,
		slog.New(slog.NewTextHandler(&logs, nil)))
	if err != nil {
		t.Fatalf("expected err %v actual %v", nil, err)
	}
	gwHandler := gatewayhandler.NewGatewayHandler(
		gateway.NewGateway(okClient{}), gateway.Routes{*route}, gatewayhandler.BaseErrorHandler())

	first := httptest.NewRecorder()
	gwHandler.ServeHTTP(first, newTestRequest(t, http.MethodGet, "http://localhost:8080/test", nil))
	second := httptest.NewRecorder()
	gwHandler.ServeHTTP(second, newTestRequest(t, http.MethodGet, "http://localhost:8080/test", nil))

	if first.Code != http.StatusOK {
		t.Errorf("expected status %d actual %d", http.StatusOK, first.Code)
	}
	if actual := first.Header().Values("Retry-After"); len(actual) != 0 {
		t.Errorf("expected retry after %v actual %v", nil, actual)
	}
	if second.Code != http.StatusTooManyRequests {
		t.Errorf("expected status %d actual %d", http.StatusTooManyRequests, second.Code)
	}
	if actual := second.Header().Get("Retry-After"); actual != "1" {
		t.Errorf("expected retry after %s actual %s", "1", actual)
	}
	if !strings.Contains(logs.String(), "level=WARN msg=\"rate limit exceeded\"") {
		t.Errorf("expected a warn log record actual %s", logs.String())
	}
	if strings.Contains(logs.String(), "level=ERROR") {
		t.Errorf("expected no error log record actual %s", logs.String())
	}
}
