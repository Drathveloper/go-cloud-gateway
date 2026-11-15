package filter

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"slices"
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

// ErrInvalidLogLevel is returned when the 'level' argument of the filter names no log level.
var ErrInvalidLogLevel = errors.New("invalid log level")

// redactedValue is what the filter logs in place of each value of a redacted header, of
// each value of a redacted query parameter and of the user info of the URL.
const redactedValue = "[REDACTED]"

// RequestResponseLogger is a filter that logs the request and response.
//
// Headers that carry credentials are logged by name only: every value of Authorization,
// Proxy-Authorization, Cookie, Set-Cookie and X-Api-Key is replaced by "[REDACTED]".
// WithRedactedHeaders adds more headers to that set.
//
// The URL is logged with the credentials it carries replaced the same way: the values of
// the query parameters access_token, id_token, refresh_token, token, api_key, apikey,
// password, secret, client_secret and signature, and its user info. The name of the
// parameter and the rest of the URL are logged as they were received.
// WithRedactedQueryParams adds more parameters to that set.
//
// Only what is logged changes: the request sent to the backend keeps its headers and its
// query string.
type RequestResponseLogger struct {
	// extraRedactedHeaders holds the canonical names added to the built-in set. The
	// built-in names are not stored here so they also apply to the zero value.
	extraRedactedHeaders map[string]struct{}
	// extraRedactedQueryParams holds the names added to the built-in set, which is not
	// stored here for the same reason.
	extraRedactedQueryParams []string
	maxBodyBytes             int64
	level                    slog.Level
	logBodies                bool
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
// The "level" argument selects the log level (debug, info, warn or error, in any case;
// default info). Any other value is an error.
// The "max-body-bytes" argument caps how many body bytes are buffered in memory and logged
// per request and response (default DefaultMaxLoggedBodyBytes; negative means unlimited).
// The "log-bodies" argument selects whether bodies are logged at all (default true).
// Disabling it skips the body capture entirely, so no body is buffered or copied.
// The "redact-headers" argument is a list of header names, in any case, whose values are
// redacted in addition to the ones the filter always redacts.
// The "redact-query-params" argument is a list of query parameter names, in any case, whose
// values are redacted in the logged URL in addition to the ones the filter always redacts.
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
		redactHeaders, err := namesArg(args, "redact-headers")
		if err != nil {
			return nil, err
		}
		redactQueryParams, err := namesArg(args, "redact-query-params")
		if err != nil {
			return nil, err
		}
		level, err := logLevelArg(args)
		if err != nil {
			return nil, err
		}
		filter := NewRequestResponseLoggerFilter(level, maxBodyBytes, logBodies)
		return filter.WithRedactedHeaders(redactHeaders...).WithRedactedQueryParams(redactQueryParams...), nil
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
	derived := *f
	derived.extraRedactedHeaders = extra
	return &derived
}

// WithRedactedQueryParams returns a copy of the filter that also redacts, in the logged
// URL, the values of the given query parameters, on top of the ones it already redacts.
// Names are given as the backend reads them, without percent-encoding ("api key", not
// "api%20key" or "api+key"), and their ASCII letters are case-insensitive.
// The receiver is left untouched, so a filter that is already serving requests is safe.
func (f *RequestResponseLogger) WithRedactedQueryParams(names ...string) *RequestResponseLogger {
	extra := make([]string, 0, len(f.extraRedactedQueryParams)+len(names))
	extra = append(extra, f.extraRedactedQueryParams...)
	for _, name := range names {
		name = strings.TrimSpace(name)
		if !slices.Contains(extra, name) {
			extra = append(extra, name)
		}
	}
	derived := *f
	derived.extraRedactedQueryParams = extra
	return &derived
}

