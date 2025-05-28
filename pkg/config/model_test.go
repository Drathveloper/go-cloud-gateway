package config_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/go-playground/validator/v10"
)

func TestMTLS_ValidateJSON(t *testing.T) {
	falseValue := false
	trueValue := true
	tests := []struct {
		name        string
		input       string
		expected    config.MTLS
		expectedErr error
	}{
		{
			name:  "unmarshal and validate should succeed when input is valid and enabled is false",
			input: "{\"enabled\":false}",
			expected: config.MTLS{
				Enabled: &falseValue,
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and enabled is true",
			input: "{\"enabled\":true,\"ca\":\"someCA\",\"cert\":\"someCert\",\"key\":\"someKey\"}",
			expected: config.MTLS{
				Enabled: &trueValue,
				CA:      "someCA",
				Cert:    "someCert",
				Key:     "someKey",
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should return error when enabled is true and CA is empty",
			input: "{\"enabled\":true,\"cert\":\"someCert\",\"key\":\"someKey\"}",
			expected: config.MTLS{
				Enabled: &trueValue,
				Cert:    "someCert",
				Key:     "someKey",
			},
			expectedErr: errors.New("Key: 'MTLS.CA' Error:Field validation for 'CA' failed on the 'required_if' tag"),
		},
		{
			name:  "unmarshal and validate should return error when enabled is true and cert is empty",
			input: "{\"enabled\":true,\"ca\":\"someCA\",\"key\":\"someKey\"}",
			expected: config.MTLS{
				Enabled: &trueValue,
				CA:      "someCA",
				Key:     "someKey",
			},
			expectedErr: errors.New("Key: 'MTLS.Cert' Error:Field validation for 'Cert' failed on the 'required_if' tag"),
		},
		{
			name:  "unmarshal and validate should return error when enabled is true and key is empty",
			input: "{\"enabled\":true,\"ca\":\"someCA\",\"cert\":\"someCert\"}",
			expected: config.MTLS{
				Enabled: &trueValue,
				CA:      "someCA",
				Cert:    "someCert",
			},
			expectedErr: errors.New("Key: 'MTLS.Key' Error:Field validation for 'Key' failed on the 'required_if' tag"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mtlsConfig config.MTLS
			err := json.Unmarshal([]byte(tt.input), &mtlsConfig)
			if err != nil {
				t.Errorf("expected no error actual %s", err)
			}
			if !reflect.DeepEqual(tt.expected, mtlsConfig) {
				t.Errorf("expected %v actual %v", tt.expected, mtlsConfig)
			}
			validate := validator.New()
			err = validate.Struct(mtlsConfig)
			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}

func TestMTLS_ValidateYAML(t *testing.T) {
	falseValue := false
	trueValue := true
	tests := []struct {
		name        string
		input       string
		expected    config.MTLS
		expectedErr error
	}{
		{
			name:  "unmarshal and validate should succeed when input is valid and enabled is false",
			input: "{\"enabled\":false}",
			expected: config.MTLS{
				Enabled: &falseValue,
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and enabled is true",
			input: "{\"enabled\":true,\"ca\":\"someCA\",\"cert\":\"someCert\",\"key\":\"someKey\"}",
			expected: config.MTLS{
				Enabled: &trueValue,
				CA:      "someCA",
				Cert:    "someCert",
				Key:     "someKey",
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should return error when enabled is true and CA is empty",
			input: "{\"enabled\":true,\"cert\":\"someCert\",\"key\":\"someKey\"}",
			expected: config.MTLS{
				Enabled: &trueValue,
				Cert:    "someCert",
				Key:     "someKey",
			},
			expectedErr: errors.New("Key: 'MTLS.CA' Error:Field validation for 'CA' failed on the 'required_if' tag"),
		},
		{
			name:  "unmarshal and validate should return error when enabled is true and cert is empty",
			input: "{\"enabled\":true,\"ca\":\"someCA\",\"key\":\"someKey\"}",
			expected: config.MTLS{
				Enabled: &trueValue,
				CA:      "someCA",
				Key:     "someKey",
			},
			expectedErr: errors.New("Key: 'MTLS.Cert' Error:Field validation for 'Cert' failed on the 'required_if' tag"),
		},
		{
			name:  "unmarshal and validate should return error when enabled is true and key is empty",
			input: "{\"enabled\":true,\"ca\":\"someCA\",\"cert\":\"someCert\"}",
			expected: config.MTLS{
				Enabled: &trueValue,
				CA:      "someCA",
				Cert:    "someCert",
			},
			expectedErr: errors.New("Key: 'MTLS.Key' Error:Field validation for 'Key' failed on the 'required_if' tag"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mtlsConfig config.MTLS
			err := yaml.Unmarshal([]byte(tt.input), &mtlsConfig)
			if err != nil {
				t.Errorf("expected no error actual %s", err)
			}
			if !reflect.DeepEqual(tt.expected, mtlsConfig) {
				t.Errorf("expected %v actual %v", tt.expected, mtlsConfig)
			}
			validate := validator.New()
			err = validate.Struct(mtlsConfig)
			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}

func TestPool_ValidateJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    config.Pool
		expectedErr error
	}{
		{
			name:  "unmarshal and validate should succeed when input is valid",
			input: "{\"connect-timeout\":\"10s\",\"max-idle-conns\":10,\"max-idle-conns-per-host\":15,\"max-conns-per-host\":20,\"idle-conn-timeout\":\"15s\",\"tls-handshake-timeout\":\"20s\"}",
			expected: config.Pool{
				ConnectTimeout:      &config.Duration{Duration: 10 * time.Second},
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 15,
				MaxConnsPerHost:     20,
				IdleConnTimeout:     &config.Duration{Duration: 15 * time.Second},
				TLSHandshakeTimeout: &config.Duration{Duration: 20 * time.Second},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and connect timeout is empty",
			input: "{\"max-idle-conns\":10,\"max-idle-conns-per-host\":15,\"max-conns-per-host\":20,\"idle-conn-timeout\":\"15s\",\"tls-handshake-timeout\":\"20s\"}",
			expected: config.Pool{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 15,
				MaxConnsPerHost:     20,
				IdleConnTimeout:     &config.Duration{Duration: 15 * time.Second},
				TLSHandshakeTimeout: &config.Duration{Duration: 20 * time.Second},
			},
			expectedErr: errors.New("Key: 'Pool.ConnectTimeout' Error:Field validation for 'ConnectTimeout' failed on the 'required' tag"),
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and max idle conns is empty",
			input: "{\"connect-timeout\":\"10s\",\"max-idle-conns-per-host\":15,\"max-conns-per-host\":20,\"idle-conn-timeout\":\"15s\",\"tls-handshake-timeout\":\"20s\"}",
			expected: config.Pool{
				ConnectTimeout:      &config.Duration{Duration: 10 * time.Second},
				MaxIdleConnsPerHost: 15,
				MaxConnsPerHost:     20,
				IdleConnTimeout:     &config.Duration{Duration: 15 * time.Second},
				TLSHandshakeTimeout: &config.Duration{Duration: 20 * time.Second},
			},
			expectedErr: errors.New("Key: 'Pool.MaxIdleConns' Error:Field validation for 'MaxIdleConns' failed on the 'required' tag"),
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and max idle conns per host is empty",
			input: "{\"connect-timeout\":\"10s\",\"max-idle-conns\":10,\"max-conns-per-host\":20,\"idle-conn-timeout\":\"15s\",\"tls-handshake-timeout\":\"20s\"}",
			expected: config.Pool{
				ConnectTimeout:      &config.Duration{Duration: 10 * time.Second},
				MaxIdleConns:        10,
				MaxConnsPerHost:     20,
				IdleConnTimeout:     &config.Duration{Duration: 15 * time.Second},
				TLSHandshakeTimeout: &config.Duration{Duration: 20 * time.Second},
			},
			expectedErr: errors.New("Key: 'Pool.MaxIdleConnsPerHost' Error:Field validation for 'MaxIdleConnsPerHost' failed on the 'required' tag"),
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and max conns per host is empty",
			input: "{\"connect-timeout\":\"10s\",\"max-idle-conns\":10,\"max-idle-conns-per-host\":15,\"idle-conn-timeout\":\"15s\",\"tls-handshake-timeout\":\"20s\"}",
			expected: config.Pool{
				ConnectTimeout:      &config.Duration{Duration: 10 * time.Second},
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 15,
				IdleConnTimeout:     &config.Duration{Duration: 15 * time.Second},
				TLSHandshakeTimeout: &config.Duration{Duration: 20 * time.Second},
			},
			expectedErr: errors.New("Key: 'Pool.MaxConnsPerHost' Error:Field validation for 'MaxConnsPerHost' failed on the 'required' tag"),
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and idle conn timeout is empty",
			input: "{\"connect-timeout\":\"10s\",\"max-idle-conns\":10,\"max-idle-conns-per-host\":15,\"max-conns-per-host\":20,\"tls-handshake-timeout\":\"20s\"}",
			expected: config.Pool{
				ConnectTimeout:      &config.Duration{Duration: 10 * time.Second},
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 15,
				MaxConnsPerHost:     20,
				TLSHandshakeTimeout: &config.Duration{Duration: 20 * time.Second},
			},
			expectedErr: errors.New("Key: 'Pool.IdleConnTimeout' Error:Field validation for 'IdleConnTimeout' failed on the 'required' tag"),
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and tls handshake timeout is empty",
			input: "{\"connect-timeout\":\"10s\",\"max-idle-conns\":10,\"max-idle-conns-per-host\":15,\"max-conns-per-host\":20,\"idle-conn-timeout\":\"15s\"}",
			expected: config.Pool{
				ConnectTimeout:      &config.Duration{Duration: 10 * time.Second},
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 15,
				MaxConnsPerHost:     20,
				IdleConnTimeout:     &config.Duration{Duration: 15 * time.Second},
			},
			expectedErr: errors.New("Key: 'Pool.TLSHandshakeTimeout' Error:Field validation for 'TLSHandshakeTimeout' failed on the 'required' tag"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var poolConfig config.Pool
			err := json.Unmarshal([]byte(tt.input), &poolConfig)
			if err != nil {
				t.Errorf("expected no error actual %s", err)
			}
			if !reflect.DeepEqual(tt.expected, poolConfig) {
				t.Errorf("expected %v actual %v", tt.expected, poolConfig)
			}
			validate := validator.New()
			err = validate.Struct(poolConfig)
			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}

func TestPool_ValidateYAML(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    config.Pool
		expectedErr error
	}{
		{
			name:  "unmarshal and validate should succeed when input is valid",
			input: "{\"connect-timeout\":\"10s\",\"max-idle-conns\":10,\"max-idle-conns-per-host\":15,\"max-conns-per-host\":20,\"idle-conn-timeout\":\"15s\",\"tls-handshake-timeout\":\"20s\"}",
			expected: config.Pool{
				ConnectTimeout:      &config.Duration{Duration: 10 * time.Second},
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 15,
				MaxConnsPerHost:     20,
				IdleConnTimeout:     &config.Duration{Duration: 15 * time.Second},
				TLSHandshakeTimeout: &config.Duration{Duration: 20 * time.Second},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and connect timeout is empty",
			input: "{\"max-idle-conns\":10,\"max-idle-conns-per-host\":15,\"max-conns-per-host\":20,\"idle-conn-timeout\":\"15s\",\"tls-handshake-timeout\":\"20s\"}",
			expected: config.Pool{
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 15,
				MaxConnsPerHost:     20,
				IdleConnTimeout:     &config.Duration{Duration: 15 * time.Second},
				TLSHandshakeTimeout: &config.Duration{Duration: 20 * time.Second},
			},
			expectedErr: errors.New("Key: 'Pool.ConnectTimeout' Error:Field validation for 'ConnectTimeout' failed on the 'required' tag"),
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and max idle conns is empty",
			input: "{\"connect-timeout\":\"10s\",\"max-idle-conns-per-host\":15,\"max-conns-per-host\":20,\"idle-conn-timeout\":\"15s\",\"tls-handshake-timeout\":\"20s\"}",
			expected: config.Pool{
				ConnectTimeout:      &config.Duration{Duration: 10 * time.Second},
				MaxIdleConnsPerHost: 15,
				MaxConnsPerHost:     20,
				IdleConnTimeout:     &config.Duration{Duration: 15 * time.Second},
				TLSHandshakeTimeout: &config.Duration{Duration: 20 * time.Second},
			},
			expectedErr: errors.New("Key: 'Pool.MaxIdleConns' Error:Field validation for 'MaxIdleConns' failed on the 'required' tag"),
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and max idle conns per host is empty",
			input: "{\"connect-timeout\":\"10s\",\"max-idle-conns\":10,\"max-conns-per-host\":20,\"idle-conn-timeout\":\"15s\",\"tls-handshake-timeout\":\"20s\"}",
			expected: config.Pool{
				ConnectTimeout:      &config.Duration{Duration: 10 * time.Second},
				MaxIdleConns:        10,
				MaxConnsPerHost:     20,
				IdleConnTimeout:     &config.Duration{Duration: 15 * time.Second},
				TLSHandshakeTimeout: &config.Duration{Duration: 20 * time.Second},
			},
			expectedErr: errors.New("Key: 'Pool.MaxIdleConnsPerHost' Error:Field validation for 'MaxIdleConnsPerHost' failed on the 'required' tag"),
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and max conns per host is empty",
			input: "{\"connect-timeout\":\"10s\",\"max-idle-conns\":10,\"max-idle-conns-per-host\":15,\"idle-conn-timeout\":\"15s\",\"tls-handshake-timeout\":\"20s\"}",
			expected: config.Pool{
				ConnectTimeout:      &config.Duration{Duration: 10 * time.Second},
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 15,
				IdleConnTimeout:     &config.Duration{Duration: 15 * time.Second},
				TLSHandshakeTimeout: &config.Duration{Duration: 20 * time.Second},
			},
			expectedErr: errors.New("Key: 'Pool.MaxConnsPerHost' Error:Field validation for 'MaxConnsPerHost' failed on the 'required' tag"),
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and idle conn timeout is empty",
			input: "{\"connect-timeout\":\"10s\",\"max-idle-conns\":10,\"max-idle-conns-per-host\":15,\"max-conns-per-host\":20,\"tls-handshake-timeout\":\"20s\"}",
			expected: config.Pool{
				ConnectTimeout:      &config.Duration{Duration: 10 * time.Second},
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 15,
				MaxConnsPerHost:     20,
				TLSHandshakeTimeout: &config.Duration{Duration: 20 * time.Second},
			},
			expectedErr: errors.New("Key: 'Pool.IdleConnTimeout' Error:Field validation for 'IdleConnTimeout' failed on the 'required' tag"),
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and tls handshake timeout is empty",
			input: "{\"connect-timeout\":\"10s\",\"max-idle-conns\":10,\"max-idle-conns-per-host\":15,\"max-conns-per-host\":20,\"idle-conn-timeout\":\"15s\"}",
			expected: config.Pool{
				ConnectTimeout:      &config.Duration{Duration: 10 * time.Second},
				MaxIdleConns:        10,
				MaxIdleConnsPerHost: 15,
				MaxConnsPerHost:     20,
				IdleConnTimeout:     &config.Duration{Duration: 15 * time.Second},
			},
			expectedErr: errors.New("Key: 'Pool.TLSHandshakeTimeout' Error:Field validation for 'TLSHandshakeTimeout' failed on the 'required' tag"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var poolConfig config.Pool
			err := yaml.Unmarshal([]byte(tt.input), &poolConfig)
			if err != nil {
				t.Errorf("expected no error actual %s", err)
			}
			if !reflect.DeepEqual(tt.expected, poolConfig) {
				t.Errorf("expected %v actual %v", tt.expected, poolConfig)
			}
			validate := validator.New()
			err = validate.Struct(poolConfig)
			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}

func TestParameterizedItem_ValidateJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    config.ParameterizedItem
		expectedErr error
	}{
		{
			name:  "unmarshal and validate should succeed when input is valid",
			input: "{\"name\":\"someName\",\"args\":{\"someKey\":\"someValue\"}}",
			expected: config.ParameterizedItem{
				Name: "someName",
				Args: map[string]any{
					"someKey": "someValue",
				},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and args is empty",
			input: "{\"name\":\"someName\"}",
			expected: config.ParameterizedItem{
				Name: "someName",
			},
			expectedErr: nil,
		},
		{
			name:        "unmarshal and validate should return error when input is valid and name is empty",
			input:       "{}",
			expected:    config.ParameterizedItem{},
			expectedErr: errors.New("Key: 'ParameterizedItem.Name' Error:Field validation for 'Name' failed on the 'required' tag"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var poolConfig config.ParameterizedItem
			err := json.Unmarshal([]byte(tt.input), &poolConfig)
			if err != nil {
				t.Errorf("expected no error actual %s", err)
			}
			if !reflect.DeepEqual(tt.expected, poolConfig) {
				t.Errorf("expected %v actual %v", tt.expected, poolConfig)
			}
			validate := validator.New()
			err = validate.Struct(poolConfig)
			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}

func TestParameterizedItem_ValidateYAML(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    config.ParameterizedItem
		expectedErr error
	}{
		{
			name:  "unmarshal and validate should succeed when input is valid",
			input: "{\"name\":\"someName\",\"args\":{\"someKey\":\"someValue\"}}",
			expected: config.ParameterizedItem{
				Name: "someName",
				Args: map[string]any{
					"someKey": "someValue",
				},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and args is empty",
			input: "{\"name\":\"someName\"}",
			expected: config.ParameterizedItem{
				Name: "someName",
			},
			expectedErr: nil,
		},
		{
			name:        "unmarshal and validate should return error when input is valid and name is empty",
			input:       "{}",
			expected:    config.ParameterizedItem{},
			expectedErr: errors.New("Key: 'ParameterizedItem.Name' Error:Field validation for 'Name' failed on the 'required' tag"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var poolConfig config.ParameterizedItem
			err := yaml.Unmarshal([]byte(tt.input), &poolConfig)
			if err != nil {
				t.Errorf("expected no error actual %s", err)
			}
			if !reflect.DeepEqual(tt.expected, poolConfig) {
				t.Errorf("expected %v actual %v", tt.expected, poolConfig)
			}
			validate := validator.New()
			err = validate.Struct(poolConfig)
			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}

func TestRoute_ValidateJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    config.Route
		expectedErr error
	}{
		{
			name:  "unmarshal and validate should succeed when input is valid",
			input: "{\"id\":\"r1\",\"uri\":\"someUri\",\"timeout\":\"30s\",\"predicates\":[{\"name\":\"p1\"}],\"filters\":[{\"name\":\"f1\"}]}",
			expected: config.Route{
				ID:  "r1",
				URI: "someUri",
				Predicates: []config.ParameterizedItem{
					{
						Name: "p1",
					},
				},
				Filters: []config.ParameterizedItem{
					{
						Name: "f1",
					},
				},
				Timeout: config.Duration{Duration: 30 * time.Second},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and filters are empty",
			input: "{\"id\":\"r1\",\"uri\":\"someUri\",\"timeout\":\"30s\",\"predicates\":[{\"name\":\"p1\"}]}",
			expected: config.Route{
				ID:  "r1",
				URI: "someUri",
				Predicates: []config.ParameterizedItem{
					{
						Name: "p1",
					},
				},
				Timeout: config.Duration{Duration: 30 * time.Second},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and predicates are empty",
			input: "{\"id\":\"r1\",\"uri\":\"someUri\",\"timeout\":\"30s\",\"filters\":[{\"name\":\"f1\"}]}",
			expected: config.Route{
				ID:  "r1",
				URI: "someUri",
				Filters: []config.ParameterizedItem{
					{
						Name: "f1",
					},
				},
				Timeout: config.Duration{Duration: 30 * time.Second},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should succeed when timeout is empty",
			input: "{\"id\":\"r1\",\"uri\":\"someUri\",\"predicates\":[{\"name\":\"p1\"}],\"filters\":[{\"name\":\"f1\"}]}",
			expected: config.Route{
				ID:  "r1",
				URI: "someUri",
				Predicates: []config.ParameterizedItem{
					{
						Name: "p1",
					},
				},
				Filters: []config.ParameterizedItem{
					{
						Name: "f1",
					},
				},
				Timeout: config.Duration{},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should return error when input is valid and uri is empty",
			input: "{\"id\":\"r1\",\"timeout\":\"30s\",\"predicates\":[{\"name\":\"p1\"}],\"filters\":[{\"name\":\"f1\"}]}",
			expected: config.Route{
				ID: "r1",
				Predicates: []config.ParameterizedItem{
					{
						Name: "p1",
					},
				},
				Filters: []config.ParameterizedItem{
					{
						Name: "f1",
					},
				},
				Timeout: config.Duration{Duration: 30 * time.Second},
			},
			expectedErr: errors.New("Key: 'Route.URI' Error:Field validation for 'URI' failed on the 'required' tag"),
		},
		{
			name:  "unmarshal and validate should return error when input is valid and id is empty",
			input: "{\"uri\":\"someUri\",\"timeout\":\"30s\",\"predicates\":[{\"name\":\"p1\"}],\"filters\":[{\"name\":\"f1\"}]}",
			expected: config.Route{
				URI: "someUri",
				Predicates: []config.ParameterizedItem{
					{
						Name: "p1",
					},
				},
				Filters: []config.ParameterizedItem{
					{
						Name: "f1",
					},
				},
				Timeout: config.Duration{Duration: 30 * time.Second},
			},
			expectedErr: errors.New("Key: 'Route.ID' Error:Field validation for 'ID' failed on the 'required' tag"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var poolConfig config.Route
			err := json.Unmarshal([]byte(tt.input), &poolConfig)
			if err != nil {
				t.Errorf("expected no error actual %s", err)
			}
			if !reflect.DeepEqual(tt.expected, poolConfig) {
				t.Errorf("expected %v actual %v", tt.expected, poolConfig)
			}
			validate := validator.New()
			err = validate.Struct(poolConfig)
			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}

func TestRoute_ValidateYAML(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    config.Route
		expectedErr error
	}{
		{
			name:  "unmarshal and validate should succeed when input is valid",
			input: "{\"id\":\"r1\",\"uri\":\"someUri\",\"timeout\":\"30s\",\"predicates\":[{\"name\":\"p1\"}],\"filters\":[{\"name\":\"f1\"}]}",
			expected: config.Route{
				ID:  "r1",
				URI: "someUri",
				Predicates: []config.ParameterizedItem{
					{
						Name: "p1",
					},
				},
				Filters: []config.ParameterizedItem{
					{
						Name: "f1",
					},
				},
				Timeout: config.Duration{Duration: 30 * time.Second},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and filters are empty",
			input: "{\"id\":\"r1\",\"uri\":\"someUri\",\"timeout\":\"30s\",\"predicates\":[{\"name\":\"p1\"}]}",
			expected: config.Route{
				ID:  "r1",
				URI: "someUri",
				Predicates: []config.ParameterizedItem{
					{
						Name: "p1",
					},
				},
				Timeout: config.Duration{Duration: 30 * time.Second},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should succeed when input is valid and predicates are empty",
			input: "{\"id\":\"r1\",\"uri\":\"someUri\",\"timeout\":\"30s\",\"filters\":[{\"name\":\"f1\"}]}",
			expected: config.Route{
				ID:  "r1",
				URI: "someUri",
				Filters: []config.ParameterizedItem{
					{
						Name: "f1",
					},
				},
				Timeout: config.Duration{Duration: 30 * time.Second},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should succeed when timeout is empty",
			input: "{\"id\":\"r1\",\"uri\":\"someUri\",\"predicates\":[{\"name\":\"p1\"}],\"filters\":[{\"name\":\"f1\"}]}",
			expected: config.Route{
				ID:  "r1",
				URI: "someUri",
				Predicates: []config.ParameterizedItem{
					{
						Name: "p1",
					},
				},
				Filters: []config.ParameterizedItem{
					{
						Name: "f1",
					},
				},
				Timeout: config.Duration{},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should return error when input is valid and uri is empty",
			input: "{\"id\":\"r1\",\"timeout\":\"30s\",\"predicates\":[{\"name\":\"p1\"}],\"filters\":[{\"name\":\"f1\"}]}",
			expected: config.Route{
				ID: "r1",
				Predicates: []config.ParameterizedItem{
					{
						Name: "p1",
					},
				},
				Filters: []config.ParameterizedItem{
					{
						Name: "f1",
					},
				},
				Timeout: config.Duration{Duration: 30 * time.Second},
			},
			expectedErr: errors.New("Key: 'Route.URI' Error:Field validation for 'URI' failed on the 'required' tag"),
		},
		{
			name:  "unmarshal and validate should return error when input is valid and id is empty",
			input: "{\"uri\":\"someUri\",\"timeout\":\"30s\",\"predicates\":[{\"name\":\"p1\"}],\"filters\":[{\"name\":\"f1\"}]}",
			expected: config.Route{
				URI: "someUri",
				Predicates: []config.ParameterizedItem{
					{
						Name: "p1",
					},
				},
				Filters: []config.ParameterizedItem{
					{
						Name: "f1",
					},
				},
				Timeout: config.Duration{Duration: 30 * time.Second},
			},
			expectedErr: errors.New("Key: 'Route.ID' Error:Field validation for 'ID' failed on the 'required' tag"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var poolConfig config.Route
			err := yaml.Unmarshal([]byte(tt.input), &poolConfig)
			if err != nil {
				t.Errorf("expected no error actual %s", err)
			}
			if !reflect.DeepEqual(tt.expected, poolConfig) {
				t.Errorf("expected %v actual %v", tt.expected, poolConfig)
			}
			validate := validator.New()
			err = validate.Struct(poolConfig)
			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}

func TestGateway_ValidateJSON(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    config.Gateway
		expectedErr error
	}{
		{
			name:  "unmarshal and validate should succeed",
			input: "{\"routes\":[{\"id\":\"r1\",\"uri\":\"someUri\"}],\"global-filters\":[{\"name\":\"f1\"}],\"global-timeout\":\"30s\",\"httpclient\":{}}",
			expected: config.Gateway{
				Routes: []config.Route{
					{
						ID:  "r1",
						URI: "someUri",
					},
				},
				GlobalFilters: []config.ParameterizedItem{
					{Name: "f1"},
				},
				GlobalTimeout: config.Duration{Duration: 30 * time.Second},
				HTTPClient:    &config.HTTPClient{},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should succeed when httpclient is not present",
			input: "{\"routes\":[{\"id\":\"r1\",\"uri\":\"someUri\"}],\"global-filters\":[{\"name\":\"f1\"}],\"global-timeout\":\"30s\"}",
			expected: config.Gateway{
				Routes: []config.Route{
					{
						ID:  "r1",
						URI: "someUri",
					},
				},
				GlobalFilters: []config.ParameterizedItem{
					{Name: "f1"},
				},
				GlobalTimeout: config.Duration{Duration: 30 * time.Second},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should succeed when global-timeout is not present",
			input: "{\"routes\":[{\"id\":\"r1\",\"uri\":\"someUri\"}],\"global-filters\":[{\"name\":\"f1\"}],\"httpclient\":{}}",
			expected: config.Gateway{
				Routes: []config.Route{
					{
						ID:  "r1",
						URI: "someUri",
					},
				},
				GlobalFilters: []config.ParameterizedItem{
					{Name: "f1"},
				},
				HTTPClient: &config.HTTPClient{},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should return error when global-filter is invalid",
			input: "{\"routes\":[{\"id\":\"r1\",\"uri\":\"someUri\"}],\"global-filters\":[{}],\"global-timeout\":\"30s\",\"httpclient\":{}}",
			expected: config.Gateway{
				Routes: []config.Route{
					{
						ID:  "r1",
						URI: "someUri",
					},
				},
				GlobalFilters: []config.ParameterizedItem{
					{},
				},
				GlobalTimeout: config.Duration{Duration: 30 * time.Second},
				HTTPClient:    &config.HTTPClient{},
			},
			expectedErr: errors.New("Key: 'Gateway.GlobalFilters[0].Name' Error:Field validation for 'Name' failed on the 'required' tag"),
		},
		{
			name:  "unmarshal and validate should succeed when global-filters is not present",
			input: "{\"routes\":[{\"id\":\"r1\",\"uri\":\"someUri\"}],\"global-timeout\":\"30s\",\"httpclient\":{}}",
			expected: config.Gateway{
				Routes: []config.Route{
					{
						ID:  "r1",
						URI: "someUri",
					},
				},
				GlobalTimeout: config.Duration{Duration: 30 * time.Second},
				HTTPClient:    &config.HTTPClient{},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should return error when route is invalid",
			input: "{\"routes\":[{\"id\":\"r1\"}],\"global-filters\":[{\"name\":\"f1\"}],\"global-timeout\":\"30s\",\"httpclient\":{}}",
			expected: config.Gateway{
				Routes: []config.Route{
					{
						ID: "r1",
					},
				},
				GlobalFilters: []config.ParameterizedItem{
					{Name: "f1"},
				},
				GlobalTimeout: config.Duration{Duration: 30 * time.Second},
				HTTPClient:    &config.HTTPClient{},
			},
			expectedErr: errors.New("Key: 'Gateway.Routes[0].URI' Error:Field validation for 'URI' failed on the 'required' tag"),
		},
		{
			name:  "unmarshal and validate should return error when routes is empty",
			input: "{\"routes\":[],\"global-filters\":[{\"name\":\"f1\"}],\"global-timeout\":\"30s\",\"httpclient\":{}}",
			expected: config.Gateway{
				Routes: []config.Route{},
				GlobalFilters: []config.ParameterizedItem{
					{Name: "f1"},
				},
				GlobalTimeout: config.Duration{Duration: 30 * time.Second},
				HTTPClient:    &config.HTTPClient{},
			},
			expectedErr: errors.New("Key: 'Gateway.Routes' Error:Field validation for 'Routes' failed on the 'min' tag"),
		},
		{
			name:  "unmarshal and validate should return error when routes is not present",
			input: "{\"global-filters\":[{\"name\":\"f1\"}],\"global-timeout\":\"30s\",\"httpclient\":{}}",
			expected: config.Gateway{
				GlobalFilters: []config.ParameterizedItem{
					{Name: "f1"},
				},
				GlobalTimeout: config.Duration{Duration: 30 * time.Second},
				HTTPClient:    &config.HTTPClient{},
			},
			expectedErr: errors.New("Key: 'Gateway.Routes' Error:Field validation for 'Routes' failed on the 'required' tag"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var poolConfig config.Gateway
			err := json.Unmarshal([]byte(tt.input), &poolConfig)
			if err != nil {
				t.Errorf("expected no error actual %s", err)
			}
			if !reflect.DeepEqual(tt.expected, poolConfig) {
				t.Errorf("expected %v actual %v", tt.expected, poolConfig)
			}
			validate := validator.New()
			err = validate.Struct(poolConfig)
			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}

func TestGateway_ValidateYAML(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    config.Gateway
		expectedErr error
	}{
		{
			name:  "unmarshal and validate should succeed",
			input: "{\"routes\":[{\"id\":\"r1\",\"uri\":\"someUri\"}],\"global-filters\":[{\"name\":\"f1\"}],\"global-timeout\":\"30s\",\"httpclient\":{}}",
			expected: config.Gateway{
				Routes: []config.Route{
					{
						ID:  "r1",
						URI: "someUri",
					},
				},
				GlobalFilters: []config.ParameterizedItem{
					{Name: "f1"},
				},
				GlobalTimeout: config.Duration{Duration: 30 * time.Second},
				HTTPClient:    &config.HTTPClient{},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should succeed when httpclient is not present",
			input: "{\"routes\":[{\"id\":\"r1\",\"uri\":\"someUri\"}],\"global-filters\":[{\"name\":\"f1\"}],\"global-timeout\":\"30s\"}",
			expected: config.Gateway{
				Routes: []config.Route{
					{
						ID:  "r1",
						URI: "someUri",
					},
				},
				GlobalFilters: []config.ParameterizedItem{
					{Name: "f1"},
				},
				GlobalTimeout: config.Duration{Duration: 30 * time.Second},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should succeed when global-timeout is not present",
			input: "{\"routes\":[{\"id\":\"r1\",\"uri\":\"someUri\"}],\"global-filters\":[{\"name\":\"f1\"}],\"httpclient\":{}}",
			expected: config.Gateway{
				Routes: []config.Route{
					{
						ID:  "r1",
						URI: "someUri",
					},
				},
				GlobalFilters: []config.ParameterizedItem{
					{Name: "f1"},
				},
				HTTPClient: &config.HTTPClient{},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should return error when global-filter is invalid",
			input: "{\"routes\":[{\"id\":\"r1\",\"uri\":\"someUri\"}],\"global-filters\":[{}],\"global-timeout\":\"30s\",\"httpclient\":{}}",
			expected: config.Gateway{
				Routes: []config.Route{
					{
						ID:  "r1",
						URI: "someUri",
					},
				},
				GlobalFilters: []config.ParameterizedItem{
					{},
				},
				GlobalTimeout: config.Duration{Duration: 30 * time.Second},
				HTTPClient:    &config.HTTPClient{},
			},
			expectedErr: errors.New("Key: 'Gateway.GlobalFilters[0].Name' Error:Field validation for 'Name' failed on the 'required' tag"),
		},
		{
			name:  "unmarshal and validate should succeed when global-filters is not present",
			input: "{\"routes\":[{\"id\":\"r1\",\"uri\":\"someUri\"}],\"global-timeout\":\"30s\",\"httpclient\":{}}",
			expected: config.Gateway{
				Routes: []config.Route{
					{
						ID:  "r1",
						URI: "someUri",
					},
				},
				GlobalTimeout: config.Duration{Duration: 30 * time.Second},
				HTTPClient:    &config.HTTPClient{},
			},
			expectedErr: nil,
		},
		{
			name:  "unmarshal and validate should return error when route is invalid",
			input: "{\"routes\":[{\"id\":\"r1\"}],\"global-filters\":[{\"name\":\"f1\"}],\"global-timeout\":\"30s\",\"httpclient\":{}}",
			expected: config.Gateway{
				Routes: []config.Route{
					{
						ID: "r1",
					},
				},
				GlobalFilters: []config.ParameterizedItem{
					{Name: "f1"},
				},
				GlobalTimeout: config.Duration{Duration: 30 * time.Second},
				HTTPClient:    &config.HTTPClient{},
			},
			expectedErr: errors.New("Key: 'Gateway.Routes[0].URI' Error:Field validation for 'URI' failed on the 'required' tag"),
		},
		{
			name:  "unmarshal and validate should return error when routes is empty",
			input: "{\"routes\":[],\"global-filters\":[{\"name\":\"f1\"}],\"global-timeout\":\"30s\",\"httpclient\":{}}",
			expected: config.Gateway{
				Routes: []config.Route{},
				GlobalFilters: []config.ParameterizedItem{
					{Name: "f1"},
				},
				GlobalTimeout: config.Duration{Duration: 30 * time.Second},
				HTTPClient:    &config.HTTPClient{},
			},
			expectedErr: errors.New("Key: 'Gateway.Routes' Error:Field validation for 'Routes' failed on the 'min' tag"),
		},
		{
			name:  "unmarshal and validate should return error when routes is not present",
			input: "{\"global-filters\":[{\"name\":\"f1\"}],\"global-timeout\":\"30s\",\"httpclient\":{}}",
			expected: config.Gateway{
				GlobalFilters: []config.ParameterizedItem{
					{Name: "f1"},
				},
				GlobalTimeout: config.Duration{Duration: 30 * time.Second},
				HTTPClient:    &config.HTTPClient{},
			},
			expectedErr: errors.New("Key: 'Gateway.Routes' Error:Field validation for 'Routes' failed on the 'required' tag"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var poolConfig config.Gateway
			err := yaml.Unmarshal([]byte(tt.input), &poolConfig)
			if err != nil {
				t.Errorf("expected no error actual %s", err)
			}
			if !reflect.DeepEqual(tt.expected, poolConfig) {
				t.Errorf("expected %v actual %v", tt.expected, poolConfig)
			}
			validate := validator.New()
			err = validate.Struct(poolConfig)
			if fmt.Sprintf("%s", tt.expectedErr) != fmt.Sprintf("%s", err) {
				t.Errorf("expected err %s actual %s", tt.expectedErr, err)
			}
		})
	}
}
