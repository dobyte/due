package buffer_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/dobyte/due/v2/core/buffer"
	"testing"
)

type User struct {
	ID  int32
	Age int8
}

func TestNewBuffer(t *testing.T) {
	buff := &bytes.Buffer{}
	buff.Grow(2)

	binary.Write(buff, binary.BigEndian, int16(2))

	fmt.Println(buff.Bytes())

	writer := buffer.NewWriter(2)
	writer.WriteInt16s(binary.BigEndian, int16(2))

	fmt.Println(writer.Bytes())

	writer.Reset()
	writer.WriteInt16s(binary.BigEndian, int16(20))

	fmt.Println(writer.Bytes())
}

func BenchmarkBuffer1(b *testing.B) {
	buffer := &bytes.Buffer{}
	buffer.Grow(8)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		binary.Write(buffer, binary.BigEndian, int64(2))
		buffer.Reset()
	}
}

func BenchmarkBuffer2(b *testing.B) {
	writer := buffer.NewWriter(8)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		writer.WriteInt64s(binary.BigEndian, 2)
		writer.Reset()
	}
}

func TestNewBuffer2(t *testing.T) {
	buff := buffer.NewBuffer()

	writer1 := buff.Malloc(8)
	writer1.WriteInt64s(binary.BigEndian, 2)

	writer2 := buff.Malloc(8)
	writer2.WriteInt64s(binary.BigEndian, 3)

	t.Log(buff.Len())
	t.Log(buff.Len())

	buff.Range(func(node *buffer.NocopyNode) bool {
		t.Log(node.Bytes())
		node.Release()
		return true
	})
}
