package gateway

import (
	"fmt"
	"net/http"
	"net/url"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
)

// Request represents a gateway request.
//
// The body of the request is read into memory and stored in the body field.
//
// The body field is nil if the original request body is empty.
type Request struct {
	URL     *url.URL
	Headers http.Header
	Method  string
	Body    []byte
}

// NewGatewayRequest creates a new gateway request from an http request.
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

// Response represents a gateway response.
//
// The body of the response is read into memory and stored in the body field.
//
// The body field is nil if the original response body is empty.
//
// The status field is the HTTP status code of the response.
type Response struct {
	Headers http.Header
	Body    []byte
	Status  int
}

// NewGatewayResponse creates a new gateway response from an http response.
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
