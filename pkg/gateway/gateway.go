package gateway

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
)

var ErrHTTP = errors.New("gateway http request to backend failed")

type HTTPClient interface {
	Do(r *http.Request) (*http.Response, error)
}

const gatewayErrMsg = "gateway request for route %s failed: %w"

type Gateway struct {
	globalFilters Filters
	httpClient    HTTPClient
}

func NewGateway(
	globalFilters Filters,
	client HTTPClient) *Gateway {
	return &Gateway{
		globalFilters: globalFilters,
		httpClient:    client,
	}
}

func (g *Gateway) Do(ctx *Context) error {
	ctx.Logger.With().Info("started routing request", "routeId", ctx.Route.ID)
	defer ctx.Logger.Info("finished routing request", "routeId", ctx.Route.ID)
	allFilters := ctx.Route.CombineGlobalFilters(g.globalFilters...)
	if err := allFilters.PreProcessAll(ctx); err != nil {
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, err)
	}
	backendReq, err := g.buildProxyRequest(ctx)
	if err != nil {
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, err)
	}
	backendRes, err := g.httpClient.Do(backendReq)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled):
			return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, context.DeadlineExceeded)
		default:
			return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, fmt.Errorf("%w: %s", ErrHTTP, err))
		}
	}
	gwRes, err := NewGatewayResponse(backendRes)
	if err != nil {
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, err)
	}
	ctx.Response = gwRes
	if err = allFilters.PostProcessAll(ctx); err != nil {
		return fmt.Errorf(gatewayErrMsg, ctx.Route.ID, err)
	}
	return nil
}

func (g *Gateway) buildProxyRequest(ctx *Context) (*http.Request, error) {
	backendURL := ctx.Route.GetDestinationURLStr(ctx.Request.URL)
	req, err := http.NewRequestWithContext(
		ctx, ctx.Request.Method, backendURL, bytes.NewBuffer(ctx.Request.Body))
	if err != nil {
		return nil, err
	}
	maps.Copy(req.Header, ctx.Request.Headers)
	return req, nil
}
