package console

import "github.com/dobyte/due/v2/log/internal"

// Format is the log output format.
type Format = internal.Format

const (
	FormatText = internal.FormatText // Text format
	FormatJson = internal.FormatJson // JSON format
)
