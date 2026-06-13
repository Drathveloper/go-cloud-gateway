package shared_test

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/drathveloper/go-cloud-gateway/internal/pkg/shared"
)

func TestSetXForwardedHeaders(t *testing.T) {
	tests := []struct {
		request       *http.Request
		expected      http.Header
		name          string
		expectedNoFor bool
	}{
		{
			name: "peer ip is set as forwarded-for when there is no prior list",
			request: &http.Request{
				Host:       "gw.example.org",
				RemoteAddr: "203.0.113.7:4321",
				Header:     http.Header{},
			},
			expected: http.Header{
				"X-Forwarded-For":   {"203.0.113.7"},
				"X-Forwarded-Host":  {"gw.example.org"},
				"X-Forwarded-Proto": {"http"},
			},
		},
		{
			name: "peer ip is appended to the prior forwarded-for list",
			request: &http.Request{
				Host:       "gw.example.org",
				RemoteAddr: "203.0.113.7:4321",
				Header: http.Header{
					"X-Forwarded-For": {"198.51.100.1, 198.51.100.2"},
				},
			},
			expected: http.Header{
				"X-Forwarded-For":   {"198.51.100.1, 198.51.100.2, 203.0.113.7"},
				"X-Forwarded-Host":  {"gw.example.org"},
				"X-Forwarded-Proto": {"http"},
			},
		},
		{
			name: "client supplied host and proto are overwritten",
			request: &http.Request{
				Host:       "gw.example.org",
				RemoteAddr: "203.0.113.7:4321",
				Header: http.Header{
					"X-Forwarded-Host":  {"evil.example.org"},
					"X-Forwarded-Proto": {"https"},
				},
			},
			expected: http.Header{
				"X-Forwarded-For":   {"203.0.113.7"},
				"X-Forwarded-Host":  {"gw.example.org"},
				"X-Forwarded-Proto": {"http"},
			},
		},
		{
			name: "proto is https when the connection is TLS",
			request: &http.Request{
				Host:       "gw.example.org",
				RemoteAddr: "203.0.113.7:4321",
				Header:     http.Header{},
				TLS:        &tls.ConnectionState{},
			},
			expected: http.Header{
				"X-Forwarded-For":   {"203.0.113.7"},
				"X-Forwarded-Host":  {"gw.example.org"},
				"X-Forwarded-Proto": {"https"},
			},
		},
		{
			name: "forwarded-for is dropped when the peer address is not parseable",
			request: &http.Request{
				Host:       "gw.example.org",
				RemoteAddr: "",
				Header: http.Header{
					"X-Forwarded-For": {"198.51.100.1"},
				},
			},
			expected: http.Header{
				"X-Forwarded-Host":  {"gw.example.org"},
				"X-Forwarded-Proto": {"http"},
			},
			expectedNoFor: true,
		},
		{
			name: "ipv6 peer is appended without brackets",
			request: &http.Request{
				Host:       "gw.example.org",
				RemoteAddr: "[2001:db8::1]:5555",
				Header:     http.Header{},
			},
			expected: http.Header{
				"X-Forwarded-For":   {"2001:db8::1"},
				"X-Forwarded-Host":  {"gw.example.org"},
				"X-Forwarded-Proto": {"http"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shared.SetXForwardedHeaders(tt.request)

			for name, want := range tt.expected {
				if got := tt.request.Header.Get(name); got != want[0] {
					t.Errorf("expected %s=%q actual %q", name, want[0], got)
				}
			}
			if tt.expectedNoFor {
				if got := tt.request.Header.Get("X-Forwarded-For"); got != "" {
					t.Errorf("expected X-Forwarded-For dropped, actual %q", got)
				}
			}
		})
	}
}
