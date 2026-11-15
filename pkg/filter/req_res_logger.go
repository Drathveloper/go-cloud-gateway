package filter

import (
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// RequestResponseLoggerFilterName is the name of the filter.
const RequestResponseLoggerFilterName = "RequestResponseLogger"

// DefaultMaxLoggedBodyBytes is the default maximum number of body bytes the filter buffers
// and logs. Bigger bodies are forwarded untouched and logged without their content.
const DefaultMaxLoggedBodyBytes int64 = 64 * 1024

// redactedHeaderValue is what the filter logs in place of each value of a redacted header.
const redactedHeaderValue = "[REDACTED]"

// RequestResponseLogger is a filter that logs the request and response.
//
// Headers that carry credentials are logged by name only: every value of Authorization,
// Proxy-Authorization, Cookie, Set-Cookie and X-Api-Key is replaced by "[REDACTED]".
// WithRedactedHeaders adds more headers to that set.
type RequestResponseLogger struct {
	// extraRedactedHeaders holds the canonical names added to the built-in set. The
	// built-in names are not stored here so they also apply to the zero value.
	extraRedactedHeaders map[string]struct{}
	maxBodyBytes         int64
	level                slog.Level
	logBodies            bool
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
// The "redact-headers" argument is a list of header names, in any case, whose values are
// redacted in addition to the ones the filter always redacts.
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
		var redactHeaders []string
		if args["redact-headers"] != nil {
			names, err := shared.ConvertToStringSlice(args["redact-headers"])
			if err != nil {
				return nil, fmt.Errorf("failed to convert 'redact-headers' attribute: %w", err)
			}
			redactHeaders = names
		}
		levelStr, _ := shared.ConvertToString(args["level"])
		filter := NewRequestResponseLoggerFilter(parseLogLevel(levelStr), maxBodyBytes, logBodies)
		return filter.WithRedactedHeaders(redactHeaders...), nil
	}
}

// WithRedactedHeaders returns a copy of the filter that also redacts the values of the
// given headers, on top of the ones it already redacts. Names are case-insensitive.
// The receiver is left untouched, so a filter that is already serving requests is safe.
func (f *RequestResponseLogger) WithRedactedHeaders(names ...string) *RequestResponseLogger {
	extra := make(map[string]struct{}, len(f.extraRedactedHeaders)+len(names))
	maps.Copy(extra, f.extraRedactedHeaders)
	for _, name := range names {
		extra[http.CanonicalHeaderKey(strings.TrimSpace(name))] = struct{}{}
	}
	return &RequestResponseLogger{
		extraRedactedHeaders: extra,
		maxBodyBytes:         f.maxBodyBytes,
		level:                f.level,
		logBodies:            f.logBodies,
	}
}

// PreProcess logs the request.
func (f *RequestResponseLogger) PreProcess(ctx *gateway.Context) error {
	if !ctx.Logger.Enabled(ctx, f.level) {
		return nil
	}
	// The live map is the one forwarded to the backend, so it is never modified: the
	// log gets a copy when, and only when, there is something to redact.
	headers := f.redactedCopy(ctx.Request.Headers)
	if !f.logBodies {
		ctx.Logger.Log(ctx, f.level, "Received request",
			"url", ctx.Request.Method+" "+ctx.Request.URL.String(),
			"headers", headers)
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
		"headers", headers,
		"body", loggableBody(body))
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
			"headers", f.redactedCopy(ctx.Response.Headers))
		return nil
	}
	// Snapshot everything logged from onDone now: the callback fires later, from
	// writeResponse, after the hop-by-hop headers have been stripped from the live map
	// and while the only safe references are the ones captured here.
	logger := ctx.Logger
	status := ctx.Response.Status
	// The snapshot is private to the log line, so it is redacted in place.
	headers := f.redact(ctx.Response.Headers.Clone())
	var body []byte
	ctx.Response.BodyReader.ObserveStream(
		func(chunk []byte) {
			body = appendUpToLimit(body, chunk, f.maxBodyBytes)
		},
		func(total int64, err error) {
			if int64(len(body)) < total || err != nil {
				// The logged body is a prefix cut at an arbitrary byte, by the limit or
				// by the stream ending early. Without this, text cut in the middle of a
				// character would be logged as bytes.
				body = trimIncompleteRune(body)
			}
			attrs := []any{"status", status, "headers", headers, "body", loggableBody(body), "bytes", total}
			if err != nil {
				attrs = append(attrs, "error", err)
			}
			logger.Log(ctx, f.level, "Returned response", attrs...)
		},
	)
	return nil
}

