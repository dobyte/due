package tcp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"testing"
	"time"

	dueerrors "github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
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

func TestTCPMetrics(t *testing.T) {
	before := collectMetrics(t)

	serverReceived := make(chan struct{}, 1)
	serverClosed := make(chan struct{}, 1)
	clientReceived := make(chan struct{}, 1)
	clientClosed := make(chan struct{}, 1)

	server := NewServer(
		WithServerAddr("127.0.0.1:0"),
		WithServerHeartbeatInterval(0),
		WithServerMetricsEnabled(true),
	).(*server)
	server.OnReceive(func(conn network.Conn, data []byte) {
		select {
		case serverReceived <- struct{}{}:
		default:
		}
		if err := conn.Send(data); err != nil {
			t.Errorf("echo message: %v", err)
		}
	})
	server.OnDisconnect(func(network.Conn) {
		select {
		case serverClosed <- struct{}{}:
		default:
		}
	})
	if err := server.Start(); err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer server.Stop()

	client := NewClient(
		WithClientAddr(server.listener.Addr().String()),
		WithClientHeartbeatInterval(0),
		WithClientMetricsEnabled(true),
	).(*client)
	client.OnReceive(func(network.Conn, []byte) {
		select {
		case clientReceived <- struct{}{}:
		default:
		}
	})
	client.OnDisconnect(func(network.Conn) {
		select {
		case clientClosed <- struct{}{}:
		default:
		}
	})

	conn, err := client.Dial()
	if err != nil {
		t.Fatalf("dial server: %v", err)
	}
	defer conn.Close(true)

	message, err := packet.PackMessage(&packet.Message{Route: 1, Buffer: []byte("hello")})
	if err != nil {
		t.Fatalf("pack message: %v", err)
	}

	if err := conn.Send(message); err != nil {
		t.Fatalf("send message: %v", err)
	}
	waitForSignal(t, serverReceived, "server receive")
	waitForSignal(t, clientReceived, "client receive")

	if err := conn.Close(true); err != nil {
		t.Fatalf("close client connection: %v", err)
	}
	waitForSignal(t, clientClosed, "client close")
	waitForSignal(t, serverClosed, "server close")

	after := collectMetrics(t)
	if got := deltaMetric(before, after, "network_message", "client", "send"); got != 1 {
		t.Fatalf("client sent messages = %d, want 1", got)
	}
	if got := deltaMetric(before, after, "network_message", "client", "receive"); got != 1 {
		t.Fatalf("client received messages = %d, want 1", got)
	}
	if got := deltaMetric(before, after, "network_message", "server", "receive"); got != 1 {
		t.Fatalf("server received messages = %d, want 1", got)
	}
	if got := deltaMetric(before, after, "network_message", "server", "send"); got != 1 {
		t.Fatalf("server sent messages = %d, want 1", got)
	}
	if got := deltaMetric(before, after, "network_send_bytes", "client", ""); got != int64(len(message)) {
		t.Fatalf("client sent bytes = %d, want %d", got, len(message))
	}
	if got := deltaMetric(before, after, "network_receive_bytes", "server", ""); got != int64(len(message)) {
		t.Fatalf("server received bytes = %d, want %d", got, len(message))
	}
	if got := deltaMetric(before, after, "network_send_bytes", "server", ""); got != int64(len(message)) {
		t.Fatalf("server sent bytes = %d, want %d", got, len(message))
	}
	if got := deltaMetric(before, after, "network_receive_bytes", "client", ""); got != int64(len(message)) {
		t.Fatalf("client received bytes = %d, want %d", got, len(message))
	}
	if got := deltaMetric(before, after, "network_connection_open", "client", ""); got != 1 {
		t.Fatalf("client opened connections = %d, want 1", got)
	}
	if got := deltaMetric(before, after, "network_connection_open", "server", ""); got != 1 {
		t.Fatalf("server opened connections = %d, want 1", got)
	}
	if got := deltaMetric(before, after, "network_connection_close", "client", ""); got != 1 {
		t.Fatalf("client closed connections = %d, want 1", got)
	}
	if got := deltaMetric(before, after, "network_connection_close", "server", ""); got != 1 {
		t.Fatalf("server closed connections = %d, want 1", got)
	}
	if got := deltaMetric(before, after, "network_error", "client", ""); got != 0 {
		t.Fatalf("client errors = %d, want 0", got)
	}
	if got := deltaMetric(before, after, "network_error", "server", ""); got != 0 {
		t.Fatalf("server errors = %d, want 0", got)
	}
}

