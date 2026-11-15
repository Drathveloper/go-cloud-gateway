package config_test

import (
	"bytes"
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/circuitbreaker"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

// newBufferLogger returns a logger that writes one line per record to buf, as text or as
// JSON. The time is left out so that the lines can be compared.
func newBufferLogger(buf *bytes.Buffer, json bool) *slog.Logger {
	opts := &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
			if len(groups) == 0 && attr.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return attr
		},
	}
	if json {
		return slog.New(slog.NewJSONHandler(buf, opts))
	}
	return slog.New(slog.NewTextHandler(buf, opts))
}

// loggedLines returns the lines written to buf so far.
func loggedLines(buf *bytes.Buffer) []string {
	if buf.Len() == 0 {
		return nil
	}
	return strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
}

// probeLine returns the line the logger writes to buf for a record. Loggers derived with
// With are compared by what they write: reflect.DeepEqual cannot tell whether two of them
// carry the same attributes.
func probeLine(buf *bytes.Buffer, logger *slog.Logger) string {
	buf.Reset()
	logger.Info("probe")
	return buf.String()
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

func TestNewRoutes_RouteLoggers(t *testing.T) {
	var logs, defaultLogs bytes.Buffer
	setDefaultLogger(t, newBufferLogger(&defaultLogs, false))
	tests := []struct {
		logger   *slog.Logger
		written  *bytes.Buffer
		name     string
		expected []string
	}{
		{
			name:    "each route should log through the given logger with its own id",
			logger:  newBufferLogger(&logs, false).With("app", "gateway"),
			written: &logs,
			expected: []string{
				"level=INFO msg=probe app=gateway route=r1",
				"level=INFO msg=probe app=gateway route=r2",
			},
		},
		{
			name:    "each route should log through the default logger with its own id when the logger is nil",
			logger:  nil,
			written: &defaultLogs,
			expected: []string{
				"level=INFO msg=probe route=r1",
				"level=INFO msg=probe route=r2",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				Gateway: config.Gateway{
					Routes: []config.Route{
						{ID: "r1", URI: "http://localhost:8001"},
						{ID: "r2", URI: "http://localhost:8002"},
					},
				},
			}

			routes, err := config.NewRoutes(
				cfg, predicate.NewFactory(predicate.BuilderRegistry), filter.NewFactory(filter.BuilderRegistry), tt.logger)
			if err != nil {
				t.Fatalf("expected nil err actual %v", err)
			}
			for i := range routes {
				routes[i].Logger.Info("probe")
			}

			if actual := loggedLines(tt.written); !slices.Equal(tt.expected, actual) {
				t.Errorf("expected %q actual %q", tt.expected, actual)
			}
		})
	}
}

// succeedRequest runs a successful request through the breaker.
func succeedRequest(breaker gateway.CircuitBreaker[*http.Response]) {
	_, _ = breaker.Execute(func() (*http.Response, error) { //nolint:bodyclose // the callback returns no response
		return nil, nil //nolint:nilnil
	})
}

// failRequest runs a failed request through the breaker.
func failRequest(breaker gateway.CircuitBreaker[*http.Response]) {
	_, _ = breaker.Execute(func() (*http.Response, error) { //nolint:bodyclose // the callback returns no response
		return nil, errors.New("dial tcp 10.0.0.1:443: connect: connection refused")
	})
}

