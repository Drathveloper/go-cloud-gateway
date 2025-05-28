package config

import (
	"fmt"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

type ReaderYAML struct {
	validate *validator.Validate
}

func NewReaderYAML(validate *validator.Validate) *ReaderYAML {
	return &ReaderYAML{
		validate: validate,
	}
}

func (r *ReaderYAML) Read(input []byte) (*Config, error) {
	baseErr := "read yaml config failed"
	var cfg Config
	if err := yaml.Unmarshal(input, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", baseErr, err)
	}
	if err := r.validate.Struct(cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", baseErr, err)
	}
	return &cfg, nil
}
