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
func NewReaderJSON(validate *validator.Validate) *ReaderJSON {
	return &ReaderJSON{
		validate: validate,
	}
}

// Read reads the given input and returns the config or error if config is invalid.
func (r *ReaderJSON) Read(input []byte) (*Config, error) {
	baseErr := "read json config failed"
	var cfg Config
	if err := json.Unmarshal(input, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", baseErr, err)
	}
	if err := r.validate.Struct(&cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", baseErr, err)
	}
	return &cfg, nil
}
