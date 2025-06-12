package ratelimit

import (
	"fmt"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// KeyFunc is a function that returns a key for the rate limiter.
type KeyFunc func(ctx *gateway.Context) string

// KeyFuncBuilder represents a keyFunc builder.
type KeyFuncBuilder interface {
	// The Build method is called to build a keyFunc with the given arguments. The arguments are passed from the keyFunc
	// configuration. The Build method should return an error if the keyFunc cannot be built with the given arguments.
	Build(args map[string]any) (KeyFunc, error)
}

// KeyFuncBuilderFunc is a function that builds a keyFunc.
type KeyFuncBuilderFunc func(args map[string]any) (KeyFunc, error)

// NewIPKeyFuncBuilder builds a new ip key func builder.
func NewIPKeyFuncBuilder() KeyFuncBuilderFunc {
	return func(_ map[string]any) (KeyFunc, error) {
		return NewIPKeyFunc(), nil
	}
}

// NewPathKeyFuncBuilder builds a new path key func builder.
func NewPathKeyFuncBuilder() KeyFuncBuilderFunc {
	return func(_ map[string]any) (KeyFunc, error) {
		return NewPathKeyFunc(), nil
	}
}

// NewPathAndMethodKeyFuncBuilder builds a new path and method key func builder.
func NewPathAndMethodKeyFuncBuilder() KeyFuncBuilderFunc {
	return func(_ map[string]any) (KeyFunc, error) {
		return NewPathAndMethodKeyFunc(), nil
	}
}

// NewHeaderKeyFuncBuilder builds a new header key func builder.
func NewHeaderKeyFuncBuilder() KeyFuncBuilderFunc {
	return func(args map[string]any) (KeyFunc, error) {
		queryParam, err := common.ConvertToString(args["header-name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'header-name' attribute: %w", err)
		}
		return NewHeaderKeyFunc(queryParam), nil
	}
}

// NewQueryKeyFuncBuilder builds a new query param key func builder.
func NewQueryKeyFuncBuilder() KeyFuncBuilderFunc {
	return func(args map[string]any) (KeyFunc, error) {
		queryParam, err := common.ConvertToString(args["query-param"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'query-param' attribute: %w", err)
		}
		return NewQueryKeyFunc(queryParam), nil
	}
}

// Build builds a keyFunc.
func (f KeyFuncBuilderFunc) Build(args map[string]any) (KeyFunc, error) {
	return f(args)
}

const (
	// IPKeyFunc is the registry name of the IP key func.
	IPKeyFunc = "ip"

	// PathKeyFunc is the registry name of the path key func.
	PathKeyFunc = "path"

	// PathMethodKeyFunc is the registry name of the path and method key func.
	PathMethodKeyFunc = "path-method"

	// QueryKeyFunc is the registry name of the query key func.
	QueryKeyFunc = "query"

	// HeaderKeyFunc is the registry name of the header key func.
	HeaderKeyFunc = "header"
)

// NewIPKeyFunc returns the IP address of the request as the key.
func NewIPKeyFunc() KeyFunc {
	return func(ctx *gateway.Context) string {
		return ctx.Request.RemoteAddr
	}
}

// NewPathKeyFunc returns the path of the request as the key.
func NewPathKeyFunc() KeyFunc {
	return func(ctx *gateway.Context) string {
		return ctx.Request.URL.Path
	}
}

// NewPathAndMethodKeyFunc returns the path and method of the request as the key.
func NewPathAndMethodKeyFunc() KeyFunc {
	return func(ctx *gateway.Context) string {
		return ctx.Request.Method + ctx.Request.URL.Path
	}
}

// NewQueryKeyFunc returns the value of the query parameter with the given name as the key.
func NewQueryKeyFunc(queryName string) KeyFunc {
	return func(ctx *gateway.Context) string {
		return ctx.Request.URL.Query().Get(queryName)
	}
}

// NewHeaderKeyFunc returns the value of the header with the given name as the key.
func NewHeaderKeyFunc(headerName string) KeyFunc {
	return func(ctx *gateway.Context) string {
		return ctx.Request.Headers.Get(headerName)
	}
}
