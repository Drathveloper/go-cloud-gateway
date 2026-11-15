package config

import (
	"encoding/json"
	"fmt"

	"github.com/go-playground/validator/v10"
)

// ReaderJSON is a reader for json files.
type ReaderJSON struct {
	validate *validator.Validate
}

// NewReaderJSON creates a new reader for json files.
//
// The validator checks the validate tags of the config types. Nil means a
// validator.New(), which is also what Validate uses.
func NewReaderJSON(validate *validator.Validate) *ReaderJSON {
	return &ReaderJSON{
		validate: validate,
	}
}

// Read reads the given input and returns the config or error if config is invalid.
// A valid config is one that Validate accepts.
func (r *ReaderJSON) Read(input []byte) (*Config, error) {
	baseErr := "read json config failed"
	var cfg Config
	if err := json.Unmarshal(input, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", baseErr, err)
	}
	if err := validateConfig(r.validate, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", baseErr, err)
	}
	return &cfg, nil
}
