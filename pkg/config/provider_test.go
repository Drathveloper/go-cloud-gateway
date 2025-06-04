package config_test

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/drathveloper/go-cloud-gateway/pkg/filter"
	"github.com/drathveloper/go-cloud-gateway/pkg/gateway"
	"github.com/drathveloper/go-cloud-gateway/pkg/predicate"
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
							Timeout: config.Duration{},
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
					Timeout: 10 * time.Second,
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
							Timeout: config.Duration{},
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
							Timeout: config.Duration{},
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

func TestNewHTTPClient(t *testing.T) {
	mockValidCert := `-----BEGIN CERTIFICATE-----
MIIE+DCCAuCgAwIBAgIUSMcgR98H2yE7GimsKnfxslDXABAwDQYJKoZIhvcNAQEL
BQAwEjEQMA4GA1UEAwwHVGVzdCBDQTAeFw0yNTA1MjgxODQxMDZaFw0yNjA1Mjgx
ODQxMDZaMBYxFDASBgNVBAMMC1Rlc3QgQ2xpZW50MIICIjANBgkqhkiG9w0BAQEF
AAOCAg8AMIICCgKCAgEAvPNNLMwuv8Po2IZoG2hGklv4WOKcmd8NxLSBKsGQzZsY
ZuQVnviaQ9SwBdTYX/wieo8kR4MiCzdLmNPjlw92DTUuz+fkr7lSAB4W35ysD+kp
E5ae+unrXlk+kwxAoqlTZtDutDoztiUbJ+BzZ94cqRcjsgtH26H8mg1OlF26h7yw
jDitF0c8VMYqRsPCkAXY4T1ejLw0Xlsu0b4dNvjPxvb/cwfu/U9ZcO9Bw9InEo8s
5p0EmTiJ3dYVvRJHyNS3uv4s8GWqocdh2ry8E5W0qnPYQj5C/IRcNNh/5VFM88zQ
1xGuVq+roXDCziv2KYIFnseAHFYDxHRnYbQCt0Lio6iRfenf7/hqYKBXeTEtRA1Z
4Irngxydi8JttrlJeu7eu3PUmLkhKSWLZTEhnYxGfmLR3qrBDLRsgsagY1YtOEfu
lEJHGht+rGjmYeqBLZlZJibDOIKFz1rKRZYQIw+hZgNSdOBEpo2rGKH+8zCShC6G
FqGIXGfjvoyQvoBwmQmz42QVUgrz5aCww529YKjy8XR989Htkt4w7RL2cwuqj7To
+xGARelItvzsiptENJpfRCWLUZ2ZivLAeVqcxsHFJnXknGSIiA5vUoqp7bUK36hX
5tsIo7rrTdPxY3WEiWSutYDXSrs9ubXwsEOBO0v1nKk4435jzKXXMjkEQeaWpHkC
AwEAAaNCMEAwHQYDVR0OBBYEFPvW5EJCGvSXvJIAH3Z8ERYrcAfsMB8GA1UdIwQY
MBaAFCa6OrNDPM7Zoum2m4hHHNW+KGijMA0GCSqGSIb3DQEBCwUAA4ICAQA5swdz
/qPjQORDfWF7BGnROmxevBItLL4HfsGyIybjnQ3x6Mo4drTI6F/dIDIRQMGbdkAm
haBHQ3vigYscxva61p61NSyrpCx5txzZrVtz2c7UTY9EhN5U5HS7JzkWMsPwheBL
0+cwYnvGm8DTmYdATA+Y/d0q1BEHZah0ESxw39dGgO31t4RvJ98dy3krhOXKMmjn
TAvo7X19BcBXMFFRUZq9R+SBoDRmDwMtJi438PTEWlL30oz3WRpoRADhF/7/bplH
6ObhaKFKLgUUMK6Y+hNYEnAYfTP7dxMt2p+uKa8j4QR03A4HgLSFrh7EWAC6lf9o
Bo4/gzzoHa5jdtapS+hfZ6OXreEtwhRXU2s5WlxvLDj0fgLchRF5XE9OvXNqVWrk
yReaKJc0wbO17SI8I+ePS3fBcAOnQVJRX1hpgBHPQzi0kM5wH1D5wzRbzHv/4SIP
WOETtkx3ehXIheDOX4Ba0P9tPTz/+6YqbwwY0zJBdLdKLxiTR2nlRSWYuNX817j7
7FSvEkAKogh5mqx3wY2sMiWdzr0Qus5jyUBI7fz2KDKsaC4eqDJlcC7C9tHoesu6
Pf7cQ9/Ojs7/q2V0RoUdUM8VZ7C9lbF1AvtVTia2RTIwO0fY8qdlBJ0cZQNk4muy
vJar84bEJhbOHUNS14hPXHT+ZOAo2gbFPNFBMw==
-----END CERTIFICATE-----
`
	mockValidKey := `-----BEGIN PRIVATE KEY-----
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
If7cNJW/IwJYa2+purHtuebxSayQlA==
-----END PRIVATE KEY-----
`
	mockValidCA := `-----BEGIN CERTIFICATE-----
MIIFBTCCAu2gAwIBAgIUff730voApjqMhnnjqP8v9NdbXFkwDQYJKoZIhvcNAQEL
BQAwEjEQMA4GA1UEAwwHVGVzdCBDQTAeFw0yNTA1MjgxODQwMzZaFw0zNTA1MjYx
ODQwMzZaMBIxEDAOBgNVBAMMB1Rlc3QgQ0EwggIiMA0GCSqGSIb3DQEBAQUAA4IC
DwAwggIKAoICAQCmdx1sIgsgegE2J5hkQNC9D0FURHSI9MAhEc77m/pQ77w5OBfa
dQXqJChJB+xcKGUBOC5C1yWNeGMmab2pO5NOqtTmcM7N/jHlbJjii559vq5IN1So
PwUfx5BRvASJ0XIRRkLyRmtHni26xrO9GwaX2yAxxtmLSHRWxc20ZXVE9E8X38Pt
7Ggl7HO9g2/s55RuRfqhidIraeh1ZumjMsepygMeAhbFyol2Qrr9n6VN+VCPSQM2
tdCSSz/XbGNjH89LElmMg6c02YPMyxh+Q+ZLL4SCMWFYB4TXSyWz41A3MJnd2TL8
PtEuwOmUPBFjFAypIZyi6Y3kY1DLaTxZfXrPBkJdkA5ZpXs2W5vZxoEVtX8+MjQq
yAigWFw19wUzq/k+rXUQ7P0z/LQOByNzZJRtWYLtQgoQbFc2OjERmgDwKPD1fB/J
Z6LJccJuAtBY7UcyrSneHmlAJmbuADmPtyrVKOyR3oOfzKfAG+F2vknm1xPN+35Q
bdG4l/sulAWreK6EUKcINOuQy/jTff2vgzbkIfI+nppNK5IVOONYMnh3qKbJC13n
qF4DjKAf5KeLjb94tlDPiPHl2UCcV8UwKCLsSYlg4mSpS9BmoFCCR6d3x1AvZ6+7
8Sf2ni+UHabLnJXr3DlC1KMIWqwyNfeg2baNJE54PdFXTfkYLEzL6ch91wIDAQAB
o1MwUTAdBgNVHQ4EFgQUJro6s0M8ztmi6babiEcc1b4oaKMwHwYDVR0jBBgwFoAU
Jro6s0M8ztmi6babiEcc1b4oaKMwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0B
AQsFAAOCAgEAF9QifeRY42tM3ml+1pXFoHk5ZkrSxRtV+i6GaRtgPFn0bVIwkhjt
C7siCUUPylcwf44NPWrO88i1CZjDVli+uOrcR7nBG2oWCkrQvZCqjvfpkKkPxZNY
oVfo67pfqTNsrYoX0ElxYcz27ARu2cBRInvE8MhlJxBonvtTWyXWhZbait5AZrnL
WP0vlviPegknC/WVEnleD4jzMr9EzYdOjwyj+/YleXI6ZX6+Q8+HTeF9eRtgXLIL
wC8FznJVDX90rGSg+GCcFl1N3dYNm7FteH0uz5S+os9B3EH6L/H01zvs4OURgAuk
lMdmnJnHIqMYbDFOta+l6ngKbHvUBLdSiL8OliOsXhH4h6KDYkYIeEFyaLqqGVoN
Emq5SOSv0oqJ2le6lIO0eRFnmiaFxOcm7lbjO4DU0dKUSeiyP9wiKLKbH+5euhPe
b1j3VZUkSZvE8A2e6EPHLSGZuIVVVnc1VSw5buKzkoNzopeubR3twiloGzg62RlL
Iv22V31ZrcCrS/fN7hNvXr7yKj15Jf3vg0FFpISyjFbLdYTmXdqdx2bai1EHHokx
vNcxgy5QlCT6/XmolsGLBJ8s+mvHZDR+GSJnCvWGUIrGzZ1ynIxljjRFwwG8IH/0
F0WydPKUjl3tmQRxYd9C8zDt6yB/fQbIoM/uGgZ0ZoZ+E5hvLVe+rYk=
-----END CERTIFICATE-----
`
	trueBool := true
	tests := []struct {
		name        string
		cfg         *config.Config
		wantClient  bool
		wantErr     bool
		checkClient func(*http.Client) bool
	}{
		{
			name:       "nil config - default client",
			cfg:        nil,
			wantClient: true,
			wantErr:    false,
			checkClient: func(c *http.Client) bool {
				return c.Timeout == config.DefaultTimeout
			},
		},
		{
			name:       "empty config - default client",
			cfg:        &config.Config{},
			wantClient: true,
			wantErr:    false,
			checkClient: func(c *http.Client) bool {
				return c.Timeout == config.DefaultTimeout
			},
		},
		{
			name: "custom pool configuration",
			cfg: &config.Config{
				Gateway: config.Gateway{
					HTTPClient: &config.HTTPClient{
						Pool: &config.Pool{
							ConnectTimeout:      &config.Duration{Duration: 30 * time.Second},
							MaxIdleConns:        100,
							MaxIdleConnsPerHost: 20,
							MaxConnsPerHost:     50,
							IdleConnTimeout:     &config.Duration{Duration: 90 * time.Second},
							TLSHandshakeTimeout: &config.Duration{Duration: 15 * time.Second},
						},
					},
				},
			},
			wantClient: true,
			wantErr:    false,
			checkClient: func(c *http.Client) bool {
				return c.Timeout == 30*time.Second
			},
		},
		{
			name: "insecure TLS enabled",
			cfg: &config.Config{
				Gateway: config.Gateway{
					HTTPClient: &config.HTTPClient{
						InsecureTLSVerify: true,
					},
				},
			},
			wantClient: true,
			wantErr:    false,
			checkClient: func(c *http.Client) bool {
				transport := c.Transport.(*http.Transport)
				return transport.TLSClientConfig.InsecureSkipVerify
			},
		},
		{
			name: "mTLS enabled with valid certs",
			cfg: &config.Config{
				Gateway: config.Gateway{
					HTTPClient: &config.HTTPClient{
						MTLS: &config.MTLS{
							Enabled: &trueBool,
							Cert:    mockValidCert,
							Key:     mockValidKey,
							CA:      mockValidCA,
						},
					},
				},
			},
			wantClient: true,
			wantErr:    false,
			checkClient: func(c *http.Client) bool {
				transport := c.Transport.(*http.Transport)
				return len(transport.TLSClientConfig.Certificates) > 0 &&
					transport.TLSClientConfig.RootCAs != nil
			},
		},
		{
			name: "mTLS enabled with invalid certs",
			cfg: &config.Config{
				Gateway: config.Gateway{
					HTTPClient: &config.HTTPClient{
						MTLS: &config.MTLS{
							Enabled: &trueBool,
							Cert:    "invalid",
							Key:     "invalid",
							CA:      "invalid",
						},
					},
				},
			},
			wantClient: false,
			wantErr:    true,
		},
		{
			name: "mTLS enabled but missing CA",
			cfg: &config.Config{
				Gateway: config.Gateway{
					HTTPClient: &config.HTTPClient{
						MTLS: &config.MTLS{
							Enabled: &trueBool,
							Cert:    mockValidCert,
							Key:     mockValidKey,
							CA:      "",
						},
					},
				},
			},
			wantClient: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := config.NewHTTPClient(tt.cfg)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewHTTPClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantClient {
				if client == nil {
					t.Error("Expected HTTP client, got nil")
					return
				}

				if tt.checkClient != nil && !tt.checkClient(client) {
					t.Error("Client check failed")
				}
			} else if client != nil {
				t.Error("Expected nil client, got non-nil")
			}
		})
	}
}