// appendUpToLimit appends chunk to body, capping the total at maxBytes bytes. A negative
// maxBytes means no cap. It keeps the logged prefix bounded while the full body still
// streams to the client untouched.
func appendUpToLimit(body, chunk []byte, maxBytes int64) []byte {
	if maxBytes < 0 {
		return append(body, chunk...)
	}
	remaining := maxBytes - int64(len(body))
	if remaining <= 0 {
		return body
	}
	if int64(len(chunk)) > remaining {
		chunk = chunk[:remaining]
	}
	return append(body, chunk...)
}

// Name returns the name of the filter.
func (f *RequestResponseLogger) Name() string {
	return RequestResponseLoggerFilterName
}

// redactedCopy returns headers ready to be logged without modifying them. When no header
// is sensitive it returns the same map and copies nothing. Otherwise it returns a shallow
// copy with the sensitive values replaced: the other headers still share their values
// with the original, so the result must be logged before the original can change.
func (f *RequestResponseLogger) redactedCopy(headers http.Header) http.Header {
	for name := range headers {
		if f.isRedacted(name) {
			return f.redact(maps.Clone(headers))
		}
	}
	return headers
}

// redact replaces every value of the sensitive headers and returns the same map. The
// values are replaced with new slices rather than overwritten, so slices shared with
// another map are left intact; the map itself must belong to the caller.
func (f *RequestResponseLogger) redact(headers http.Header) http.Header {
	for name, values := range headers {
		if !f.isRedacted(name) {
			continue
		}
		redacted := make([]string, len(values))
		for i := range redacted {
			redacted[i] = redactedHeaderValue
		}
		headers[name] = redacted
	}
	return headers
}

// isRedacted reports whether the values of the named header must not be logged. The name
// is canonicalized first because a filter can write a key to the map in any case.
func (f *RequestResponseLogger) isRedacted(name string) bool {
	name = http.CanonicalHeaderKey(name)
	switch name {
	case "Authorization", "Proxy-Authorization", "Cookie", "Set-Cookie", "X-Api-Key":
		return true
	}
	_, redacted := f.extraRedactedHeaders[name]
	return redacted
}

// parseLogLevel returns the level named by level, in any case, or info when it names none.
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// loggableBody returns the body in the form it is logged. Text is logged as a string:
// slog's JSON handler encodes a []byte in base64, which makes a text body unreadable.
// A body that is not valid UTF-8 stays a []byte: as a string its invalid bytes would be
// replaced by the handler and could no longer be recovered from the log. A missing body
// is logged as the empty string, so the attribute has the same type with or without one.
func loggableBody(body []byte) any {
	if utf8.Valid(body) {
		return string(body)
	}
	return body
}

// trimIncompleteRune removes a UTF-8 sequence left unfinished at the end of body.
// Anything else, including invalid bytes, is left as it is.
func trimIncompleteRune(body []byte) []byte {
	for size := 1; size < utf8.UTFMax && size <= len(body); size++ {
		start := len(body) - size
		if !utf8.RuneStart(body[start]) {
			continue
		}
		if !utf8.FullRune(body[start:]) {
			return body[:start]
		}
		break
	}
	return body
}
