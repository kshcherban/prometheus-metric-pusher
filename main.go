package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang/snappy"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/prompb"
)

// KV is a type that implements the flag.Value interface for key=value flags
type KV []string

// String returns a string representation of the value
func (kv *KV) String() string {
	return fmt.Sprint(*kv)
}

// Set adds a value to the slice so we can repeat flag
func (kv *KV) Set(value string) error {
	*kv = append(*kv, value)
	return nil
}

func main() {
	// Define cli flags
	metricName := flag.String("metric", "example_metric", "metric name")
	metricValue := flag.Float64("value", 0.0, "metric value, float")
	promUrl := flag.String("url", "http://localhost:9090/api/v1/write", "prometheus url")
	var metricLabels KV
	flag.Var(&metricLabels, "label", "labels in a format label=value, can be repeated")
	flag.Parse()

	// Set metric variable
	var (
		metric = promauto.NewGauge(prometheus.GaugeOpts{
			Name: *metricName,
			Help: "An example metric",
		})
	)

	// Set the value of the metric
	metric.Set(*metricValue)

	// Define all labels
	protoLabels := []prompb.Label{
		{
			Name:  "__name__",
			Value: *metricName,
		},
	}

	// Parse key=value labels from cli arguments into protobuf struct
	for _, kv := range metricLabels {
		parts := strings.Split(kv, "=")
		protoLabels = append(protoLabels, prompb.Label{
			Name:  parts[0],
			Value: parts[1],
		})
	}

	// Create a Prometheus TimeSeries for the metric
	ts := prompb.TimeSeries{
		Labels: protoLabels,
		Samples: []prompb.Sample{
			{
				Value:     *metricValue,
				Timestamp: int64(model.TimeFromUnix(time.Now().Unix())),
			},
		},
	}

	// Create a Prometheus WriteRequest
	req := &prompb.WriteRequest{
		Timeseries: make([]prompb.TimeSeries, 0, 0),
	}

	req.Timeseries = append(req.Timeseries, ts)

	// Compress and encode the WriteRequest
	data, err := req.Marshal()
	if err != nil {
		log.Fatalf("error marshaling request: %v", err)
	}
	compressed := snappy.Encode(nil, data)

	// Push the WriteRequest to Prometheus
	resp, err := http.Post(*promUrl, "application/x-protobuf", bytes.NewBuffer(compressed))
	if err != nil {
		log.Fatalf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Check the response from Prometheus
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("error reading response: %v", err)
	}
	// Prometheus returns 204 status code on success
	if resp.StatusCode != http.StatusNoContent {
		log.Fatalf("unexpected status code: %d, body: %s", resp.StatusCode, b)
	} else {
		log.Printf("wrote metric: %v=%v", *metricName, *metricValue)
	}
}
