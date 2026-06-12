package httpclient_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/circuitbreaker"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/httpclient"
)

type MockHTTPClient struct {
	ExpectedResponse *http.Response
	ExpectedError    error
}

func (c *MockHTTPClient) Do(_ *http.Request) (*http.Response, error) {
	return c.ExpectedResponse, c.ExpectedError
}

type MockCircuitBreaker[T any] struct {
	ExpectedResult T
	ExpectedError  error
	ObservedError  error
	ExpectedName   string
	ExpectedState  circuitbreaker.State
	ExpectedCounts circuitbreaker.Counts
	ExecuteCalled  bool
}

func (cb *MockCircuitBreaker[T]) Name() string {
	return cb.ExpectedName
}

func (cb *MockCircuitBreaker[T]) State() circuitbreaker.State {
	return cb.ExpectedState
}

func (cb *MockCircuitBreaker[T]) Counts() circuitbreaker.Counts {
	return cb.ExpectedCounts
}

//nolint:ireturn
func (cb *MockCircuitBreaker[T]) Execute(f func() (T, error)) (T, error) {
	cb.ExecuteCalled = true
	res, err := f()
	cb.ObservedError = err
	return res, err
}

func TestCircuitBreakerHTTPClient_Do(t *testing.T) {
	tests := []struct {
		httpClient       gateway.HTTPClient
		circuitBreaker   gateway.CircuitBreaker[*http.Response]
		expectedErr      error
		expectedRes      *http.Response
		name             string
		isGenericContext bool
	}{
		{
			name: "do should succeed when no circuit breaker configured for route",
			httpClient: &MockHTTPClient{
				ExpectedResponse: &http.Response{
					StatusCode: http.StatusOK,
				},
				ExpectedError: nil,
			},
			circuitBreaker: nil,
			expectedRes: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedErr: nil,
		},
		{
			name: "do should succeed when context is not gateway context",
			httpClient: &MockHTTPClient{
				ExpectedResponse: &http.Response{
					StatusCode: http.StatusOK,
				},
				ExpectedError: nil,
			},
			circuitBreaker: nil,
			expectedRes: &http.Response{
				StatusCode: http.StatusOK,
			},
			isGenericContext: true,
			expectedErr:      nil,
		},
		{
			name: "do should succeed when circuit breaker is closed",
			httpClient: &MockHTTPClient{
				ExpectedResponse: &http.Response{
					StatusCode: http.StatusOK,
				},
				ExpectedError: nil,
			},
			circuitBreaker: &MockCircuitBreaker[*http.Response]{
				ExpectedResult: &http.Response{},
				ExpectedError:  nil,
			},
			expectedRes: &http.Response{
				StatusCode: http.StatusOK,
			},
			expectedErr: nil,
		},
		{
			name: "do should return the backend response when request returned 5xx error",
			httpClient: &MockHTTPClient{
				ExpectedResponse: &http.Response{
					StatusCode: http.StatusInternalServerError,
				},
				ExpectedError: nil,
			},
			circuitBreaker: &MockCircuitBreaker[*http.Response]{
				ExpectedResult: &http.Response{},
				ExpectedError:  errors.New("someErr"),
			},
			expectedRes: &http.Response{
				StatusCode: http.StatusInternalServerError,
			},
			expectedErr: nil,
		},
		{
			name: "do should return error when request failed",
			httpClient: &MockHTTPClient{
				ExpectedResponse: nil,
				ExpectedError:    errors.New("someErr"),
			},
			circuitBreaker: &MockCircuitBreaker[*http.Response]{
				ExpectedResult: nil,
				ExpectedError:  errors.New("someErr"),
			},
			expectedRes: nil,
			expectedErr: errors.New("wrapped circuit breaker request failed: someErr"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctx context.Context
			if tt.isGenericContext {
				ctx = t.Context()
			} else {
				ctx = &gateway.Context{
					Route: &gateway.Route{
						ID:             "someId",
						CircuitBreaker: tt.circuitBreaker,
					},
					Context: t.Context(),
				}
			}
			req := &http.Request{}
			req = req.WithContext(ctx)
			cbClient := httpclient.NewCircuitBreakerHTTPClient(tt.httpClient)

			res, err := cbClient.Do(req) //nolint:bodyclose

			if !reflect.DeepEqual(tt.expectedRes, res) {
				t.Errorf("expected response %v actual %v", tt.expectedRes, res)
			}
			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}

type closeCountingBody struct {
	io.Reader

	closes int
}

func (c *closeCountingBody) Close() error {
	c.closes++
	return nil
}

func TestCircuitBreakerHTTPClient_Do_Returns5xxResponseAndCountsFailure(t *testing.T) {
	body := &closeCountingBody{Reader: bytes.NewReader([]byte("backend error detail"))}
	client := &MockHTTPClient{
		ExpectedResponse: &http.Response{
			StatusCode: http.StatusInternalServerError,
			Body:       body,
		},
	}
	breaker := &MockCircuitBreaker[*http.Response]{}
	ctx := &gateway.Context{
		Route: &gateway.Route{
			ID:             "someId",
			CircuitBreaker: breaker,
		},
		Context: t.Context(),
	}
	req := &http.Request{}
	req = req.WithContext(ctx)
	cbClient := httpclient.NewCircuitBreakerHTTPClient(client)

	res, err := cbClient.Do(req) //nolint:bodyclose // the body must stay open for the client

	if err != nil {
		t.Fatalf("expected the 5xx response to be returned, actual error %v", err)
	}
	if res == nil || res.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected the backend 5xx response, actual %v", res)
	}
	if !errors.Is(breaker.ObservedError, httpclient.ErrInternalServer) {
		t.Errorf("expected the breaker to count a failure, actual observed error %v", breaker.ObservedError)
	}
	if body.closes != 0 {
		t.Errorf("expected the 5xx body to stay open for the client, actual closes %d", body.closes)
	}
}

type wrapKey struct{}

func TestCircuitBreakerHTTPClient_Do_AppliesThroughWrappedContext(t *testing.T) {
	breaker := &MockCircuitBreaker[*http.Response]{}
	client := &MockHTTPClient{
		ExpectedResponse: &http.Response{
			StatusCode: http.StatusOK,
		},
	}
	gwCtx := &gateway.Context{
		Route: &gateway.Route{
			ID:             "someId",
			CircuitBreaker: breaker,
		},
		Context: t.Context(),
	}
	req := &http.Request{}
	req = req.WithContext(context.WithValue(gwCtx, wrapKey{}, "value"))
	cbClient := httpclient.NewCircuitBreakerHTTPClient(client)

	res, err := cbClient.Do(req) //nolint:bodyclose

	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status %d", res.StatusCode)
	}
	if !breaker.ExecuteCalled {
		t.Error("expected the circuit breaker to be applied through the wrapped context")
	}
}
