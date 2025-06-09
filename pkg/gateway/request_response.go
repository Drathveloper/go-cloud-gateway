package gateway

import (
	"fmt"
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
		return nil, fmt.Errorf("build gateway request failed: %w", err)
	}
	return &Request{
		URL:     request.URL,
		Method:  request.Method,
		Headers: request.Header,
		Body:    append([]byte(nil), bodyBytes...),
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
		Body:    append([]byte(nil), bodyBytes...),
	}, nil
}
