package filter

import (
	"fmt"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

const (
	// AddResponseHeaderFilterName is the name of the filter.
	AddResponseHeaderFilterName = "AddResponseHeader"

	// SetResponseHeaderFilterName is the name of the filter.
	SetResponseHeaderFilterName = "SetResponseHeader"

	// RemoveResponseHeaderFilterName is the name of the filter.
	RemoveResponseHeaderFilterName = "RemoveResponseHeader"
)

// AddResponseHeader is a filter that adds a header to the response.
type AddResponseHeader struct {
	headerName  string
	headerValue string
}

// NewAddResponseHeaderFilter creates a new AddResponseHeaderFilter.
func NewAddResponseHeaderFilter(name, value string) *AddResponseHeader {
	return &AddResponseHeader{
		headerName:  name,
		headerValue: value,
	}
}

// NewAddResponseHeaderBuilder creates a new AddResponseHeaderBuilder.
func NewAddResponseHeaderBuilder() gateway.FilterBuilder {
	return gateway.FilterBuilderFunc(func(args map[string]any) (gateway.Filter, error) {
		name, err := common.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		value, err := common.ConvertToString(args["value"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'value' attribute: %w", err)
		}
		return NewAddResponseHeaderFilter(name, value), nil
	})
}

// PreProcess does nothing.
func (f *AddResponseHeader) PreProcess(_ *gateway.Context) error {
	return nil
}

// PostProcess adds the header to the response.
func (f *AddResponseHeader) PostProcess(ctx *gateway.Context) error {
	ctx.Response.Headers.Add(f.headerName, f.headerValue)
	return nil
}

// Name returns the name of the filter.
func (f *AddResponseHeader) Name() string {
	return AddResponseHeaderFilterName
}

// SetResponseHeader is a filter that sets a header in the response.
type SetResponseHeader struct {
	headerName  string
	headerValue string
}

// NewSetResponseHeaderFilter creates a new SetResponseHeaderFilter.
func NewSetResponseHeaderFilter(name, value string) *SetResponseHeader {
	return &SetResponseHeader{
		headerName:  name,
		headerValue: value,
	}
}

// NewSetResponseHeaderBuilder creates a new SetResponseHeaderBuilder.
func NewSetResponseHeaderBuilder() gateway.FilterBuilder {
	return gateway.FilterBuilderFunc(func(args map[string]any) (gateway.Filter, error) {
		name, err := common.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		value, err := common.ConvertToString(args["value"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'value' attribute: %w", err)
		}
		return NewSetResponseHeaderFilter(name, value), nil
	})
}

// PreProcess does nothing.
func (f *SetResponseHeader) PreProcess(_ *gateway.Context) error {
	return nil
}

// PostProcess sets the header in the response.
func (f *SetResponseHeader) PostProcess(ctx *gateway.Context) error {
	ctx.Response.Headers.Set(f.headerName, f.headerValue)
	return nil
}

// Name returns the name of the filter.
func (f *SetResponseHeader) Name() string {
	return SetResponseHeaderFilterName
}

// RemoveResponseHeader is a filter that removes a header from the response.
type RemoveResponseHeader struct {
	headerName string
}

// NewRemoveResponseHeaderFilter creates a new RemoveResponseHeaderFilter.
func NewRemoveResponseHeaderFilter(name string) *RemoveResponseHeader {
	return &RemoveResponseHeader{
		headerName: name,
	}
}

// NewRemoveResponseHeaderBuilder creates a new RemoveResponseHeaderBuilder.
func NewRemoveResponseHeaderBuilder() gateway.FilterBuilder {
	return gateway.FilterBuilderFunc(func(args map[string]any) (gateway.Filter, error) {
		name, err := common.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		return NewRemoveResponseHeaderFilter(name), nil
	})
}

// PreProcess does nothing.
func (f *RemoveResponseHeader) PreProcess(_ *gateway.Context) error {
	return nil
}

// PostProcess removes the header from the response.
func (f *RemoveResponseHeader) PostProcess(ctx *gateway.Context) error {
	ctx.Response.Headers.Del(f.headerName)
	return nil
}

// Name returns the name of the filter.
func (f *RemoveResponseHeader) Name() string {
	return RemoveResponseHeaderFilterName
}
