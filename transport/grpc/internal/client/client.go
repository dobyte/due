package client

import (
	"github.com/dobyte/due/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type Options struct {
	Addr       string
	IsSecure   bool
	CertFile   string
	ServerName string
}

func Dial(opts *Options) (*grpc.ClientConn, error) {
	options := make([]grpc.DialOption, 1)

	if opts.IsSecure {
		if opts.CertFile == "" {
			return nil, errors.New("certificate file required")
		}

		creds, err := credentials.NewClientTLSFromFile(opts.CertFile, opts.ServerName)
		if err != nil {
			return nil, err
		}
		options = append(options, grpc.WithTransportCredentials(creds))
	} else {
		options = append(options, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	return grpc.Dial(opts.Addr, options...)
}
