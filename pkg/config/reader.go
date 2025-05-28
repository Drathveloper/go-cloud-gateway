package config

type Reader interface {
	Read(input []byte) (*Config, error)
}
