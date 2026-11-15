package bootstrap_test

import (
	"bytes"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/drathveloper/go-cloud-gateway/pkg/gatewayhandler"
)

// logBuffer collects the log of a gateway under test. The gateway writes it from the
// server goroutines while the test reads it, hence the mutex.
type logBuffer struct {
	buf   bytes.Buffer
	mutex sync.Mutex
}

func (b *logBuffer) Write(output []byte) (int, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	return b.buf.Write(output) //nolint:wrapcheck
}

// Lines returns the lines logged so far.
func (b *logBuffer) Lines() []string {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.buf.Len() == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(b.buf.String(), "\n"), "\n")
}

// newTestLogger returns a logger that writes one text line per record to logs. The time
// is left out so that the lines can be compared.
func newTestLogger(logs *logBuffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) == 0 && attr.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return attr
		},
	}))
}

// setDefaultLogger makes the given logger the default one until the test ends.
func setDefaultLogger(t *testing.T, logger *slog.Logger) {
	t.Helper()
	previous := slog.Default()
	previousOutput, previousFlags := log.Writer(), log.Flags()
	t.Cleanup(func() {
		slog.SetDefault(previous)
		// slog.SetDefault also redirects the log package to the new handler, and does
		// not undo it when the previous default is put back.
		log.SetOutput(previousOutput)
		log.SetFlags(previousFlags)
	})
	slog.SetDefault(logger)
}

// hasPrefixes reports whether there are as many lines as prefixes and each line starts
// with the prefix in its position. Prefixes are used for the lines that end with values
// that change on every run, such as the port of a backend.
func hasPrefixes(lines, prefixes []string) bool {
	return slices.EqualFunc(lines, prefixes, strings.HasPrefix)
}

// newStatusBackend returns a backend that answers every request with the given status.
func newStatusBackend(t *testing.T, status int) *httptest.Server {
	t.Helper()
	backend := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(status)
	}))
	t.Cleanup(backend.Close)
	return backend
}

// newUnreachableBackendURL returns the url of a backend that refuses every connection.
func newUnreachableBackendURL(t *testing.T) string {
	t.Helper()
	backend := httptest.NewServer(http.NotFoundHandler())
	backend.Close()
	return backend.URL
}

func TestInitialize_Logger(t *testing.T) {
	tests := []struct {
		build         func(cfg *config.Config, logger *slog.Logger) *bootstrap.Options
		name          string
		expectDefault bool
	}{
		{
			name: "logger set in the options builder should be used",
			build: func(cfg *config.Config, logger *slog.Logger) *bootstrap.Options {
				return bootstrap.NewOptionsBuilder(cfg).WithLogger(logger).Build()
			},
			expectDefault: false,
		},
		{
			name: "logger set in options built by hand should be used",
			build: func(cfg *config.Config, logger *slog.Logger) *bootstrap.Options {
				return &bootstrap.Options{
					Config:              cfg,
					Logger:              logger,
					GatewayErrorHandler: gatewayhandler.BaseErrorHandler(),
				}
			},
			expectDefault: false,
		},
		{
			name: "default logger should be used when the options builder has no logger",
			build: func(cfg *config.Config, _ *slog.Logger) *bootstrap.Options {
				return bootstrap.NewOptionsBuilder(cfg).Build()
			},
			expectDefault: true,
		},
		{
			name: "default logger should be used when the options builder is given a nil logger",
			build: func(cfg *config.Config, _ *slog.Logger) *bootstrap.Options {
				return bootstrap.NewOptionsBuilder(cfg).WithLogger(nil).Build()
			},
			expectDefault: true,
		},
		{
			name: "default logger should be used when options built by hand have no logger",
			build: func(cfg *config.Config, _ *slog.Logger) *bootstrap.Options {
				return &bootstrap.Options{
					Config:              cfg,
					GatewayErrorHandler: gatewayhandler.BaseErrorHandler(),
				}
			},
			expectDefault: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var optionLogs, defaultLogs logBuffer
			cfg := readTestConfig(t, fmt.Sprintf(
				`{"gateway":{"routes":[{"id":"r1","uri":%q}]}}`, newUnreachableBackendURL(t)))
			opts := tt.build(cfg, newTestLogger(&optionLogs))
			// The default logger is installed once the options exist: it is the one of
			// the moment the gateway is initialized that counts.
			setDefaultLogger(t, newTestLogger(&defaultLogs))
			gatewayServer := startTestGatewayWithOptions(t, opts)

			status, _, err := getTestURL(t, gatewayServer.Client(), gatewayServer.URL+"/test")
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}

			if status != http.StatusBadGateway {
				t.Errorf("expected status %d actual %d", http.StatusBadGateway, status)
			}
			expectedLogs, otherLogs := optionLogs.Lines(), defaultLogs.Lines()
			if tt.expectDefault {
				expectedLogs, otherLogs = otherLogs, expectedLogs
			}
			expected := []string{`level=ERROR msg="http request failed" route=r1 error=`}
			if !hasPrefixes(expectedLogs, expected) {
				t.Errorf("expected log lines starting with %q actual %q", expected, expectedLogs)
			}
			if len(otherLogs) != 0 {
				t.Errorf("expected nothing in the other logger actual %q", otherLogs)
			}
		})
	}
}

