package config

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

// ReaderYAML is a reader for yaml files.
type ReaderYAML struct {
	validate *validator.Validate
}

// NewReaderYAML creates a new reader for yaml files.
//
// The validator checks the validate tags of the config types. Nil means a
// validator.New(), which is also what Validate uses.
func NewReaderYAML(validate *validator.Validate) *ReaderYAML {
	return &ReaderYAML{
		validate: validate,
	}
}

// Read reads the given input and returns the config or error if config is invalid.
// A valid config is one that Validate accepts.
func (r *ReaderYAML) Read(input []byte) (*Config, error) {
	baseErr := "read yaml config failed"
	var cfg Config
	if err := yaml.Unmarshal(input, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", baseErr, err)
	}
	if err := validateConfig(r.validate, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", baseErr, err)
	}
	return &cfg, nil
}
