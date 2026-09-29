package quic

import (
	"io"

	"github.com/dobyte/due/v2/core/buffer"
)

// bufferWriter 缓冲写入器
// 复用visit回调，避免每个包都分配一次回调；仅供连接的写协程使用，无需加锁
type bufferWriter struct {
	writer io.Writer
	err    error
	visit  func([]byte) bool
}

// newBufferWriter 创建缓冲写入器
// QUIC流仅暴露 Write([]byte) 方法，写入器在不打平复合缓冲的前提下将其冲刷到底层写入器
// @param writer io.Writer 底层写入器
// @return @1 *bufferWriter 缓冲写入器
func newBufferWriter(writer io.Writer) *bufferWriter {
	w := &bufferWriter{writer: writer}
	w.visit = w.writePart
	return w
}

// write 冲刷缓冲
// 将 buf 的所有分片写入底层写入器
// @param buf buffer.Buffer 消息缓冲
// @return @1 error 错误信息
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

// writeBuffer 写入缓冲
// 在不打平复合缓冲的前提下写入数据，并透明处理短写
// @param writer io.Writer 底层写入器
// @param buf buffer.Buffer 消息缓冲
// @return @1 error 错误信息
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