// PreProcess logs the request.
func (f *RequestResponseLogger) PreProcess(ctx *gateway.Context) error {
	if !ctx.Logger.Enabled(ctx, f.level) {
		return nil
	}
	// The live map is the one forwarded to the backend, so it is never modified: the
	// log gets a copy when, and only when, there is something to redact.
	headers := f.redactedCopy(ctx.Request.Headers)
	// The same goes for the URL: the request to the backend is built from the live one.
	target := ctx.Request.Method + " " + f.loggedURL(ctx.Request.URL)
	if !f.logBodies {
		ctx.Logger.Log(ctx, f.level, "Received request",
			"url", target,
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
		"url", target,
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
			redacted[i] = redactedValue
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

// loggedURL returns the URL in the form it is logged, without modifying it: the values of
// the sensitive query parameters and the user info are replaced. Everything else is
// written as url.URL.String writes it.
//
// A server request only has user info when its request line holds an absolute URL
// ("GET http://user:password@host/path HTTP/1.1"). All of it is replaced, not only the
// password, because a token is often sent as the user name.
func (f *RequestResponseLogger) loggedURL(reqURL *url.URL) string {
	query := f.redactedQuery(reqURL.RawQuery)
	// String does not write the user info of an opaque URL.
	hasUserInfo := reqURL.User != nil && reqURL.Opaque == ""
	if !hasUserInfo && query == reqURL.RawQuery {
		return reqURL.String()
	}
	logged := *reqURL
	logged.RawQuery = query
	if !hasUserInfo {
		return logged.String()
	}
	// The mark cannot be set as the user name, because String would percent-encode its
	// brackets. An empty user info is written as a lone '@', which is the first one of the
	// URL because nothing before the host can hold another, and the mark goes before it.
	logged.User = url.User("")
	beforeUserInfo, afterUserInfo, _ := strings.Cut(logged.String(), "@")
	return beforeUserInfo + redactedValue + "@" + afterUserInfo
}

// redactedQuery returns the raw query string with the value of every sensitive parameter
// replaced. When there is none it returns the same string and allocates nothing.
//
// The query is scanned as it was received rather than parsed and encoded again, which
// would sort the parameters and normalize their encoding: the order, the repeated
// parameters, the ones without a value and the malformed ones are all kept. A parameter
// ends at '&' or ';', its name ends at the first '=' and the rest, which can hold more
// '=', is its value. An empty value is left as it is, since there is nothing to hide.
func (f *RequestResponseLogger) redactedQuery(query string) string {
	var redacted strings.Builder
	// copied is how much of the query has been written to redacted so far.
	copied := 0
	for start := 0; start < len(query); {
		end := start
		for end < len(query) && !isQuerySeparator(query[end]) {
			end++
		}
		name, value, _ := strings.Cut(query[start:end], "=")
		if value != "" && f.isRedactedQueryParam(name) {
			if copied == 0 {
				// Enough for the usual case, a value that is longer than what replaces it,
				// and for one that is shorter. More of them only make the buffer grow.
				redacted.Grow(len(query) + len(redactedValue))
			}
			redacted.WriteString(query[copied : end-len(value)])
			redacted.WriteString(redactedValue)
			copied = end
		}
		start = end + 1
	}
	if copied == 0 {
		return query
	}
	redacted.WriteString(query[copied:])
	return redacted.String()
}

// isQuerySeparator reports whether char ends a parameter of the query string. Go only
// splits on '&', but other servers also split on ';', so a credential behind either one
// has to be found.
func isQuerySeparator(char byte) bool {
	return char == '&' || char == ';'
}

// isRedactedQueryParam reports whether the values of the query parameter must not be
// logged. The name is given as it is written in the URL.
func (f *RequestResponseLogger) isRedactedQueryParam(rawName string) bool {
	builtIn := [...]string{
		"access_token", "id_token", "refresh_token", "token", "api_key", "apikey",
		"password", "secret", "client_secret", "signature",
	}
	// Only an escape makes a name longer than what it decodes to. This runs for every
	// parameter of every logged request, and nearly all of them have no escape and are
	// ruled out by their length, without being compared.
	escaped := strings.IndexByte(rawName, '%') >= 0
	return isAnyQueryParamName(rawName, builtIn[:], escaped) ||
		isAnyQueryParamName(rawName, f.extraRedactedQueryParams, escaped)
}

// isAnyQueryParamName reports whether raw, the name of a query parameter as it is written
// in the URL, is one of names once decoded. The caller says whether raw has an escape.
func isAnyQueryParamName(raw string, names []string, escaped bool) bool {
	for _, name := range names {
		if len(raw) < len(name) || !escaped && len(raw) != len(name) {
			continue
		}
		if isQueryParamName(raw, name) {
			return true
		}
	}
	return false
}

// isQueryParamName reports whether raw, the name of a query parameter as it is written in
// the URL, is name once decoded, ignoring the case of the ASCII letters.
//
// The name is decoded because the backend decodes it: "access%5Ftoken" and "%74oken" are
// access_token and token to it. It is decoded while it is compared, so nothing is
// allocated, and leniently: an escape that is not valid is taken literally, as some
// servers do, where url.QueryUnescape would fail.
func isQueryParamName(raw, name string) bool {
	matched := 0
	for len(raw) > 0 {
		char, width := decodeQueryByte(raw)
		if matched == len(name) || lowerASCII(char) != lowerASCII(name[matched]) {
			return false
		}
		matched++
		raw = raw[width:]
	}
	return matched == len(name)
}

// decodeQueryByte decodes the first byte of text, which must not be empty and is part of
// a query string. It returns the byte and how many bytes of text encode it: three for a
// valid percent escape, one for anything else, including a '+', which stands for a space.
func decodeQueryByte(text string) (byte, int) {
	const (
		escapeWidth     = 3
		bitsPerHexDigit = 4
	)
	switch {
	case text[0] == '+':
		return ' ', 1
	case text[0] == '%' && len(text) >= escapeWidth:
		high, isHighHex := hexValue(text[1])
		low, isLowHex := hexValue(text[2])
		if isHighHex && isLowHex {
			return high<<bitsPerHexDigit | low, escapeWidth
		}
	}
	return text[0], 1
}

// hexValue returns the value of a hexadecimal digit and whether char is one.
func hexValue(char byte) (byte, bool) {
	const firstLetterValue = 10
	switch {
	case '0' <= char && char <= '9':
		return char - '0', true
	case 'a' <= char && char <= 'f':
		return char - 'a' + firstLetterValue, true
	case 'A' <= char && char <= 'F':
		return char - 'A' + firstLetterValue, true
	default:
		return 0, false
	}
}

// lowerASCII returns the lower case of an ASCII letter and any other byte as it is.
func lowerASCII(char byte) byte {
	if 'A' <= char && char <= 'Z' {
		return char + ('a' - 'A')
	}
	return char
}

// namesArg reads an argument that holds a list of names. A missing argument is no names.
func namesArg(args map[string]any, key string) ([]string, error) {
	if args[key] == nil {
		return nil, nil
	}
	names, err := shared.ConvertToStringSlice(args[key])
	if err != nil {
		return nil, fmt.Errorf("failed to convert '%s' attribute: %w", key, err)
	}
	return names, nil
}

// logLevelArg reads the 'level' argument. A missing level means info. Any other value has
// to name a level: a typo such as "warning" used to be read as info without any notice,
// which logs at a level nobody asked for.
func logLevelArg(args map[string]any) (slog.Level, error) {
	if args["level"] == nil {
		return slog.LevelInfo, nil
	}
	name, err := shared.ConvertToString(args["level"])
	if err != nil {
		return slog.LevelInfo, fmt.Errorf("failed to convert 'level' attribute: %w", err)
	}
	switch strings.ToLower(name) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("%w %q: expected debug, info, warn or error", ErrInvalidLogLevel, name)
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
