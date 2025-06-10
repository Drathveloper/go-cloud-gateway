package config

// Reader is a reader for config files.
type Reader interface {
	// Read reads the given input and returns the config or error if config is invalid.
	Read(input []byte) (*Config, error)
}
