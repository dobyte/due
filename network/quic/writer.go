package quic

import (
	"io"

	"github.com/dobyte/due/v2/core/buffer"
)

// bufferWriter is a buffered writer.
//
// It reuses the visit callback so that a callback is not allocated for every packet. It is used
// only by the write goroutine of a connection and needs no locking.
type bufferWriter struct {
	writer io.Writer
	err    error
	visit  func([]byte) bool
}

// newBufferWriter returns a new buffered writer.
//
// A QUIC stream only exposes the Write([]byte) method, so the writer flushes to the underlying
// writer without flattening composite buffers.
func newBufferWriter(writer io.Writer) *bufferWriter {
	w := &bufferWriter{writer: writer}
	w.visit = w.writePart
	return w
}

// write flushes the buffer.
//
// It writes every fragment of buf to the underlying writer.
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

// writeBuffer writes a buffer.
//
// It writes the data without flattening composite buffers and handles short writes transparently.
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
