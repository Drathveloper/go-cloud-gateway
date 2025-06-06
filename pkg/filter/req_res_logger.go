package filter

import (
	"log/slog"
	"strings"

	"github.com/drathveloper/go-cloud-gateway/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

const RequestResponseLoggerFilterName = "RequestResponseLogger"

type RequestResponseLogger struct {
	level slog.Level
}

func NewRequestResponseLoggerFilter(level slog.Level) *RequestResponseLogger {
	return &RequestResponseLogger{
		level: level,
	}
}

func NewRequestResponseLoggerBuilder() gateway.FilterBuilder {
	return gateway.FilterBuilderFunc(func(args map[string]any) (gateway.Filter, error) {
		level, _ := common.ConvertToString(args["level"])
		switch strings.ToLower(level) {
		case "debug":
			return NewRequestResponseLoggerFilter(slog.LevelDebug), nil
		case "info":
			return NewRequestResponseLoggerFilter(slog.LevelInfo), nil
		case "warn":
			return NewRequestResponseLoggerFilter(slog.LevelWarn), nil
		case "error":
			return NewRequestResponseLoggerFilter(slog.LevelError), nil
		default:
			return NewRequestResponseLoggerFilter(slog.LevelInfo), nil
		}
	})
}

func (f *RequestResponseLogger) PreProcess(ctx *gateway.Context) error {
	if ctx.Logger.Enabled(ctx, f.level) {
		ctx.Logger.Log(ctx, f.level, "Received request",
			"url", ctx.Request.Method+" "+ctx.Request.URL.String(),
			"headers", ctx.Request.Headers,
			"body", ctx.Request.Body)
	}
	return nil
}

func (f *RequestResponseLogger) PostProcess(ctx *gateway.Context) error {
	if ctx.Logger.Enabled(ctx, f.level) {
		ctx.Logger.Log(ctx, f.level, "Returned response",
			"status", ctx.Response.Status,
			"headers", ctx.Response.Headers,
			"body", ctx.Response.Body)
	}
	return nil
}

func (f *RequestResponseLogger) Name() string {
	return RequestResponseLoggerFilterName
}