func TestNewRoutes_CircuitBreakerLogsStateChanges(t *testing.T) {
	const (
		waitInOpenState = 20 * time.Millisecond
		opened          = `level=WARN msg="circuit breaker state changed" route=r1 breaker=r1 from=closed to=open`
		halfOpened      = `level=INFO msg="circuit breaker state changed" route=r1 breaker=r1 from=open to=half-open`
		closed          = `level=INFO msg="circuit breaker state changed" route=r1 breaker=r1 from=half-open to=closed`
		reopened        = `level=WARN msg="circuit breaker state changed" route=r1 breaker=r1 from=half-open to=open`
	)
	// The breaker only leaves the open state when it is used once the wait is over, so
	// sleeping for longer than needed cannot change the outcome.
	waitOpenState := func(_ gateway.CircuitBreaker[*http.Response]) {
		time.Sleep(2 * waitInOpenState)
	}
	tests := []struct {
		name     string
		steps    []func(breaker gateway.CircuitBreaker[*http.Response])
		expected []string
	}{
		{
			name: "breaker that stays closed should log nothing",
			steps: []func(gateway.CircuitBreaker[*http.Response]){
				succeedRequest, succeedRequest, failRequest, succeedRequest,
			},
			expected: nil,
		},
		{
			name: "breaker that opens should log it at warn level",
			steps: []func(gateway.CircuitBreaker[*http.Response]){
				failRequest, failRequest, failRequest,
			},
			expected: []string{opened},
		},
		{
			name: "open breaker that rejects requests should log nothing else",
			steps: []func(gateway.CircuitBreaker[*http.Response]){
				failRequest, failRequest, failRequest, failRequest, succeedRequest,
			},
			expected: []string{opened},
		},
		{
			name: "breaker that half-opens and closes should log both at info level",
			steps: []func(gateway.CircuitBreaker[*http.Response]){
				failRequest, failRequest, failRequest, waitOpenState, succeedRequest,
			},
			expected: []string{opened, halfOpened, closed},
		},
		{
			name: "half-open breaker that opens again should log it at warn level",
			steps: []func(gateway.CircuitBreaker[*http.Response]){
				failRequest, failRequest, failRequest, waitOpenState, failRequest,
			},
			expected: []string{opened, halfOpened, reopened},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			routes := newCircuitBreakerRoute(t, newBufferLogger(&logs, false), waitInOpenState)

			for _, step := range tt.steps {
				step(routes[0].CircuitBreaker)
			}

			if actual := loggedLines(&logs); !slices.Equal(tt.expected, actual) {
				t.Errorf("expected %q actual %q", tt.expected, actual)
			}
		})
	}
}

// A circuit breaker state is an integer: logged as it is, a JSON handler prints 0, 1 or 2
// instead of its name.
func TestNewRoutes_CircuitBreakerLogsStatesByName(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		json     bool
	}{
		{
			name:     "text handler should log the state names",
			json:     false,
			expected: `level=WARN msg="circuit breaker state changed" route=r1 breaker=r1 from=closed to=open`,
		},
		{
			name: "json handler should log the state names",
			json: true,
			expected: `{"level":"WARN","msg":"circuit breaker state changed",` +
				`"route":"r1","breaker":"r1","from":"closed","to":"open"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			routes := newCircuitBreakerRoute(t, newBufferLogger(&logs, tt.json), time.Minute)

			for range 3 {
				failRequest(routes[0].CircuitBreaker)
			}

			if actual := loggedLines(&logs); !slices.Equal([]string{tt.expected}, actual) {
				t.Errorf("expected %q actual %q", tt.expected, actual)
			}
		})
	}
}

// stalledHandler is a log handler whose output makes no progress: Handle reports that it
// was called and does not return until it is released.
type stalledHandler struct {
	entered chan struct{}
	release chan struct{}
}

func (h stalledHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h stalledHandler) Handle(context.Context, slog.Record) error {
	h.entered <- struct{}{}
	<-h.release
	return nil
}

func (h stalledHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h stalledHandler) WithGroup(string) slog.Handler {
	return h
}

// The breaker used to log its state changes while it held its lock: a log handler that
// was slow held up every request of the route until the line was written.
func TestNewRoutes_CircuitBreakerSlowLogHandlerDoesNotHoldUpTheRoute(t *testing.T) {
	// Only a failing test waits this long.
	const watchdog = 5 * time.Second
	handler := stalledHandler{entered: make(chan struct{}, 1), release: make(chan struct{})}
	breaker := newCircuitBreakerRoute(t, slog.New(handler), time.Minute)[0].CircuitBreaker
	opened := make(chan struct{})
	go func() {
		defer close(opened)
		for range 3 {
			failRequest(breaker)
		}
	}()
	select {
	case <-handler.entered:
	case <-time.After(watchdog):
		t.Fatal("expected the breaker to log that it opened")
	}

	answered := make(chan error, 1)
	go func() {
		_, err := breaker.Execute(func() (*http.Response, error) { //nolint:bodyclose // the callback returns no response
			return nil, nil //nolint:nilnil
		})
		answered <- err
	}()

	select {
	case err := <-answered:
		if !errors.Is(err, circuitbreaker.ErrOpenState) {
			t.Errorf("expected %v actual %v", circuitbreaker.ErrOpenState, err)
		}
	case <-time.After(watchdog):
		t.Error("expected the route to answer while the state change is being logged")
	}
	close(handler.release)
	<-opened
}
