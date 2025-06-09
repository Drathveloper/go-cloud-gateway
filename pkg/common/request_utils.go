package common

import (
	"bytes"
	"io"
	"net/http"
	"sync"
)

const (
	initialBufferSize = 64 * 1024
)

var (
	//nolint:gochecknoglobals
	bufPool = sync.Pool{
		New: func() any {
			return bytes.NewBuffer(make([]byte, 0, initialBufferSize))
		},
	}
)

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

func WriteHeader(w http.ResponseWriter, header http.Header) {
	dst := w.Header()
	for k, values := range header {
		dst[k] = values
	}
}
