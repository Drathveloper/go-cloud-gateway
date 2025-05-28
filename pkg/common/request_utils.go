package common

import (
	"io"
)

func ReadBody(readCloser io.ReadCloser) ([]byte, error) {
	if readCloser != nil {
		var err error
		content, err := io.ReadAll(readCloser)
		if err != nil {
			return nil, err
		}
		if err = readCloser.Close(); err != nil {
			return nil, err
		}
		return content, err
	}
	return make([]byte, 0), nil
}
