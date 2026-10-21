package ws

import (
	"io"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/gorilla/websocket"
)

// writeBuffer writes buf as one binary WebSocket message. Single-node buffers use Gorilla's
// direct fast path; segmented buffers are streamed without flattening them into another slice.
func writeBuffer(conn *websocket.Conn, buf buffer.Buffer) error {
	if buf.Nodes() <= 1 {
		return conn.WriteMessage(websocket.BinaryMessage, buf.Bytes())
	}

	w, err := conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return err
	}

	var writeErr error
	visited := buf.VisitBytes(func(data []byte) bool {
		var n int
		n, writeErr = w.Write(data)
		if writeErr == nil && n != len(data) {
			writeErr = io.ErrShortWrite
		}
		return writeErr == nil
	})

	closeErr := w.Close()
	if writeErr != nil {
		return writeErr
	}
	if !visited {
		return errors.ErrInvalidMessage
	}
	return closeErr
}
