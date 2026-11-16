package config

import "github.com/dobyte/due/v2/errors"

// Configuration is a config item loaded from a config source.
type Configuration struct {
	decoder  Decoder // Decoder used to decode the config content
	scanner  Scanner // Scanner used to scan the config content into a destination
	Path     string  // Path of the config file relative to the source root
	File     string  // File name including its extension
	Name     string  // File name without its extension
	Format   string  // File format
	Content  []byte  // File content
	FullPath string  // Full path of the config file
}

// Decode decodes the config content, reporting [errors.ErrInvalidDecoder] when no decoder is
// attached.
func (c *Configuration) Decode() (any, error) {
	if c.decoder == nil {
		return nil, errors.ErrInvalidDecoder
	}

	return c.decoder(c.Format, c.Content)
}

// Scan scans the config content into dest, reporting [errors.ErrInvalidScanner] when no scanner is
// attached.
func (c *Configuration) Scan(dest any) error {
	if c.scanner == nil {
		return errors.ErrInvalidScanner
	}

	return c.scanner(c.Format, c.Content, dest)
}
