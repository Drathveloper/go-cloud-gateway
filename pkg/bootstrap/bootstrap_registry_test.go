package bootstrap_test

import (
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
)

const (
	// customHeader is the response header the custom test filter sets.
	customHeader = "X-Custom-Filter"
	// builtinHeader is the response header the test config asks SetResponseHeader for.
	builtinHeader = "X-Builtin-Filter"
	// targetHeader is the request header the custom test predicate looks at.
	targetHeader = "X-Target"
)

// tagFilter is a custom filter that sets customHeader in the response to its tag.
type tagFilter struct {
	tag string
}

func (f *tagFilter) PreProcess(_ *gateway.Context) error {
	return nil
}

func (f *tagFilter) PostProcess(ctx *gateway.Context) error {
	ctx.Response.Headers.Set(customHeader, f.tag)
	return nil
}

func (f *tagFilter) Name() string {
	return "Tag"
}

// targetPredicate is a custom predicate that matches the requests whose targetHeader is
// its target.
type targetPredicate struct {
	target string
}

func (p *targetPredicate) Test(request *http.Request) bool {
	return request.Header.Get(targetHeader) == p.target
}

func (p *targetPredicate) Name() string {
	return "Target"
}

// newCustomBuilders returns a filter registered under filterName that tags the responses
// with the given name, and a predicate registered under predicateName that only matches
// the requests targeted at that name. The builders ignore their arguments.
func newCustomBuilders(
	filterName, predicateName, name string,
) ([]bootstrap.CustomFilter, []bootstrap.CustomPredicate) {
	filterBuilder := gateway.FilterBuilderFunc(func(_ map[string]any) (gateway.Filter, error) {
		return &tagFilter{tag: name}, nil
	})
	predicateBuilder := gateway.PredicateBuilderFunc(func(_ map[string]any) (gateway.Predicate, error) {
		return &targetPredicate{target: name}, nil
	})
	return []bootstrap.CustomFilter{{Name: filterName, Builder: filterBuilder}},
		[]bootstrap.CustomPredicate{{Name: predicateName, Builder: predicateBuilder}}
}

// newRegistryTestConfig returns a config with a route that uses the predicate and the
// filter of the given names. The arguments are the ones the built-in Method predicate
// and SetResponseHeader filter take, so that the names of those two can be used as well.
func newRegistryTestConfig(t *testing.T, filterName, predicateName string) *config.Config {
	t.Helper()
	return readTestConfig(t, fmt.Sprintf(`{"gateway":{"routes":[{"id":"r1","uri":%q,`+
		`"predicates":[{"name":%q,"args":{"methods":["GET"]}}],`+
		`"filters":[{"name":%q,"args":{"name":%q,"value":"builtin"}}]}]}}`,
		newStatusBackend(t, http.StatusOK).URL, predicateName, filterName, builtinHeader))
}

// getTestTarget sends a request targeted at the given name and returns the status code
// and the headers of the response.
func getTestTarget(t *testing.T, gatewayServer *httptest.Server, target string) (int, http.Header) {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, gatewayServer.URL+"/test", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	if target != "" {
		request.Header.Set(targetHeader, target)
	}
	res, err := gatewayServer.Client().Do(request)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer res.Body.Close() //nolint:errcheck
	return res.StatusCode, res.Header
}

// builderIdentities returns what tells the builders of a registry apart, by name. The
// built-in builders are functions, which cannot be compared with each other directly.
func builderIdentities[Builder any](registry map[string]Builder) map[string]uintptr {
	identities := make(map[string]uintptr, len(registry))
	for name, builder := range registry {
		identities[name] = reflect.ValueOf(builder).Pointer()
	}
	return identities
}

