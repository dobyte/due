package quic

import (
	"io"

	"github.com/dobyte/due/v2/core/buffer"
)

// bufferWriter reuses the visit callback instead of allocating one per packet.
// Only a connection's write worker uses this object, so no locking is required.
type bufferWriter struct {
	writer io.Writer
	err    error
	visit  func([]byte) bool
}

// newBufferWriter creates a writer that flushes a composite buffer without
// flattening it, since QUIC streams only expose a Write([]byte) method.
func newBufferWriter(writer io.Writer) *bufferWriter {
	w := &bufferWriter{writer: writer}
	w.visit = w.writePart
	return w
}

// write flushes every segment of buf to the underlying writer.
func (w *bufferWriter) write(buf buffer.Buffer) error {
	if buf.Nodes() == 1 {
		return writeAll(w.writer, buf.Bytes())
	}
	w.err = nil
	buf.VisitBytes(w.visit)
	return w.err
}

func (w *bufferWriter) writePart(b []byte) bool {
	w.err = writeAll(w.writer, b)
	return w.err == nil
}

// writeBuffer writes a composite buffer without flattening it and handles short
// writes transparently.
func writeBuffer(writer io.Writer, buf buffer.Buffer) error {
	if buf.Nodes() == 1 {
		return writeAll(writer, buf.Bytes())
	}
	return newBufferWriter(writer).write(buf)
}

func writeAll(writer io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := writer.Write(b)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(b) {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}
