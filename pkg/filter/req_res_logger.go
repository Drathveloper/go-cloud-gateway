package filter

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// RequestResponseLoggerFilterName is the name of the filter.
const RequestResponseLoggerFilterName = "RequestResponseLogger"

// DefaultMaxLoggedBodyBytes is the default maximum number of body bytes the filter buffers
// and logs. Bigger bodies are forwarded untouched and logged without their content.
const DefaultMaxLoggedBodyBytes int64 = 64 * 1024

// RequestResponseLogger is a filter that logs the request and response.
type RequestResponseLogger struct {
	level        slog.Level
	maxBodyBytes int64
	logBodies    bool
}

// NewRequestResponseLoggerFilter creates a new RequestResponseLoggerFilter. Bodies larger
// than maxBodyBytes are forwarded untouched and logged without their content; a negative
// value disables the limit.
func NewRequestResponseLoggerFilter(level slog.Level, maxBodyBytes int64, logBodies bool) *RequestResponseLogger {
	return &RequestResponseLogger{
		level:        level,
		maxBodyBytes: maxBodyBytes,
		logBodies:    logBodies,
	}
}

// NewHeadersOnlyRequestResponseLoggerFilter creates a RequestResponseLogger that logs
// method, URL, status and headers but never touches the bodies. Skipping the body
// capture avoids buffering and copying every request and response body, so this is
// the cheap variant for production traffic.
func NewHeadersOnlyRequestResponseLoggerFilter(level slog.Level) *RequestResponseLogger {
	return &RequestResponseLogger{
		level:        level,
		maxBodyBytes: 0,
		logBodies:    false,
	}
}

// NewRequestResponseLoggerBuilder creates a new RequestResponseLoggerBuilder.
//
// The "level" argument selects the log level (debug, info, warn or error; default info).
// The "max-body-bytes" argument caps how many body bytes are buffered in memory and logged
// per request and response (default DefaultMaxLoggedBodyBytes; negative means unlimited).
// The "log-bodies" argument selects whether bodies are logged at all (default true).
// Disabling it skips the body capture entirely, so no body is buffered or copied.
func NewRequestResponseLoggerBuilder() gateway.FilterBuilderFunc {
	return func(args map[string]any) (gateway.Filter, error) {
		maxBodyBytes := DefaultMaxLoggedBodyBytes
		if args["max-body-bytes"] != nil {
			maxBody, err := shared.ConvertToInt(args["max-body-bytes"])
			if err != nil {
				return nil, fmt.Errorf("failed to convert 'max-body-bytes' attribute: %w", err)
			}
			maxBodyBytes = int64(maxBody)
		}
		logBodies := true
		if args["log-bodies"] != nil {
			logBodiesArg, err := shared.ConvertToBool(args["log-bodies"])
			if err != nil {
				return nil, fmt.Errorf("failed to convert 'log-bodies' attribute: %w", err)
			}
			logBodies = logBodiesArg
		}
		levelStr, _ := shared.ConvertToString(args["level"])
		var level slog.Level
		switch strings.ToLower(levelStr) {
		case "debug":
			level = slog.LevelDebug
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		default:
			level = slog.LevelInfo
		}
		return NewRequestResponseLoggerFilter(level, maxBodyBytes, logBodies), nil
	}
}

// PreProcess logs the request.
func (f *RequestResponseLogger) PreProcess(ctx *gateway.Context) error {
	if !ctx.Logger.Enabled(ctx, f.level) {
		return nil
	}
	if !f.logBodies {
		ctx.Logger.Log(ctx, f.level, "Received request",
			"url", ctx.Request.Method+" "+ctx.Request.URL.String(),
			"headers", ctx.Request.Headers)
		return nil
	}
	var body []byte
	if err := ctx.Request.BodyReader.CaptureWithLimit(f.maxBodyBytes); err == nil {
		// The captured buffer is logged as-is: re-reading the body here would
		// copy it twice more for no benefit.
		body = ctx.Request.BodyReader.Bytes()
	}
	ctx.Logger.Log(ctx, f.level, "Received request",
		"url", ctx.Request.Method+" "+ctx.Request.URL.String(),
		"headers", ctx.Request.Headers,
		"body", body)
	return nil
}

// PostProcess logs the response.
func (f *RequestResponseLogger) PostProcess(ctx *gateway.Context) error {
	if !ctx.Logger.Enabled(ctx, f.level) {
		return nil
	}
	if !f.logBodies {
		ctx.Logger.Log(ctx, f.level, "Returned response",
			"status", ctx.Response.Status,
			"headers", ctx.Response.Headers)
		return nil
	}
	var body []byte
	if err := ctx.Response.BodyReader.CaptureWithLimit(f.maxBodyBytes); err == nil {
		// The captured buffer is logged as-is: re-reading the body here would
		// copy it twice more for no benefit.
		body = ctx.Response.BodyReader.Bytes()
	}
	ctx.Logger.Log(ctx, f.level, "Returned response",
		"status", ctx.Response.Status,
		"headers", ctx.Response.Headers,
		"body", body)
	return nil
}

// Name returns the name of the filter.
func (f *RequestResponseLogger) Name() string {
	return RequestResponseLoggerFilterName
}