func TestMetricsEnabledFollowsConfigAndOptions(t *testing.T) {
	const (
		clientEnabledKey = "etc.network.tcp.client.metrics.enable"
		serverEnabledKey = "etc.network.tcp.server.metrics.enable"
	)

	originalClientEnabled := etc.Get(clientEnabledKey).Bool()
	originalServerEnabled := etc.Get(serverEnabledKey).Bool()
	t.Cleanup(func() {
		if err := etc.Set(clientEnabledKey, originalClientEnabled); err != nil {
			t.Errorf("restore client metrics config: %v", err)
		}
		if err := etc.Set(serverEnabledKey, originalServerEnabled); err != nil {
			t.Errorf("restore server metrics config: %v", err)
		}
	})

	if err := etc.Set(clientEnabledKey, true); err != nil {
		t.Fatalf("enable client metrics config: %v", err)
	}
	if err := etc.Set(serverEnabledKey, false); err != nil {
		t.Fatalf("disable server metrics config: %v", err)
	}
	if got := NewServer().(*server).metrics; got != nil {
		t.Fatal("server metrics must be disabled by config")
	}
	if got := NewClient().(*client).metrics; got == nil {
		t.Fatal("client metrics must be enabled by config")
	}
	if got := NewServer(WithServerMetricsEnabled(true)).(*server).metrics; got == nil {
		t.Fatal("server option must enable metrics")
	}
	if got := NewClient(WithClientMetricsEnabled(false)).(*client).metrics; got != nil {
		t.Fatal("client option must disable metrics")
	}

	if err := etc.Set(clientEnabledKey, false); err != nil {
		t.Fatalf("disable client metrics config: %v", err)
	}
	if err := etc.Set(serverEnabledKey, true); err != nil {
		t.Fatalf("enable server metrics config: %v", err)
	}
	if got := NewServer().(*server).metrics; got == nil {
		t.Fatal("server metrics must be enabled by config")
	}
	if got := NewClient().(*client).metrics; got != nil {
		t.Fatal("client metrics must be disabled by config")
	}
	if got := NewServer(WithServerMetricsEnabled(false)).(*server).metrics; got != nil {
		t.Fatal("server option must disable metrics")
	}
	if got := NewClient(WithClientMetricsEnabled(true)).(*client).metrics; got == nil {
		t.Fatal("client option must enable metrics")
	}
}

func TestReadTCPMessageRecordsPartialAndEmptyFrames(t *testing.T) {
	frames := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{name: "empty frame", data: []byte{0, 0, 0, 0}},
		{name: "partial header", data: []byte{0, 0}, wantErr: true},
		{name: "partial payload", data: []byte{0, 0, 0, 3, 1}, wantErr: true},
	}

	for _, enabled := range []bool{false, true} {
		name := "disabled"
		var recorder *network.MetricsRecorder
		if enabled {
			name = "enabled"
			recorder = network.NewMetricsRecorder("tcp-test", network.NetClient)
		}

		t.Run(name, func(t *testing.T) {
			for _, frame := range frames {
				t.Run(frame.name, func(t *testing.T) {
					before := collectMetrics(t)
					left, right := net.Pipe()
					readerDone := make(chan error, 1)
					go func() {
						_, err := readMessage(left, recorder)
						readerDone <- err
					}()

					if _, err := right.Write(frame.data); err != nil {
						t.Fatalf("write frame: %v", err)
					}
					if err := right.Close(); err != nil {
						t.Fatalf("close writer: %v", err)
					}
					err := <-readerDone
					_ = left.Close()
					if (err != nil) != frame.wantErr {
						t.Fatalf("read error = %v, want error %t", err, frame.wantErr)
					}

					after := collectMetrics(t)
					wantBytes := int64(0)
					if enabled {
						wantBytes = int64(len(frame.data))
					}
					if got := deltaMetric(before, after, "network_receive_bytes", "client", ""); got != wantBytes {
						t.Fatalf("received bytes = %d, want %d", got, wantBytes)
					}
				})
			}
		})
	}
}

func TestRecordErrorIgnoresDisabledMetrics(t *testing.T) {
	recordError(nil, network.OperationRead, errors.New("read failed"))
}

