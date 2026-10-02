package console

import (
	"io"
	"os"
	"strings"

	"github.com/dobyte/due/v2/log/internal"
)

// Name is the syncer name.
const Name = "console"

// Syncer is a console log syncer.
type Syncer struct {
	opts      *options           // Options
	writer    io.WriteCloser     // Output writer
	formatter internal.Formatter // Log formatter
}

// NewSyncer returns a new console log syncer. The optional opts configure the syncer.
func NewSyncer(opts ...Option) *Syncer {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Syncer{}
	s.opts = o
	s.init()

	return s
}

// init initializes the syncer.
func (s *Syncer) init() {
	s.writer = os.Stdout

	if s.opts.format == FormatJson {
		s.formatter = internal.NewJsonFormatter()
	} else {
		s.formatter = internal.NewTextFormatter(s.checkSupportColor())
	}
}

// checkSupportColor reports whether the output stream supports colored output.
func (s *Syncer) checkSupportColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	term := os.Getenv("TERM")
	if term == "dumb" {
		return false
	}

	if f, ok := s.writer.(*os.File); ok {
		if fi, err := f.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
			return true
		}
	}

	return strings.Contains(term, "color") || strings.HasPrefix(term, "xterm")
}

// Name returns the syncer name.
func (s *Syncer) Name() string {
	return Name
}

// Write writes the given entity to the console. It returns any error encountered while writing.
func (s *Syncer) Write(entity *internal.Entity) error {
	buf := s.formatter.Format(entity)
	defer buf.Release()

	data := buf.Bytes()
	for len(data) > 0 {
		n, err := s.writer.Write(data)
		if err != nil {
			return err
		}

		if n == 0 {
			return io.ErrShortWrite
		}

		data = data[n:]
	}

	return nil
}

// Close closes the syncer.
func (s *Syncer) Close() error {
	return nil
}
