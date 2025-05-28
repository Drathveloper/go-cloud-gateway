package filter

import (
	"fmt"

	"github.com/drathveloper/go-cloud-gateway/pkg/common"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

const (
	AddRequestHeaderFilterName    = "AddRequestHeader"
	SetRequestHeaderFilterName    = "SetRequestHeader"
	RemoveRequestHeaderFilterName = "RemoveRequestHeader"
)

type AddRequestHeader struct {
	headerName  string
	headerValue string
}

func NewAddRequestHeaderFilter(name, value string) *AddRequestHeader {
	return &AddRequestHeader{
		headerName:  name,
		headerValue: value,
	}
}

func NewAddRequestHeaderBuilder() gateway.FilterBuilder {
	return gateway.FilterBuilderFunc(func(args map[string]any) (gateway.Filter, error) {
		name, err := common.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		value, err := common.ConvertToString(args["value"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'value' attribute: %w", err)
		}
		return NewAddRequestHeaderFilter(name, value), nil
	})
}

func (f *AddRequestHeader) PreProcess(ctx *gateway.Context) error {
	ctx.Request.Headers.Add(f.headerName, f.headerValue)
	return nil
}

func (f *AddRequestHeader) PostProcess(_ *gateway.Context) error {
	return nil
}

func (f *AddRequestHeader) Name() string {
	return AddRequestHeaderFilterName
}

type SetRequestHeader struct {
	headerName  string
	headerValue string
}

func NewSetRequestHeaderBuilder() gateway.FilterBuilder {
	return gateway.FilterBuilderFunc(func(args map[string]any) (gateway.Filter, error) {
		name, err := common.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		value, err := common.ConvertToString(args["value"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'value' attribute: %w", err)
		}
		return NewSetRequestHeaderFilter(name, value), nil
	})
}

func NewSetRequestHeaderFilter(name, value string) *SetRequestHeader {
	return &SetRequestHeader{
		headerName:  name,
		headerValue: value,
	}
}

func (f *SetRequestHeader) PreProcess(ctx *gateway.Context) error {
	ctx.Request.Headers.Set(f.headerName, f.headerValue)
	return nil
}

func (f *SetRequestHeader) PostProcess(_ *gateway.Context) error {
	return nil
}

func (f *SetRequestHeader) Name() string {
	return SetRequestHeaderFilterName
}

type RemoveRequestHeader struct {
	headerName string
}

func NewRemoveRequestHeaderFilter(name string) *RemoveRequestHeader {
	return &RemoveRequestHeader{
		headerName: name,
	}
}

func NewRemoveRequestHeaderBuilder() gateway.FilterBuilder {
	return gateway.FilterBuilderFunc(func(args map[string]any) (gateway.Filter, error) {
		name, err := common.ConvertToString(args["name"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'name' attribute: %w", err)
		}
		return NewRemoveRequestHeaderFilter(name), nil
	})
}

func (f *RemoveRequestHeader) PreProcess(ctx *gateway.Context) error {
	ctx.Request.Headers.Del(f.headerName)
	return nil
}

func (f *RemoveRequestHeader) PostProcess(_ *gateway.Context) error {
	return nil
}

func (f *RemoveRequestHeader) Name() string {
	return RemoveRequestHeaderFilterName
}
