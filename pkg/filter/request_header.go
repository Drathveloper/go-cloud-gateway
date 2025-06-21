package filter

import (
	"fmt"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

const (
	// AddRequestHeaderFilterName is the name of the filter.
	AddRequestHeaderFilterName = "AddRequestHeader"

	// SetRequestHeaderFilterName is the name of the filter.
	SetRequestHeaderFilterName = "SetRequestHeader"

	// RemoveRequestHeaderFilterName is the name of the filter.
	RemoveRequestHeaderFilterName = "RemoveRequestHeader"
)

// AddRequestHeader is a filter that adds a header to the request.
type AddRequestHeader struct {
	headerName  string
	headerValue string
}

// NewAddRequestHeaderFilter creates a new AddRequestHeaderFilter.
func NewAddRequestHeaderFilter(name, value string) *AddRequestHeader {
	return &AddRequestHeader{
		headerName:  name,
		headerValue: value,
	}
}

// NewAddRequestHeaderBuilder creates a new AddRequestHeaderBuilder.
func NewAddRequestHeaderBuilder() gateway.FilterBuilderFunc {
	return func(args map[string]any) (gateway.Filter, error) {
		name, err := common.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		value, err := common.ConvertToString(args["value"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'value' attribute: %w", err)
		}
		return NewAddRequestHeaderFilter(name, value), nil
	}
}

// PreProcess adds the header to the request.
func (f *AddRequestHeader) PreProcess(ctx *gateway.Context) error {
	ctx.Request.Headers.Add(f.headerName, f.headerValue)
	return nil
}

// PostProcess does nothing.
func (f *AddRequestHeader) PostProcess(_ *gateway.Context) error {
	return nil
}

// Name returns the name of the filter.
func (f *AddRequestHeader) Name() string {
	return AddRequestHeaderFilterName
}

// SetRequestHeader is a filter that sets a header in the request.
type SetRequestHeader struct {
	headerName  string
	headerValue string
}

// NewSetRequestHeaderBuilder creates a new SetRequestHeaderBuilder.
func NewSetRequestHeaderBuilder() gateway.FilterBuilderFunc {
	return func(args map[string]any) (gateway.Filter, error) {
		name, err := common.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		value, err := common.ConvertToString(args["value"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'value' attribute: %w", err)
		}
		return NewSetRequestHeaderFilter(name, value), nil
	}
}

// NewSetRequestHeaderFilter creates a new SetRequestHeaderFilter.
func NewSetRequestHeaderFilter(name, value string) *SetRequestHeader {
	return &SetRequestHeader{
		headerName:  name,
		headerValue: value,
	}
}

// PreProcess sets the header in the request.
func (f *SetRequestHeader) PreProcess(ctx *gateway.Context) error {
	ctx.Request.Headers.Set(f.headerName, f.headerValue)
	return nil
}

// PostProcess does nothing.
func (f *SetRequestHeader) PostProcess(_ *gateway.Context) error {
	return nil
}

// Name returns the name of the filter.
func (f *SetRequestHeader) Name() string {
	return SetRequestHeaderFilterName
}

// RemoveRequestHeader is a filter that removes a header from the request.
type RemoveRequestHeader struct {
	headerName string
}

// NewRemoveRequestHeaderFilter creates a new RemoveRequestHeaderFilter.
func NewRemoveRequestHeaderFilter(name string) *RemoveRequestHeader {
	return &RemoveRequestHeader{
		headerName: name,
	}
}

// NewRemoveRequestHeaderBuilder creates a new RemoveRequestHeaderBuilder.
func NewRemoveRequestHeaderBuilder() gateway.FilterBuilderFunc {
	return func(args map[string]any) (gateway.Filter, error) {
		name, err := common.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		return NewRemoveRequestHeaderFilter(name), nil
	}
}

// PreProcess removes the header from the request.
func (f *RemoveRequestHeader) PreProcess(ctx *gateway.Context) error {
	ctx.Request.Headers.Del(f.headerName)
	return nil
}

// PostProcess does nothing.
func (f *RemoveRequestHeader) PostProcess(_ *gateway.Context) error {
	return nil
}

// Name returns the name of the filter.
func (f *RemoveRequestHeader) Name() string {
	return RemoveRequestHeaderFilterName
}
