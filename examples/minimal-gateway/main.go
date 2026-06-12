package main

import (
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"strconv"

	"github.com/drathveloper/go-cloud-gateway/pkg/bootstrap"
	"github.com/drathveloper/go-cloud-gateway/pkg/config"
	"github.com/go-playground/validator/v10"
)

const requiredArgsLen = 2

func main() {
	if len(os.Args) < requiredArgsLen {
		log.Fatal("config file argument is required")
	}
	startPprof()
	cfg := readConfigFile(os.Args[1])
	builder := bootstrap.NewOptionsBuilder(cfg)
	if port := os.Getenv("GATEWAY_PORT"); port != "" {
		portNum, err := strconv.Atoi(port)
		if err != nil {
			log.Fatalf("invalid GATEWAY_PORT %q: %s", port, err)
		}
		builder.WithServerOptions(bootstrap.ServerOpts{Port: portNum})
	}
	server, err := bootstrap.Initialize(builder.Build())
	if err != nil {
		log.Fatalf("gateway initialization failed: %s", err)
	}
	log.Fatal(server.ListenAndServe())
}

// startPprof exposes net/http/pprof on PPROF_ADDR (e.g. "localhost:6060").
// Disabled unless the environment variable is set.
func startPprof() {
	addr := os.Getenv("PPROF_ADDR")
	if addr == "" {
		return
	}
	go func() {
		log.Printf("pprof listening on http://%s/debug/pprof/", addr)
		log.Println(http.ListenAndServe(addr, nil)) //nolint:gosec
	}()
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
