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
}

// NewRequestResponseLoggerFilter creates a new RequestResponseLoggerFilter. Bodies larger
// than maxBodyBytes are forwarded untouched and logged without their content; a negative
// value disables the limit.
func NewRequestResponseLoggerFilter(level slog.Level, maxBodyBytes int64) *RequestResponseLogger {
	return &RequestResponseLogger{
		level:        level,
		maxBodyBytes: maxBodyBytes,
	}
}

// NewRequestResponseLoggerBuilder creates a new RequestResponseLoggerBuilder.
//
// The "level" argument selects the log level (debug, info, warn or error; default info).
// The "max-body-bytes" argument caps how many body bytes are buffered in memory and logged
// per request and response (default DefaultMaxLoggedBodyBytes; negative means unlimited).
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
		return NewRequestResponseLoggerFilter(level, maxBodyBytes), nil
	}
}

// PreProcess logs the request.
func (f *RequestResponseLogger) PreProcess(ctx *gateway.Context) error {
	if ctx.Logger.Enabled(ctx, f.level) {
		var body []byte
		if err := ctx.Request.BodyReader.CaptureWithLimit(f.maxBodyBytes); err == nil {
			body, _ = shared.ReadBody(ctx.Request.BodyReader)
		}
		ctx.Logger.Log(ctx, f.level, "Received request",
			"url", ctx.Request.Method+" "+ctx.Request.URL.String(),
			"headers", ctx.Request.Headers,
			"body", body)
	}
	return nil
}

// PostProcess logs the response.
func (f *RequestResponseLogger) PostProcess(ctx *gateway.Context) error {
	if ctx.Logger.Enabled(ctx, f.level) {
		var body []byte
		if err := ctx.Response.BodyReader.CaptureWithLimit(f.maxBodyBytes); err == nil {
			body, _ = shared.ReadBody(ctx.Response.BodyReader)
		}
		ctx.Logger.Log(ctx, f.level, "Returned response",
			"status", ctx.Response.Status,
			"headers", ctx.Response.Headers,
			"body", body)
	}
	return nil
}

// Name returns the name of the filter.
func (f *RequestResponseLogger) Name() string {
	return RequestResponseLoggerFilterName
}
