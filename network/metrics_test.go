package network

import (
	"context"
	"os"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

var testMetricReader *metric.ManualReader

func TestMain(m *testing.M) {
	testMetricReader = metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(testMetricReader))
	otel.SetMeterProvider(provider)

	code := m.Run()
	_ = provider.Shutdown(context.Background())
	os.Exit(code)
}

func TestMetricsRecorderRecordsNetworkEvents(t *testing.T) {
	recorder := NewMetricsRecorder("tcp", NetServer)
	recorder.Receive(128, true)
	recorder.Receive(5, false)
	recorder.Send(64, true)
	recorder.Send(5, false)
	recorder.Error(OperationRead, ErrorTypeIO)
	recorder.ConnectionOpened()
	recorder.ConnectionClosed()

	data := collectMetrics(t)

	message := metricByName(t, data, "network_message")
	messageSum, ok := message.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("network_message data = %T, want Sum[int64]", message.Data)
	}
	if got := sumByDirection(messageSum, "receive"); got != 1 {
		t.Fatalf("received message count = %d, want 1", got)
	}
	if got := sumByDirection(messageSum, "send"); got != 1 {
		t.Fatalf("sent message count = %d, want 1", got)
	}

	if got := sumMetric[int64](t, data, "network_receive_bytes"); got != 133 {
		t.Fatalf("received bytes = %d, want 133", got)
	}
	if got := sumMetric[int64](t, data, "network_send_bytes"); got != 69 {
		t.Fatalf("sent bytes = %d, want 69", got)
	}
	if got := sumMetric[int64](t, data, "network_error"); got != 1 {
		t.Fatalf("error count = %d, want 1", got)
	}
	if got := sumMetric[int64](t, data, "network_connection_open"); got != 1 {
		t.Fatalf("opened connection count = %d, want 1", got)
	}
	if got := sumMetric[int64](t, data, "network_connection_close"); got != 1 {
		t.Fatalf("closed connection count = %d, want 1", got)
	}
	if got := sumMetric[int64](t, data, "network_connection_active"); got != 0 {
		t.Fatalf("active connections = %d, want 0", got)
	}

	if got := message.Unit; got != "{message}" {
		t.Fatalf("message unit = %q, want {message}", got)
	}
	if got := metricByName(t, data, "network_receive_bytes").Unit; got != "By" {
		t.Fatalf("receive bytes unit = %q, want By", got)
	}
}

func TestMetricsRecorderIgnoresNonPositiveMeasurements(t *testing.T) {
	recorder := NewMetricsRecorder("tcp", NetClient)
	recorder.Receive(0, true)
	recorder.Send(-1, true)

	data := collectMetrics(t)
	if got := sumMetricWithAttribute[int64](t, data, "network_receive_bytes", "network.role", "client"); got != 0 {
		t.Fatalf("received bytes = %d, want 0", got)
	}
	if got := sumMetricWithAttribute[int64](t, data, "network_send_bytes", "network.role", "client"); got != 0 {
		t.Fatalf("sent bytes = %d, want 0", got)
	}

	if got := sumMessageByAttributes(t, data, "client", "receive"); got != 0 {
		t.Fatalf("received message count = %d, want 0", got)
	}
	if got := sumMessageByAttributes(t, data, "client", "send"); got != 0 {
		t.Fatalf("sent message count = %d, want 0", got)
	}
}

func TestNewMetricsRecorderReusesRecorderByTransportAndRole(t *testing.T) {
	server := NewMetricsRecorder("tcp-cache-test", NetServer)
	if got := NewMetricsRecorder("tcp-cache-test", NetServer); got != server {
		t.Fatal("same transport and role must reuse metrics recorder")
	}
	if got := NewMetricsRecorder("tcp-cache-test", NetClient); got == server {
		t.Fatal("different role must use a different metrics recorder")
	}
	if got := NewMetricsRecorder("ws-cache-test", NetServer); got == server {
		t.Fatal("different transport must use a different metrics recorder")
	}
}

func collectMetrics(t *testing.T) metricdata.ResourceMetrics {
	t.Helper()
	var data metricdata.ResourceMetrics
	if err := testMetricReader.Collect(context.Background(), &data); err != nil {
		t.Fatalf("collect metrics: %v", err)
	}
	return data
}

func metricByName(t *testing.T, data metricdata.ResourceMetrics, name string) metricdata.Metrics {
	t.Helper()
	for _, scope := range data.ScopeMetrics {
		if scope.Scope.Name != "due.network" {
			continue
		}
		for _, m := range scope.Metrics {
			if m.Name == name {
				return m
			}
		}
	}
	t.Fatalf("metric %q was not collected", name)
	return metricdata.Metrics{}
}

func sumMetric[T int64 | float64](t *testing.T, data metricdata.ResourceMetrics, name string) T {
	return sumMetricWithAttribute[T](t, data, name, "", "")
}

func sumMetricWithAttribute[T int64 | float64](t *testing.T, data metricdata.ResourceMetrics, name, key, want string) T {
	t.Helper()
	m := metricByName(t, data, name)
	switch sum := m.Data.(type) {
	case metricdata.Sum[int64]:
		var total int64
		for _, point := range sum.DataPoints {
			if key != "" {
				value, ok := point.Attributes.Value(attribute.Key(key))
				if !ok || value.AsString() != want {
					continue
				}
			}
			total += point.Value
		}
		return T(total)
	case metricdata.Sum[float64]:
		var total float64
		for _, point := range sum.DataPoints {
			if key != "" {
				value, ok := point.Attributes.Value(attribute.Key(key))
				if !ok || value.AsString() != want {
					continue
				}
			}
			total += point.Value
		}
		return T(total)
	default:
		t.Fatalf("metric %q data = %T, want Sum", name, m.Data)
		return 0
	}
}

func sumByDirection(sum metricdata.Sum[int64], direction string) int64 {
	var total int64
	for _, point := range sum.DataPoints {
		if value, ok := point.Attributes.Value("network.direction"); ok && value.AsString() == direction {
			total += point.Value
		}
	}
	return total
}

func sumMessageByAttributes(t *testing.T, data metricdata.ResourceMetrics, role, direction string) int64 {
	t.Helper()
	message := metricByName(t, data, "network_message")
	messageSum, ok := message.Data.(metricdata.Sum[int64])
	if !ok {
		t.Fatalf("network_message data = %T, want Sum[int64]", message.Data)
	}

	var total int64
	for _, point := range messageSum.DataPoints {
		if value, ok := point.Attributes.Value("network.role"); !ok || value.AsString() != role {
			continue
		}
		if value, ok := point.Attributes.Value("network.direction"); !ok || value.AsString() != direction {
			continue
		}
		total += point.Value
	}
	return total
}
