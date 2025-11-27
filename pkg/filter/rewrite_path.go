package filter

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
)

// RewritePathFilterName is the name of the filter.
const RewritePathFilterName = "RewritePath"

// GatewayOriginalRequestAttr is the name of the attribute that contains the original request URL.
const GatewayOriginalRequestAttr = "GATEWAY_ORIGINAL_REQUEST_URL"

// RewritePath is a filter that rewrites the path of the request.
type RewritePath struct {
	pattern     *regexp.Regexp
	Regexp      string
	Replacement string
}

// NewRewritePathFilter creates a new RewritePathFilter.
func NewRewritePathFilter(regexpStr, replacement string) (*RewritePath, error) {
	normalizedReplacement := strings.ReplaceAll(replacement, "$\\", "$")
	pattern, err := regexp.Compile(regexpStr)
	if err != nil {
		return nil, fmt.Errorf("failed to build rewrite path filter: %w", err)
	}
	return &RewritePath{
		Regexp:      regexpStr,
		Replacement: normalizedReplacement,
		pattern:     pattern,
	}, nil
}

// NewRewritePathBuilder creates a new RewritePathBuilder.
func NewRewritePathBuilder() gateway.FilterBuilderFunc {
	return func(args map[string]any) (gateway.Filter, error) {
		regex, err := shared.ConvertToString(args["regexp"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'regexp' attribute: %w", err)
		}
		replacement, err := shared.ConvertToString(args["replacement"])
		if err != nil {
			return nil, fmt.Errorf("failed to convert 'replacement' attribute: %w", err)
		}
		return NewRewritePathFilter(regex, replacement)
	}
}

// PreProcess rewrites the path of the request.
// If the path does not match the regexp, the filter will do nothing.
// If the path matches the regexp, the filter will rewrite the path.
// The original request URL is stored in the context as an attribute with the name GatewayOriginalRequestAttr.
func (f *RewritePath) PreProcess(ctx *gateway.Context) error {
	ctx.Attributes[GatewayOriginalRequestAttr] = ctx.Request.URL
	currentPath := ctx.Request.URL.Path
	if !f.pattern.MatchString(currentPath) {
		return nil
	}
	newPath := f.pattern.ReplaceAllString(currentPath, f.Replacement)
	if currentPath == newPath {
		return nil
	}
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

// PostProcess does nothing.
func (f *RewritePath) PostProcess(_ *gateway.Context) error {
	return nil
}

// Name returns the name of the filter.
func (f *RewritePath) Name() string {
	return RewritePathFilterName
}
