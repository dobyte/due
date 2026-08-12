package tcp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"

	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

type countingConn struct {
	net.Conn
	bytes int
}

func (c *countingConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	c.bytes += n
	return n, err
}

func readMessage(conn net.Conn, metrics *network.MetricsRecorder) ([]byte, error) {
	reader := &countingConn{Conn: conn}
	data, err := packet.ReadMessage(reader)
	metrics.Receive(reader.bytes, false)
	return data, err
}

func recordError(metrics *network.MetricsRecorder, operation network.Operation, err error) {
	if err == nil || isNormalClose(err) {
		return
	}

	errorType := network.ErrorTypeIO
	switch {
	case errors.Is(err, errors.ErrInvalidMessage):
		errorType = network.ErrorTypeInvalidMessage
	case errors.Is(err, errors.ErrTooManyConnection):
		errorType = network.ErrorTypeConnectionLimit
	case errors.Is(err, context.DeadlineExceeded):
		errorType = network.ErrorTypeTimeout
	case isTLSError(err):
		errorType = network.ErrorTypeTLS
	default:
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			errorType = network.ErrorTypeTimeout
		}
	}

	metrics.Error(operation, errorType)
}

func isTLSError(err error) bool {
	var (
		alertErr       tls.AlertError
		recordErr      tls.RecordHeaderError
		certificateErr x509.CertificateInvalidError
		hostnameErr    x509.HostnameError
		authorityErr   x509.UnknownAuthorityError
	)

	return errors.As(err, &alertErr) ||
		errors.As(err, &recordErr) ||
		errors.As(err, &certificateErr) ||
		errors.As(err, &hostnameErr) ||
		errors.As(err, &authorityErr)
}

func isNormalClose(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed)
}
