package filter

import (
	"fmt"
	"gateway/pkg/common"
	"gateway/pkg/gateway"
)

const (
	AddResponseHeaderFilterName    = "AddResponseHeader"
	SetResponseHeaderFilterName    = "SetResponseHeader"
	RemoveResponseHeaderFilterName = "RemoveResponseHeader"
)

type AddResponseHeader struct {
	headerName  string
	headerValue string
}

func NewAddResponseHeaderFilter(name, value string) *AddResponseHeader {
	return &AddResponseHeader{
		headerName:  name,
		headerValue: value,
	}
}

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

func (f *AddResponseHeader) PreProcess(_ *gateway.Context) error {
	return nil
}

func (f *AddResponseHeader) PostProcess(ctx *gateway.Context) error {
	ctx.Response.Headers.Add(f.headerName, f.headerValue)
	return nil
}

func (f *AddResponseHeader) Name() string {
	return AddResponseHeaderFilterName
}

type SetResponseHeader struct {
	headerName  string
	headerValue string
}

func NewSetResponseHeaderFilter(name, value string) *SetResponseHeader {
	return &SetResponseHeader{
		headerName:  name,
		headerValue: value,
	}
}

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

func (f *SetResponseHeader) PreProcess(_ *gateway.Context) error {
	return nil
}

func (f *SetResponseHeader) PostProcess(ctx *gateway.Context) error {
	ctx.Response.Headers.Set(f.headerName, f.headerValue)
	return nil
}

func (f *SetResponseHeader) Name() string {
	return SetResponseHeaderFilterName
}

type RemoveResponseHeader struct {
	headerName string
}

func NewRemoveResponseHeaderFilter(name string) *RemoveResponseHeader {
	return &RemoveResponseHeader{
		headerName: name,
	}
}

func NewRemoveResponseHeaderBuilder() gateway.FilterBuilder {
	return gateway.FilterBuilderFunc(func(args map[string]any) (gateway.Filter, error) {
		name, err := common.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		return NewRemoveResponseHeaderFilter(name), nil
	})
}

func (f *RemoveResponseHeader) PreProcess(_ *gateway.Context) error {
	return nil
}

func (f *RemoveResponseHeader) PostProcess(ctx *gateway.Context) error {
	ctx.Response.Headers.Del(f.headerName)
	return nil
}

func (f *RemoveResponseHeader) Name() string {
	return RemoveResponseHeaderFilterName
}
