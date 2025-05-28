package gateway

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/drathveloper/go-cloud-gateway/pkg/common"
)

type Request struct {
	URL     *url.URL
	Method  string
	Headers http.Header
	Body    []byte
}

func NewGatewayRequest(request *http.Request) (*Request, error) {
	bodyBytes, err := common.ReadBody(request.Body)
	if err != nil {
		return nil, fmt.Errorf("build gateway response failed: %w", err)
	}
	request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	return &Request{
		URL:     request.URL,
		Method:  request.Method,
		Headers: request.Header,
		Body:    bodyBytes,
	}, nil
}

type Response struct {
	Status  int
	Headers http.Header
	Body    []byte
}

func NewGatewayResponse(response *http.Response) (*Response, error) {
	bodyBytes, err := common.ReadBody(response.Body)
	if err != nil {
		return nil, fmt.Errorf("build gateway response failed: %w", err)
	}
	return &Response{
		Status:  response.StatusCode,
		Headers: response.Header,
		Body:    bodyBytes,
	}, nil
}
