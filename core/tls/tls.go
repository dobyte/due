package tls

import (
	"crypto/tls"
	"crypto/x509"
	"os"

	"github.com/dobyte/due/v2/errors"
)

func MakeRedisTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	// Load client cert
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	// Load CA cert
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()

	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, errors.ErrInvalidCertFile
	}

	return &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
	}, nil
}

func MakeTCPClientTLSConfig(caFile string, serverName string) (*tls.Config, error) {
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()

	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, errors.ErrInvalidCertFile
	}

	return &tls.Config{ServerName: serverName, RootCAs: caCertPool}, nil
}
