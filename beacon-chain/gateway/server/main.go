package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"

	gwruntime "github.com/grpc-ecosystem/grpc-gateway/runtime"
	joonix "github.com/joonix/log"
	"github.com/prysmaticlabs/prysm/beacon-chain/gateway"
	"github.com/sirupsen/logrus"
)

// Endpoint describes a gRPC endpoint
type Endpoint struct {
	Network, Addr string
}

// Options is a set of options to be passed to Run
type Options struct {
	// Addr is the address to listen
	Addr string

	// GRPCServer defines an endpoint of a gRPC service
	GRPCServer Endpoint

	// SwaggerDir is a path to a directory from which the server
	// serves swagger specs.
	SwaggerDir string

	// Mux is a list of options to be passed to the grpc-gateway multiplexer
	Mux []gwruntime.ServeMuxOption
}

var (
	beaconRPC = flag.String("beacon-rpc", "localhost:4000", "Beacon chain gRPC endpoint")
	port      = flag.Int("port", 8000, "Port to serve on")
	debug     = flag.Bool("debug", false, "Enable debug logging")
)

func init() {
	logrus.SetFormatter(joonix.NewFormatter())
}

var log = logrus.New()

func main() {
	flag.Parse()
	if *debug {
		log.SetLevel(logrus.DebugLevel)
	}

	mux := http.NewServeMux()
	gw := gateway.New(context.Background(), *beaconRPC, fmt.Sprintf("0.0.0.0:%d", *port), mux)
	mux.HandleFunc("/swagger/", gateway.SwaggerServer())
	mux.HandleFunc("/healthz", healthzServer(gw))
	gw.Start()

	select {}
}

// healthzServer returns a simple health handler which returns ok.
func healthzServer(gw *gateway.Gateway) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		if err := gw.Status(); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if _, err := fmt.Fprintln(w, "ok"); err != nil {
			log.WithError(err).Error("failed to respond to healthz")
		}
	}
}
