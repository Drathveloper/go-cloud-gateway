package shared

import (
	"net/http"
	"net/textproto"
	"strings"
)

// hopHeaders are the well-known hop-by-hop headers (RFC 2616 section 13.5.1).
// They are meaningful only for a single transport connection and must not be
// forwarded by a proxy: forwarding e.g. "Connection: close" tears down the
// keep-alive of the next hop on every request.
//
//nolint:gochecknoglobals
var hopHeaders = []string{
	"Connection",
	"Proxy-Connection",
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",
	"Trailer",
	"Transfer-Encoding",
	"Upgrade",
}

// RemoveHopByHopHeaders removes the hop-by-hop headers from the given header map:
// first the ones nominated by the Connection header (RFC 7230 section 6.1), then
// the well-known set. The map is modified in place.
func RemoveHopByHopHeaders(header http.Header) {
	for _, value := range header["Connection"] {
		for name := range strings.SplitSeq(value, ",") {
			if name = textproto.TrimString(name); name != "" {
				header.Del(name)
			}
		}
	}
	for _, name := range hopHeaders {
		header.Del(name)
	}
}
