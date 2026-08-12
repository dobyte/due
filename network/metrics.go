package network

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "due.network"

const (
	metricMessage          = "network_message"
	metricError            = "network_error"
	metricReceiveBytes     = "network_receive_bytes"
	metricSendBytes        = "network_send_bytes"
	metricConnectionOpen   = "network_connection_open"
	metricConnectionClose  = "network_connection_close"
	metricConnectionActive = "network_connection_active"
)

type Role string

const (
	NetServer Role = "server"
	NetClient Role = "client"
)

type Operation string

const (
	OperationAccept    Operation = "accept"
	OperationAuthorize Operation = "authorize"
	OperationClose     Operation = "close"
	OperationDecode    Operation = "decode"
	OperationDial      Operation = "dial"
	OperationHeartbeat Operation = "heartbeat"
	OperationListen    Operation = "listen"
	OperationQueue     Operation = "queue"
	OperationRead      Operation = "read"
	OperationResolve   Operation = "resolve"
	OperationWrite     Operation = "write"
)

type ErrorType string

const (
	ErrorTypeConnectionLimit ErrorType = "connection_limit"
	ErrorTypeIO              ErrorType = "io"
	ErrorTypeInvalidMessage  ErrorType = "invalid_message"
	ErrorTypeTimeout         ErrorType = "timeout"
	ErrorTypeTLS             ErrorType = "tls"
)

var (
	meter = otel.Meter(meterName)

	messages         metric.Int64ObservableCounter
	errorsTotal      metric.Int64Counter
	receiveBytes     metric.Int64ObservableCounter
	sendBytes        metric.Int64ObservableCounter
	connectionsOpen  metric.Int64Counter
	connectionsClose metric.Int64Counter
	connectionsAlive metric.Int64UpDownCounter

	metricsContext = context.Background()
	recorders      sync.Map
)

func init() {
	var err error

	messages, err = meter.Int64ObservableCounter(
		metricMessage,
		metric.WithDescription("Counts successfully transferred application messages."),
		metric.WithUnit("{message}"),
	)
	if err != nil {
		panic(fmt.Errorf("create network message counter: %w", err))
	}

	errorsTotal, err = meter.Int64Counter(
		metricError,
		metric.WithDescription("Counts network transport errors."),
		metric.WithUnit("{error}"),
	)
	if err != nil {
		panic(fmt.Errorf("create network error counter: %w", err))
	}

	receiveBytes, err = meter.Int64ObservableCounter(
		metricReceiveBytes,
		metric.WithDescription("Counts received Due protocol frame bytes."),
		metric.WithUnit("By"),
	)
	if err != nil {
		panic(fmt.Errorf("create network receive bytes counter: %w", err))
	}

	sendBytes, err = meter.Int64ObservableCounter(
		metricSendBytes,
		metric.WithDescription("Counts sent Due protocol frame bytes."),
		metric.WithUnit("By"),
	)
	if err != nil {
		panic(fmt.Errorf("create network send bytes counter: %w", err))
	}

	connectionsOpen, err = meter.Int64Counter(
		metricConnectionOpen,
		metric.WithDescription("Counts opened network connections."),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		panic(fmt.Errorf("create network connection open counter: %w", err))
	}

	connectionsClose, err = meter.Int64Counter(
		metricConnectionClose,
		metric.WithDescription("Counts closed network connections."),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		panic(fmt.Errorf("create network connection close counter: %w", err))
	}

	connectionsAlive, err = meter.Int64UpDownCounter(
		metricConnectionActive,
		metric.WithDescription("Tracks currently active network connections."),
		metric.WithUnit("{connection}"),
	)
	if err != nil {
		panic(fmt.Errorf("create network active connection counter: %w", err))
	}

	if _, err = meter.RegisterCallback(observeMetrics, messages, receiveBytes, sendBytes); err != nil {
		panic(fmt.Errorf("register network metrics callback: %w", err))
	}
}

type MetricsRecorder struct {
	transportAttr   attribute.KeyValue
	roleAttr        attribute.KeyValue
	base            metric.MeasurementOption
	receive         metric.MeasurementOption
	send            metric.MeasurementOption
	receiveBytes    atomic.Int64
	receiveMessages atomic.Int64
	sendBytes       atomic.Int64
	sendMessages    atomic.Int64
}

func NewMetricsRecorder(transport string, role Role) *MetricsRecorder {
	key := metricsRecorderKey{transport: transport, role: role}
	if recorder, ok := recorders.Load(key); ok {
		return recorder.(*MetricsRecorder)
	}

	transportAttr := attribute.String("network.transport", transport)
	roleAttr := attribute.String("network.role", string(role))

	recorder := &MetricsRecorder{
		transportAttr: transportAttr,
		roleAttr:      roleAttr,
		base: metric.WithAttributeSet(attribute.NewSet(
			transportAttr,
			roleAttr,
		)),
		receive: metric.WithAttributeSet(attribute.NewSet(
			transportAttr,
			roleAttr,
			attribute.String("network.direction", "receive"),
		)),
		send: metric.WithAttributeSet(attribute.NewSet(
			transportAttr,
			roleAttr,
			attribute.String("network.direction", "send"),
		)),
	}
	actual, _ := recorders.LoadOrStore(key, recorder)
	return actual.(*MetricsRecorder)
}

func (r *MetricsRecorder) Receive(size int, isMessage bool) {
	if size <= 0 {
		return
	}

	r.receiveBytes.Add(int64(size))
	if isMessage {
		r.receiveMessages.Add(1)
	}
}

func (r *MetricsRecorder) MessageReceived() {
	r.receiveMessages.Add(1)
}

func (r *MetricsRecorder) Send(size int, isMessage bool) {
	if size <= 0 {
		return
	}

	r.sendBytes.Add(int64(size))
	if isMessage {
		r.sendMessages.Add(1)
	}
}

func (r *MetricsRecorder) Error(operation Operation, errorType ErrorType) {
	errorsTotal.Add(metricsContext, 1, metric.WithAttributes(
		r.transportAttr,
		r.roleAttr,
		attribute.String("network.operation", string(operation)),
		attribute.String("error.type", string(errorType)),
	))
}

func (r *MetricsRecorder) ConnectionOpened() {
	connectionsOpen.Add(metricsContext, 1, r.base)
	connectionsAlive.Add(metricsContext, 1, r.base)
}

func (r *MetricsRecorder) ConnectionClosed() {
	connectionsClose.Add(metricsContext, 1, r.base)
	connectionsAlive.Add(metricsContext, -1, r.base)
}

type metricsRecorderKey struct {
	transport string
	role      Role
}

func observeMetrics(_ context.Context, observer metric.Observer) error {
	recorders.Range(func(_, value any) bool {
		recorder := value.(*MetricsRecorder)
		observer.ObserveInt64(receiveBytes, recorder.receiveBytes.Load(), recorder.base)
		observer.ObserveInt64(sendBytes, recorder.sendBytes.Load(), recorder.base)
		observer.ObserveInt64(messages, recorder.receiveMessages.Load(), recorder.receive)
		observer.ObserveInt64(messages, recorder.sendMessages.Load(), recorder.send)
		return true
	})
	return nil
}
