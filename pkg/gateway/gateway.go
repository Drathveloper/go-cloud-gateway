package gateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
)

var ErrHTTP = errors.New("gateway http request to backend failed")

type HTTPClient interface {
	Do(r *http.Request) (*http.Response, error)
}

const gatewayErrMsg = "gateway request for route %s failed: %w"

var readerPool = sync.Pool{
	New: func() any { return new(bytes.Reader) },
}

type Gateway struct {
	globalFilters Filters
	httpClient    HTTPClient
}

func NewGateway(globalFilters Filters, client HTTPClient) *Gateway {
	return &Gateway{
		globalFilters: globalFilters,
		httpClient:    client,
	}
}

func (g *Gateway) Do(ctx *Context) error {
	allFilters := ctx.Route.CombineGlobalFilters(g.globalFilters...)
	if err := allFilters.PreProcessAll(ctx); err != nil {
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, err)
	}
	backendReq, reader, err := g.buildProxyRequest(ctx)
	if err != nil {
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, err)
	}
	backendRes, err := g.httpClient.Do(backendReq)

	if reader != nil {
		readerPool.Put(reader)
	}

	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
			return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, context.DeadlineExceeded)
		default:
			return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, fmt.Errorf("%w: %s", ErrHTTP, err))
		}
	}
	ctx.Response, err = NewGatewayResponse(backendRes)
	if err != nil {
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, err)
	}
	if err = allFilters.PostProcessAll(ctx); err != nil {
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, err)
	}
	return nil
}

func (g *Gateway) buildProxyRequest(ctx *Context) (*http.Request, *bytes.Reader, error) {
	if len(ctx.Request.Body) == 0 {
		return g.buildNoBodyProxyRequest(ctx), nil, nil
	}
	return g.buildBodyProxyRequest(ctx)
}

func (g *Gateway) buildBodyProxyRequest(ctx *Context) (*http.Request, *bytes.Reader, error) {
	reader := readerPool.Get().(*bytes.Reader)
	reader.Reset(ctx.Request.Body)
	req := &http.Request{
		ContentLength: int64(len(ctx.Request.Body)),
		Method:        ctx.Request.Method,
		URL:           ctx.Route.GetDestinationURL(ctx.Request.URL),
		Header:        ctx.Request.Headers,
		Body:          io.NopCloser(reader),
	}
	return req.WithContext(ctx), reader, nil
}

func (g *Gateway) buildNoBodyProxyRequest(ctx *Context) *http.Request {
	req := &http.Request{
		ContentLength: 0,
		Method:        ctx.Request.Method,
		URL:           ctx.Route.GetDestinationURL(ctx.Request.URL),
		Header:        ctx.Request.Headers,
		Body:          http.NoBody,
	}
	return req.WithContext(ctx)
}
