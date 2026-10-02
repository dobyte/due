package internal

// Format is the log output format.
type Format string

const (
	FormatText Format = "text" // Text format
	FormatJson Format = "json" // JSON format
)

// defaultBufferSize is the initial capacity of the log buffer.
const defaultBufferSize = 2048
