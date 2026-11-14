package config

import (
	"context"

	"github.com/dobyte/due/v2/core/config"
)

// Option configures a configurator.
type Option = config.Option

// Encoder encodes content in the given format into bytes.
type Encoder = config.Encoder

// Decoder decodes content in the given format.
type Decoder = config.Decoder

// Scanner scans content in the given format into dest.
type Scanner = config.Scanner

// WithContext sets the base context of the configurator.
func WithContext(ctx context.Context) Option {
	return config.WithContext(ctx)
}

// WithSources sets the config sources.
func WithSources(sources ...Source) Option {
	return config.WithSources(sources...)
}

// WithEncoder sets the encoder used when storing config content.
func WithEncoder(encoder Encoder) Option {
	return config.WithEncoder(encoder)
}

// WithDecoder sets the decoder used when loading config content.
func WithDecoder(decoder Decoder) Option {
	return config.WithDecoder(decoder)
}

// WithScanner sets the scanner used when scanning config content into a destination.
func WithScanner(scanner Scanner) Option {
	return config.WithScanner(scanner)
}
