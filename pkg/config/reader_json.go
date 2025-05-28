package config

import (
	"encoding/json"
	"fmt"

	"github.com/go-playground/validator/v10"
)

type ReaderJSON struct {
	validate *validator.Validate
}

func NewReaderJSON(validate *validator.Validate) *ReaderJSON {
	return &ReaderJSON{
		validate: validate,
	}
}

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
