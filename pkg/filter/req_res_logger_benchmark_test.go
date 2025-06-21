package filter_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/url"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

type silentHandler struct{}

func (h *silentHandler) Enabled(_ context.Context, _ slog.Level) bool  { return false }
func (h *silentHandler) Handle(_ context.Context, _ slog.Record) error { return nil }
func (h *silentHandler) WithAttrs(_ []slog.Attr) slog.Handler          { return h }
func (h *silentHandler) WithGroup(_ string) slog.Handler               { return h }

func newSilentLogger() *slog.Logger {
	return slog.New(&silentHandler{})
}

func BenchmarkRequestResponseLogger_SilentPreProcess(b *testing.B) {
	f := filter.NewRequestResponseLoggerFilter(slog.LevelInfo)
	body := []byte(`request body`)
	ctx := &gateway.Context{
		Logger: newSilentLogger(),
		Request: &gateway.Request{
			Method:     "GET",
			URL:        mustParseURL("https://example.com/test"),
			Headers:    map[string][]string{"X-Test": {"true"}},
			BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer(body)), int64(len(body))),
		},
	}

	b.ResetTimer()
	for range b.N {
		_ = f.PreProcess(ctx)
	}
}

func BenchmarkRequestResponseLogger_SilentPostProcess(b *testing.B) {
	f := filter.NewRequestResponseLoggerFilter(slog.LevelInfo)
	body := []byte(`{"status":"ok"}`)
	ctx := &gateway.Context{
		Logger: newSilentLogger(),
		Response: &gateway.Response{
			Status:     200,
			Headers:    map[string][]string{"Content-Type": {"application/json"}},
			BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer(body)), int64(len(body))),
		},
	}

	b.ResetTimer()
	for range b.N {
		_ = f.PostProcess(ctx)
	}
}

func BenchmarkRequestResponseLogger_PreProcess_WithRealLogger(b *testing.B) {
	f := filter.NewRequestResponseLoggerFilter(slog.LevelInfo)
	var buf bytes.Buffer
	logger := newBufferedLogger(&buf)
	body := []byte(`{"key":"value"}`)
	ctx := &gateway.Context{
		Logger: logger,
		Request: &gateway.Request{
			Method:     "POST",
			URL:        mustParseURL("https://example.com/api"),
			Headers:    map[string][]string{"Authorization": {"Bearer xyz"}},
			BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer(body)), int64(len(body))),
		},
	}

	b.ResetTimer()
	for range b.N {
		buf.Reset()
		_ = f.PreProcess(ctx)
	}
}

func BenchmarkRequestResponseLogger_PostProcess_WithRealLogger(b *testing.B) {
	f := filter.NewRequestResponseLoggerFilter(slog.LevelInfo)
	var buf bytes.Buffer
	logger := newBufferedLogger(&buf)
	body := []byte(`{"success":true}`)
	ctx := &gateway.Context{
		Logger: logger,
		Response: &gateway.Response{
			Status:     201,
			Headers:    map[string][]string{"Content-Type": {"application/json"}},
			BodyReader: gateway.NewReplayableBody(io.NopCloser(bytes.NewBuffer(body)), int64(len(body))),
		},
	}

	b.ResetTimer()
	for range b.N {
		buf.Reset()
		_ = f.PostProcess(ctx)
	}
}

func mustParseURL(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return parsed
}

func newBufferedLogger(buf *bytes.Buffer) *slog.Logger {
	handler := slog.NewTextHandler(buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return slog.New(handler)
}
