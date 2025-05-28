package filter

import (
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

const RequestResponseLoggerFilterName = "RequestResponseLogger"

type RequestResponseLogger struct{}

func NewRequestResponseLoggerFilter() *RequestResponseLogger {
	return &RequestResponseLogger{}
}

func NewRequestResponseLoggerBuilder() gateway.FilterBuilder {
	return gateway.FilterBuilderFunc(func(args map[string]any) (gateway.Filter, error) {
		return NewRequestResponseLoggerFilter(), nil
	})
}

func (f *RequestResponseLogger) PreProcess(ctx *gateway.Context) error {
	ctx.Logger.Info("Received request",
		"url", ctx.Request.Method+" "+ctx.Request.URL.String(),
		"headers", ctx.Request.Headers,
		"body", string(ctx.Request.Body))
	return nil
}

func (f *RequestResponseLogger) PostProcess(ctx *gateway.Context) error {
	ctx.Logger.Info("Returned response",
		"status", ctx.Response.Status,
		"headers", ctx.Response.Headers,
		"body", string(ctx.Response.Body))
	return nil
}

func (f *RequestResponseLogger) Name() string {
	return RequestResponseLoggerFilterName
}