func TestInitialize_CustomBuildersAreNotRegisteredGlobally(t *testing.T) {
	tests := []struct {
		name          string
		filterName    string
		predicateName string
	}{
		{
			name:          "custom builders with new names should not be added to the global registries",
			filterName:    "GloballyUnknownFilter",
			predicateName: "GloballyUnknownPredicate",
		},
		{
			name:          "custom builders with built-in names should not replace the built-in ones",
			filterName:    filter.SetResponseHeaderFilterName,
			predicateName: predicate.MethodPredicateName,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtersBefore := builderIdentities(filter.BuilderRegistry)
			predicatesBefore := builderIdentities(predicate.BuilderRegistry)
			customFilters, customPredicates := newCustomBuilders(tt.filterName, tt.predicateName, "custom")
			opts := bootstrap.NewOptionsBuilder(newRegistryTestConfig(t, tt.filterName, tt.predicateName)).
				WithCustomFilters(customFilters...).
				WithCustomPredicates(customPredicates...).
				Build()
			gatewayServer := startTestGatewayWithOptions(t, opts)

			status, headers := getTestTarget(t, gatewayServer, "custom")

			// The custom builders are the ones the gateway was built with.
			if status != http.StatusOK {
				t.Errorf("expected status %d actual %d", http.StatusOK, status)
			}
			if actual := headers.Get(customHeader); actual != "custom" {
				t.Errorf("expected header %s %q actual %q", customHeader, "custom", actual)
			}
			if filtersAfter := builderIdentities(filter.BuilderRegistry); !maps.Equal(filtersBefore, filtersAfter) {
				t.Errorf("expected global filter registry %v actual %v", filtersBefore, filtersAfter)
			}
			if predicatesAfter := builderIdentities(predicate.BuilderRegistry); !maps.Equal(predicatesBefore, predicatesAfter) {
				t.Errorf("expected global predicate registry %v actual %v", predicatesBefore, predicatesAfter)
			}
		})
	}
}

// Two gateways of the same process that register different builders under one name must
// each get their own, and a gateway that registers none must get neither.
func TestInitialize_CustomBuildersAreScopedToTheirGateway(t *testing.T) {
	tests := []struct {
		expectedPlainErr error
		name             string
		filterName       string
		predicateName    string
	}{
		{
			name:             "custom filter with a new name should be unknown to a gateway without it",
			filterName:       "ScopedFilter",
			predicateName:    predicate.MethodPredicateName,
			expectedPlainErr: filter.ErrFilterBuilder,
		},
		{
			name:             "custom predicate with a new name should be unknown to a gateway without it",
			filterName:       filter.SetResponseHeaderFilterName,
			predicateName:    "ScopedPredicate",
			expectedPlainErr: predicate.ErrInvalidPredicate,
		},
		{
			name:             "custom builders with built-in names should leave the built-in ones to a gateway without them",
			filterName:       filter.SetResponseHeaderFilterName,
			predicateName:    predicate.MethodPredicateName,
			expectedPlainErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newRegistryTestConfig(t, tt.filterName, tt.predicateName)
			gateways := make(map[string]*httptest.Server)
			for _, name := range []string{"first", "second"} {
				customFilters, customPredicates := newCustomBuilders(tt.filterName, tt.predicateName, name)
				opts := bootstrap.NewOptionsBuilder(cfg).
					WithCustomFilters(customFilters...).
					WithCustomPredicates(customPredicates...).
					Build()
				gateways[name] = startTestGatewayWithOptions(t, opts)
			}

			plainServer, err := bootstrap.Initialize(bootstrap.NewOptionsBuilder(cfg).Build())

			for name, gatewayServer := range gateways {
				assertOwnCustomBuilders(t, gatewayServer, name)
			}
			if !errors.Is(err, tt.expectedPlainErr) {
				t.Fatalf("expected err %v actual %v", tt.expectedPlainErr, err)
			}
			if tt.expectedPlainErr != nil {
				return
			}
			plainGateway := httptest.NewServer(plainServer.Handler)
			defer plainGateway.Close()
			status, headers := getTestTarget(t, plainGateway, "")
			if status != http.StatusOK {
				t.Errorf("expected status %d actual %d", http.StatusOK, status)
			}
			if actual := headers.Get(builtinHeader); actual != "builtin" {
				t.Errorf("expected header %s %q actual %q", builtinHeader, "builtin", actual)
			}
			if actual := headers.Get(customHeader); actual != "" {
				t.Errorf("expected no header %s actual %q", customHeader, actual)
			}
		})
	}
}

// assertOwnCustomBuilders checks that the gateway was built with the custom builders of
// the given name: it only matches the requests targeted at that name and tags their
// responses with it.
func assertOwnCustomBuilders(t *testing.T, gatewayServer *httptest.Server, name string) {
	t.Helper()
	for _, target := range []string{"first", "second"} {
		status, headers := getTestTarget(t, gatewayServer, target)
		if target != name {
			if status != http.StatusNotFound {
				t.Errorf("gateway %s: expected status %d for target %s actual %d",
					name, http.StatusNotFound, target, status)
			}
			continue
		}
		if status != http.StatusOK {
			t.Errorf("gateway %s: expected status %d for target %s actual %d", name, http.StatusOK, target, status)
		}
		if actual := headers.Get(customHeader); actual != name {
			t.Errorf("gateway %s: expected header %s %q actual %q", name, customHeader, name, actual)
		}
	}
}
