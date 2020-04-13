package prometheus

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"runtime/debug"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prysmaticlabs/prysm/shared"
	"github.com/sirupsen/logrus"
)

var log = logrus.WithField("prefix", "prometheus")

// Service provides Prometheus metrics via the /metrics route. This route will
// show all the metrics registered with the Prometheus DefaultRegisterer.
type Service struct {
	server      *http.Server
	svcRegistry *shared.ServiceRegistry
	failStatus  error
}

// Handler represents a path and handler func to serve on the same port as /metrics, /healthz, /goroutinez, etc.
type Handler struct {
	Path    string
	Handler func(http.ResponseWriter, *http.Request)
}

// NewPrometheusService sets up a new instance for a given address host:port.
// An empty host will match with any IP so an address like ":2121" is perfectly acceptable.
func NewPrometheusService(addr string, svcRegistry *shared.ServiceRegistry, additionalHandlers ...Handler) *Service {
	s := &Service{svcRegistry: svcRegistry}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", s.healthzHandler)
	mux.HandleFunc("/goroutinez", s.goroutinezHandler)

	// Register additional handlers.
	for _, h := range additionalHandlers {
		mux.HandleFunc(h.Path, h.Handler)
	}

	s.server = &http.Server{Addr: addr, Handler: mux}

	return s
}

func (s *Service) healthzHandler(w http.ResponseWriter, _ *http.Request) {
	// Call all services in the registry.
	// if any are not OK, write 500
	// print the statuses of all services.

	statuses := s.svcRegistry.Statuses()
	hasError := false
	var buf bytes.Buffer
	for k, v := range statuses {
		var status string
		if v == nil {
			status = "OK"
		} else {
			hasError = true
			status = "ERROR " + v.Error()
		}

		if _, err := buf.WriteString(fmt.Sprintf("%s: %s\n", k, status)); err != nil {
			hasError = true
		}
	}

	// Write status header
	if hasError {
		w.WriteHeader(http.StatusInternalServerError)
		log.WithField("statuses", buf.String()).Warn("Node is unhealthy!")
	} else {
		w.WriteHeader(http.StatusOK)
	}

	// Write http body
	if _, err := w.Write(buf.Bytes()); err != nil {
		log.Errorf("Could not write healthz body %v", err)
	}
}

func (s *Service) goroutinezHandler(w http.ResponseWriter, _ *http.Request) {
	stack := debug.Stack()
	if _, err := w.Write(stack); err != nil {
		log.WithError(err).Error("Failed to write goroutines stack")
	}
	if err := pprof.Lookup("goroutine").WriteTo(w, 2); err != nil {
		log.WithError(err).Error("Failed to write pprof goroutines")
	}
}

// Start the prometheus service.
func (s *Service) Start() {
	go func() {
		// See if the port is already used.
		addrParts := strings.Split(s.server.Addr, ":")
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%s", addrParts[1]), time.Second)
		if err == nil {
			if err := conn.Close(); err != nil {
				log.WithError(err).Error("Failed to close connection")
			}
			// Something on the port; we cannot use it.
			log.WithField("address", s.server.Addr).Warn("Port already in use; cannot start prometheus service")
		} else {
			// Nothing on that port; we can use it.
			log.WithField("address", s.server.Addr).Debug("Starting prometheus service")
			err := s.server.ListenAndServe()
			if err != nil && err != http.ErrServerClosed {
				log.Errorf("Could not listen to host:port :%s: %v", s.server.Addr, err)
				s.failStatus = err
			}
		}
	}()
}

// Stop the service gracefully.
func (s *Service) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}

// Status checks for any service failure conditions.
func (s *Service) Status() error {
	if s.failStatus != nil {
		return s.failStatus
	}
	return nil
}
