package main

import (
	"log"
	"os"

	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/go-playground/validator/v10"
)

const requiredArgsLen = 2

func main() {
	if len(os.Args) < requiredArgsLen {
		log.Fatal("config file argument is required")
	}
	cfg := readConfigFile(os.Args[1])
	opts := bootstrap.NewOptionsBuilder(cfg).Build()
	server, err := bootstrap.Initialize(opts)
	if err != nil {
		log.Fatalf("gateway initialization failed: %s", err)
	}
	log.Fatal(server.ListenAndServe())
}

func readConfigFile(filename string) *config.Config {
	fileBytes, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("gateway initialization failed: %s", err)
	}
	validate := validator.New()
	configReader := config.NewReaderJSON(validate)
	cfg, err := configReader.Read(fileBytes)
	if err != nil {
		log.Fatalf("gateway initialization failed: %s", err)
	}
	return cfg
}
