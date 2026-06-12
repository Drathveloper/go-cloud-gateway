package shared

import (
	"net"
	"net/http"
	"strings"
)

const (
	xForwardedHostHeader  = "X-Forwarded-Host"
	xForwardedProtoHeader = "X-Forwarded-Proto"
)

// SetXForwardedHeaders sets the X-Forwarded-For, X-Forwarded-Host and X-Forwarded-Proto
// headers on the request, in place, so the backend can identify the original client.
// It mirrors the semantics of net/http/httputil (*ProxyRequest).SetXForwarded:
//   - The connection peer IP is appended to any X-Forwarded-For list sent by the client,
//     preserving the chain; when the peer cannot be parsed the header is dropped instead,
//     so the backend never receives a client-controlled list presented as gateway-made.
//   - X-Forwarded-Host and X-Forwarded-Proto are overwritten: their inbound values are
//     client-controlled and must not cross the proxy. A trusted proxy in front of the
//     gateway is therefore not preserved; backends must trust the gateway values.
func SetXForwardedHeaders(request *http.Request) {
	if clientIP, _, err := net.SplitHostPort(request.RemoteAddr); err == nil {
		if prior := request.Header[xForwardedForHeader]; len(prior) > 0 {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		request.Header.Set(xForwardedForHeader, clientIP)
	} else {
		request.Header.Del(xForwardedForHeader)
	}
	request.Header.Set(xForwardedHostHeader, request.Host)
	if request.TLS == nil {
		request.Header.Set(xForwardedProtoHeader, "http")
	} else {
		request.Header.Set(xForwardedProtoHeader, "https")
	}
}
