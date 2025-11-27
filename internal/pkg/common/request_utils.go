package common

import (
	"bytes"
	"io"
	"maps"
	"net/http"
	"strings"
	"sync"
)

const initialBufferSize = 64 * 1024

const (
	xForwardedForHeader = "X-Forwarded-For"
	xRealIPHeader       = "X-Real-Ip"
	localIPAddr         = "127.0.0.1"
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

// GetRemoteAddr returns the remote address of the request. It will make the best effort to return the IP address
// of the client. This is the order followed by the function to get the IP address of the client:
// 1. Check X-Forwarded-For header, return if present.
// 2. Check X-Real-Ip header, return if present.
// 3. Return request RemoteAddr attribute trimming port.
func GetRemoteAddr(request *http.Request) string {
	if len(request.Header[xForwardedForHeader]) != 0 {
		return request.Header[xForwardedForHeader][0]
	}
	if len(request.Header[xRealIPHeader]) != 0 {
		return request.Header[xRealIPHeader][0]
	}
	if strings.HasPrefix(request.RemoteAddr, "[::1]") {
		return localIPAddr
	}
	return strings.Split(request.RemoteAddr, ":")[0]
}
