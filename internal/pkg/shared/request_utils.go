package shared

import (
	"bytes"
	"io"
	"maps"
	"net"
	"net/http"
	"net/netip"
	"net/textproto"
	"strings"
	"sync"
)

const initialBufferSize = 64 * 1024

const (
	xForwardedForHeader = "X-Forwarded-For"
	xRealIPHeader       = "X-Real-Ip"
)

var (
	//nolint:gochecknoglobals
	bufPool = sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, 0, initialBufferSize))
		},
	}
)

// ReadBody reads the body of a request or a response and returns the body as a byte slice.
func ReadBody(readCloser io.ReadCloser) ([]byte, error) {
	if readCloser == nil {
		return nil, nil
	}
	defer readCloser.Close() //nolint:errcheck

	buf := bufPool.Get().(*bytes.Buffer) //nolint:forcetypeassert
	buf.Reset()
	defer bufPool.Put(buf)

	if _, err := buf.ReadFrom(readCloser); err != nil {
		return nil, err //nolint:wrapcheck
	}

	b := buf.Bytes()
	result := append([]byte(nil), b...)
	return result, nil
}

// WriteHeader writes the header to the response.
func WriteHeader(w http.ResponseWriter, header http.Header) {
	maps.Copy(w.Header(), header)
}

// GetRemoteAddr returns the client IP address of the request. It will make the best effort
// to return the IP address of the client. This is the order followed by the function:
// 1. The first entry of the X-Forwarded-For list, when it is a valid IP.
// 2. The X-Real-Ip value, when it is a valid IP.
// 3. The host part of the connection RemoteAddr.
//
// X-Forwarded-For and X-Real-Ip are client-controlled: unless the gateway runs behind a
// trusted proxy that overwrites them, any client can choose the value returned here.
// Keying rate limits or audit logs on it requires such a proxy in front.
func GetRemoteAddr(request *http.Request) string {
	if ip := firstForwardedIP(request.Header[xForwardedForHeader]); ip != "" {
		return ip
	}
	if values := request.Header[xRealIPHeader]; len(values) != 0 {
		if addr, err := netip.ParseAddr(textproto.TrimString(values[0])); err == nil {
			return addr.String()
		}
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err != nil {
		return request.RemoteAddr
	}
	return host
}

// firstForwardedIP returns the first entry of the X-Forwarded-For list when it is a valid
// IP address. The first entry is the original client; later entries belong to proxies.
func firstForwardedIP(values []string) string {
	if len(values) == 0 {
		return ""
	}
	first, _, _ := strings.Cut(values[0], ",")
	addr, err := netip.ParseAddr(textproto.TrimString(first))
	if err != nil {
		return ""
	}
	return addr.String()
}
