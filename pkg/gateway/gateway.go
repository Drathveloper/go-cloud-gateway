package gateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
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

	readerPool.Put(reader)

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
	backendURL := ctx.Route.GetDestinationURL(ctx.Request.URL)

	reader := readerPool.Get().(*bytes.Reader)
	reader.Reset(ctx.Request.Body)

	req, err := http.NewRequestWithContext(
		ctx, ctx.Request.Method, backendURL, reader)
	if err != nil {
		readerPool.Put(reader)
		return nil, nil, err
	}
	req.Header = ctx.Request.Headers
	return req, reader, nil
}