func TestRecordErrorClassifiesNetworkErrors(t *testing.T) {
	recorder := network.NewMetricsRecorder("tcp-error-test", network.NetServer)

	tests := []struct {
		name      string
		err       error
		errorType network.ErrorType
	}{
		{
			name:      "invalid message",
			err:       fmt.Errorf("decode frame: %w", dueerrors.ErrInvalidMessage),
			errorType: network.ErrorTypeInvalidMessage,
		},
		{
			name:      "connection limit",
			err:       fmt.Errorf("accept connection: %w", dueerrors.ErrTooManyConnection),
			errorType: network.ErrorTypeConnectionLimit,
		},
		{
			name:      "deadline exceeded",
			err:       fmt.Errorf("read deadline: %w", context.DeadlineExceeded),
			errorType: network.ErrorTypeTimeout,
		},
		{
			name:      "net timeout",
			err:       timeoutError{},
			errorType: network.ErrorTypeTimeout,
		},
		{
			name:      "tls",
			err:       tls.RecordHeaderError{},
			errorType: network.ErrorTypeTLS,
		},
		{
			name:      "generic io",
			err:       errors.New("write failed"),
			errorType: network.ErrorTypeIO,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := collectMetrics(t)
			recordError(recorder, network.OperationRead, tt.err)
			after := collectMetrics(t)

			if got := deltaErrorMetric(before, after, "server", network.OperationRead, tt.errorType); got != 1 {
				t.Fatalf("error count = %d, want 1 for type %q", got, tt.errorType)
			}
		})
	}
}

func TestRecordErrorIgnoresNormalClose(t *testing.T) {
	recorder := network.NewMetricsRecorder("tcp-normal-close-test", network.NetServer)

	for _, err := range []error{io.EOF, net.ErrClosed} {
		before := collectMetrics(t)
		recordError(recorder, network.OperationRead, err)
		after := collectMetrics(t)

		if got := deltaMetric(before, after, "network_error", "server", ""); got != 0 {
			t.Fatalf("error count = %d, want 0 for %v", got, err)
		}
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

func deltaMetric(before, after metricdata.ResourceMetrics, name, role, direction string) int64 {
	return sumMetric(after, name, role, direction) - sumMetric(before, name, role, direction)
}

func deltaErrorMetric(before, after metricdata.ResourceMetrics, role string, operation network.Operation, errorType network.ErrorType) int64 {
	return sumErrorMetric(after, role, operation, errorType) - sumErrorMetric(before, role, operation, errorType)
}

func sumMetric(data metricdata.ResourceMetrics, name, role, direction string) int64 {
	for _, scope := range data.ScopeMetrics {
		if scope.Scope.Name != "due.network" {
			continue
		}
		for _, metric := range scope.Metrics {
			if metric.Name != name {
				continue
			}
			sum, ok := metric.Data.(metricdata.Sum[int64])
			if !ok {
				return 0
			}
			var total int64
			for _, point := range sum.DataPoints {
				if !hasAttribute(point.Attributes, "network.role", role) {
					continue
				}
				if direction != "" && !hasAttribute(point.Attributes, "network.direction", direction) {
					continue
				}
				total += point.Value
			}
			return total
		}
	}
	return 0
}

func sumErrorMetric(data metricdata.ResourceMetrics, role string, operation network.Operation, errorType network.ErrorType) int64 {
	for _, scope := range data.ScopeMetrics {
		if scope.Scope.Name != "due.network" {
			continue
		}
		for _, metric := range scope.Metrics {
			if metric.Name != "network_error" {
				continue
			}
			sum, ok := metric.Data.(metricdata.Sum[int64])
			if !ok {
				return 0
			}
			var total int64
			for _, point := range sum.DataPoints {
				if !hasAttribute(point.Attributes, "network.role", role) {
					continue
				}
				if !hasAttribute(point.Attributes, "network.operation", string(operation)) {
					continue
				}
				if !hasAttribute(point.Attributes, "error.type", string(errorType)) {
					continue
				}
				total += point.Value
			}
			return total
		}
	}
	return 0
}

func hasAttribute(set attribute.Set, key, want string) bool {
	value, ok := set.Value(attribute.Key(key))
	return ok && value.AsString() == want
}

func waitForSignal(t *testing.T, signal <-chan struct{}, name string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", name)
	}
}

type timeoutError struct{}

func (timeoutError) Error() string {
	return "timeout"
}

func (timeoutError) Timeout() bool {
	return true
}

func (timeoutError) Temporary() bool {
	return false
}
