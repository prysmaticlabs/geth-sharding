package tracing

import (
	"errors"

	"contrib.go.opencensus.io/exporter/jaeger"
	"github.com/prysmaticlabs/prysm/shared/version"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

var log = logrus.WithField("prefix", "tracing")

// Setup creates and initializes a new tracing configuration..
func Setup(serviceName, processName, endpoint string, sampleFraction float64, enable bool) error {
	if !enable {
		trace.ApplyConfig(trace.Config{DefaultSampler: trace.NeverSample()})
		return nil
	}

	if serviceName == "" {
		return errors.New("tracing service name cannot be empty")
	}

	trace.ApplyConfig(trace.Config{
		DefaultSampler:          trace.ProbabilitySampler(sampleFraction),
		MaxMessageEventsPerSpan: 500,
	})

	log.Infof("Starting Jaeger exporter endpoint at address = %s", endpoint)
	exporter, err := jaeger.NewExporter(jaeger.Options{
		CollectorEndpoint: endpoint,
		Process: jaeger.Process{
			ServiceName: serviceName,
			Tags: []jaeger.Tag{
				jaeger.StringTag("process_name", processName),
				jaeger.StringTag("version", version.GetVersion()),
			},
		},
		BufferMaxCount: 10000,
		OnError: func(err error) {
			log.WithError(err).Error("Failed to process span")
		},
	})
	if err != nil {
		return err
	}
	trace.RegisterExporter(exporter)

	return nil
}
