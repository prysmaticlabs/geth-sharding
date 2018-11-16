package metric

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/p2p"
	"github.com/prysmaticlabs/prysm/shared/prometheus"
)

const addr = "127.0.0.1:8989"

func TestMessageMetrics(t *testing.T) {
	service := prometheus.NewPrometheusService(addr)
	go service.Start()
	defer service.Stop()

	adapter := New()
	if adapter == nil {
		t.Error("Expected metric adapter")
	}
	data := &pb.AggregatedAttestation{
		Slot:             0,
		AttesterBitfield: []byte{99},
	}
	h := adapter(func(p2p.Message) { time.Sleep(10 * time.Millisecond) })
	h(p2p.Message{Ctx: context.Background(), Data: data})
	h = adapter(func(p2p.Message) { time.Sleep(100 * time.Microsecond) })
	h(p2p.Message{Ctx: context.Background(), Data: data})

	metrics := getMetrics(t)
	testMetricExists(t, metrics, fmt.Sprintf("p2p_message_sent_total{message=\"%T\"} 2", data))
	testMetricExists(t, metrics, fmt.Sprintf("p2p_message_sent_latency_seconds_bucket{message=\"%T\",le=\"0.005\"} 1", data))
	testMetricExists(t, metrics, fmt.Sprintf("p2p_message_sent_latency_seconds_bucket{message=\"%T\",le=\"0.01\"} 1", data))
	testMetricExists(t, metrics, fmt.Sprintf("p2p_message_received_bytes_bucket{message=\"%T\",le=\"32\"} 2", data))
}

func getMetrics(t *testing.T) []string {
	resp, err := http.Get(fmt.Sprintf("http://%s/metrics", addr))
	if err != nil {
		t.Error(err)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Error(err)
	}
	return strings.Split(string(body), "\n")
}

func testMetricExists(t *testing.T, metrics []string, pattern string) string {
	for _, line := range metrics {
		if strings.HasPrefix(line, pattern) {
			return line
		}
	}
	t.Errorf("Pattern \"%s\" not found", pattern)
	return ""
}
