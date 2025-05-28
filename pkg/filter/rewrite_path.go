package filter

import (
	"fmt"
	"gateway/pkg/common"
	"gateway/pkg/gateway"
	"net/url"
	"regexp"
	"strings"
)

const RewritePathFilterName = "RewritePath"
const GatewayOriginalRequestAttr = "GATEWAY_ORIGINAL_REQUEST_URL"

type RewritePath struct {
	Regexp      string
	Replacement string
	pattern     *regexp.Regexp
}

func NewRewritePathFilter(regexpStr, replacement string) (*RewritePath, error) {
	normalizedReplacement := strings.ReplaceAll(replacement, "$\\", "$")
	pattern, err := regexp.Compile(regexpStr)
	if err != nil {
		return nil, err
	}
	return &RewritePath{
		Regexp:      regexpStr,
		Replacement: normalizedReplacement,
		pattern:     pattern,
	}, nil
}

func NewRewritePathBuilder() gateway.FilterBuilder {
	return gateway.FilterBuilderFunc(func(args map[string]any) (gateway.Filter, error) {
		regex, err := common.ConvertToString(args["regexp"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'regexp' attribute: %w", err)
		}
		replacement, err := common.ConvertToString(args["replacement"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'replacement' attribute: %w", err)
		}
		return NewRewritePathFilter(regex, replacement)
	})
}

func (f *RewritePath) PreProcess(ctx *gateway.Context) error {
	ctx.Attributes[GatewayOriginalRequestAttr] = ctx.Request.URL
	currentPath := ctx.Request.URL.Path
	newPath := f.pattern.ReplaceAllString(currentPath, f.Replacement)
	newURL := &url.URL{
		Scheme:   ctx.Request.URL.Scheme,
		Host:     ctx.Request.URL.Host,
		Path:     newPath,
		RawQuery: ctx.Request.URL.RawQuery,
		Fragment: ctx.Request.URL.Fragment,
	}
	ctx.Request.URL = newURL
	return nil
}

func (f *RewritePath) PostProcess(_ *gateway.Context) error {
	return nil
}

func (f *RewritePath) Name() string {
	return RewritePathFilterName
}
