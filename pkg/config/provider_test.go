package config_test

import (
	"errors"
	"fmt"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
	"reflect"
	"testing"
	"time"
)

func TestNewRoutes(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		expected    gateway.Routes
		expectedErr error
	}{
		{
			name: "new routes should succeed",
			config: &config.Config{
				Gateway: config.Gateway{
					Routes: []config.Route{
						{
							ID:  "r1",
							URI: "someUri",
							Predicates: []config.ParameterizedItem{
								{
									Name: "Method",
									Args: map[string]any{
										"methods": []any{"GET", "POST"},
									},
								},
							},
							Filters: []config.ParameterizedItem{
								{
									Name: "AddRequestHeader",
									Args: map[string]any{
										"name":  "X-Test",
										"value": "True",
									},
								},
							},
						},
					},
				},
			},
			expected: gateway.Routes{
				{
					ID:  "r1",
					URI: "someUri",
					Predicates: gateway.Predicates{
						predicate.NewMethodPredicate("GET", "POST"),
					},
					Filters: gateway.Filters{
						filter.NewAddRequestHeaderFilter("X-Test", "True"),
					},
				},
			},
			expectedErr: nil,
		},
		{
			name: "new routes should return error when predicate is not valid",
			config: &config.Config{
				Gateway: config.Gateway{
					Routes: []config.Route{
						{
							ID:  "r1",
							URI: "someUri",
							Predicates: []config.ParameterizedItem{
								{
									Name: "Other",
								},
							},
							Filters: []config.ParameterizedItem{
								{
									Name: "AddRequestHeader",
									Args: map[string]any{
										"name":  "X-Test",
										"value": "True",
									},
								},
							},
						},
					},
				},
			},
			expected:    nil,
			expectedErr: errors.New("map routes from config to gateway failed: parse predicates failed: predicate builder not found for predicate Other"),
		},
		{
			name: "new routes should return error when filter is not valid",
			config: &config.Config{
				Gateway: config.Gateway{
					Routes: []config.Route{
						{
							ID:  "r1",
							URI: "someUri",
							Predicates: []config.ParameterizedItem{
								{
									Name: "Method",
									Args: map[string]any{
										"methods": []any{"GET", "POST"},
									},
								},
							},
							Filters: []config.ParameterizedItem{
								{
									Name: "Invent",
								},
							},
						},
					},
				},
			},
			expected:    nil,
			expectedErr: errors.New("map routes from config to gateway failed: parse filters failed: filter builder not found for filter Invent"),
		},
		{
			name: "new routes should return empty when predicate is not valid",
			config: &config.Config{
				Gateway: config.Gateway{
					Routes: nil,
				},
			},
			expected:    gateway.Routes{},
			expectedErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routes, err := config.NewRoutes(
				tt.config,
				predicate.NewFactory(predicate.BuilderRegistry),
				filter.NewFactory(filter.BuilderRegistry))

			if !reflect.DeepEqual(tt.expected, routes) {
				t.Errorf("expected %v actual %v", tt.expected, routes)
			}
			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}

func TestNewGlobalFilters(t *testing.T) {
	tests := []struct {
		name        string
		config      *config.Config
		expected    gateway.Filters
		expectedErr error
	}{
		{
			name: "new global filters should succeed",
			config: &config.Config{
				Gateway: config.Gateway{
					GlobalFilters: []config.ParameterizedItem{
						{
							Name: "AddRequestHeader",
							Args: map[string]any{
								"name":  "X-Test",
								"value": "True",
							},
						},
					},
				},
			},
			expected: gateway.Filters{
				filter.NewAddRequestHeaderFilter("X-Test", "True"),
			},
			expectedErr: nil,
		},
		{
			name: "new global filters should return error when filter is not valid",
			config: &config.Config{
				Gateway: config.Gateway{
					GlobalFilters: []config.ParameterizedItem{
						{
							Name: "Invent",
						},
					},
				},
			},
			expected:    nil,
			expectedErr: errors.New("parse filters failed: filter builder not found for filter Invent"),
		},
		{
			name: "new global filters should return empty when no global filters are defined",
			config: &config.Config{
				Gateway: config.Gateway{
					GlobalFilters: nil,
				},
			},
			expected:    gateway.Filters{},
			expectedErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalFilters, err := config.NewGlobalFilters(
				tt.config,
				filter.NewFactory(filter.BuilderRegistry))

			if !reflect.DeepEqual(tt.expected, globalFilters) {
				t.Errorf("expected %v actual %v", tt.expected, globalFilters)
			}
			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}

func TestNewGlobalTimeout(t *testing.T) {
	tests := []struct {
		name     string
		config   *config.Config
		expected time.Duration
	}{
		{
			name: "new global timeout should succeed when config is present",
			config: &config.Config{
				GlobalTimeout: config.Duration{
					Duration: 30 * time.Second,
				},
			},
			expected: 30 * time.Second,
		},
		{
			name:     "new global timeout should return default timeout when config is not present",
			config:   &config.Config{},
			expected: 10 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalTimeout := config.NewGlobalTimeout(tt.config)

			if tt.expected != globalTimeout {
				t.Errorf("expected %v actual %v", tt.expected, globalTimeout)
			}
		})
	}
}