func TestInitialize_RouteLogsCarryTheRouteID(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		expected       []string
		expectedStatus int
	}{
		{
			name:           "filter logs should carry the id of the matched route",
			path:           "/one",
			expectedStatus: http.StatusOK,
			expected: []string{
				`level=INFO msg="Received request" route=r1 url=`,
				`level=INFO msg="Returned response" route=r1 status=200 `,
			},
		},
		{
			name:           "error handler logs should carry the id of the matched route",
			path:           "/two",
			expectedStatus: http.StatusBadGateway,
			expected: []string{
				`level=INFO msg="Received request" route=r2 url=`,
				`level=ERROR msg="http request failed" route=r2 error=`,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs logBuffer
			// Bodies are left out of the log: the response body is logged once it has
			// been sent, which may be after the client has read it.
			cfgJSON := fmt.Sprintf(`{"gateway":{`+
				`"global-filters":[{"name":"RequestResponseLogger","args":{"log-bodies":false}}],`+
				`"routes":[`+
				`{"id":"r1","uri":%q,"predicates":[{"name":"Path","args":{"patterns":["/one"]}}]},`+
				`{"id":"r2","uri":%q,"predicates":[{"name":"Path","args":{"patterns":["/two"]}}]}]}}`,
				newStatusBackend(t, http.StatusOK).URL, newUnreachableBackendURL(t))
			opts := bootstrap.NewOptionsBuilder(readTestConfig(t, cfgJSON)).WithLogger(newTestLogger(&logs)).Build()
			gatewayServer := startTestGatewayWithOptions(t, opts)

			status, _, err := getTestURL(t, gatewayServer.Client(), gatewayServer.URL+tt.path)
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}

			if status != tt.expectedStatus {
				t.Errorf("expected status %d actual %d", tt.expectedStatus, status)
			}
			if actual := logs.Lines(); !hasPrefixes(actual, tt.expected) {
				t.Errorf("expected log lines starting with %q actual %q", tt.expected, actual)
			}
		})
	}
}

// A breaker that opens makes the gateway answer 503 from then on: the log must say when
// it happened and for which route.
func TestInitialize_CircuitBreakerOpeningIsLogged(t *testing.T) {
	const (
		opened     = `level=WARN msg="circuit breaker state changed" route=r1 breaker=r1 from=closed to=open`
		rejected   = `level=ERROR msg="circuit breaker is open" route=r1 error=`
		httpFailed = `level=ERROR msg="http request failed" route=r1 error=`
	)
	tests := []struct {
		backendURL func(t *testing.T) string
		name       string
		expected   []string
		statuses   []int
	}{
		{
			name: "breaker opened by backend errors should log it before rejecting requests",
			backendURL: func(t *testing.T) string {
				t.Helper()
				return newStatusBackend(t, http.StatusInternalServerError).URL
			},
			statuses: []int{
				http.StatusInternalServerError,
				http.StatusInternalServerError,
				http.StatusInternalServerError,
				http.StatusServiceUnavailable,
			},
			expected: []string{opened, rejected},
		},
		{
			name:       "breaker opened by an unreachable backend should log it before rejecting requests",
			backendURL: newUnreachableBackendURL,
			statuses: []int{
				http.StatusBadGateway,
				http.StatusBadGateway,
				http.StatusBadGateway,
				http.StatusServiceUnavailable,
			},
			// The breaker opens while the third request is still in flight, before the
			// error handler logs its failure.
			expected: []string{httpFailed, httpFailed, opened, httpFailed, rejected},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs logBuffer
			cfgJSON := fmt.Sprintf(`{"gateway":{"routes":[{"id":"r1","uri":%q,"circuit-breaker":{`+
				`"enabled":true,"interval":"1m","failure-rate-threshold":50,"num-allowed-half-open-calls":1,`+
				`"wait-duration-in-open-state":"1m","min-requests-threshold":3}}]}}`, tt.backendURL(t))
			opts := bootstrap.NewOptionsBuilder(readTestConfig(t, cfgJSON)).WithLogger(newTestLogger(&logs)).Build()
			gatewayServer := startTestGatewayWithOptions(t, opts)

			statuses := make([]int, 0, len(tt.statuses))
			for range tt.statuses {
				status, _, err := getTestURL(t, gatewayServer.Client(), gatewayServer.URL+"/test")
				if err != nil {
					t.Fatalf("expected nil err actual %v", err)
				}
				statuses = append(statuses, status)
			}

			if !slices.Equal(tt.statuses, statuses) {
				t.Errorf("expected statuses %v actual %v", tt.statuses, statuses)
			}
			if actual := logs.Lines(); !hasPrefixes(actual, tt.expected) {
				t.Errorf("expected log lines starting with %q actual %q", tt.expected, actual)
			}
		})
	}
}
