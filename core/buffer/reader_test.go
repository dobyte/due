package buffer_test

import (
	"bytes"
	"encoding/binary"
	"github.com/dobyte/due/v2/core/buffer"
	"io"
	"testing"
)

func TestReader(t *testing.T) {
	writer := buffer.NewWriter(0)
	writer.WriteBools(true)
	writer.WriteInt8s(1)
	writer.WriteUint8s(2)
	writer.WriteInt16s(binary.BigEndian, 3)
	writer.WriteUint16s(binary.BigEndian, 4)
	writer.WriteInt32s(binary.BigEndian, 5)
	writer.WriteUint32s(binary.BigEndian, 6)
	writer.WriteInt64s(binary.BigEndian, 7)
	writer.WriteUint64s(binary.BigEndian, 8)
	writer.WriteFloat32s(binary.BigEndian, 9.20)
	writer.WriteFloat64s(binary.BigEndian, 10.20)
	writer.WriteRunes(binary.BigEndian, 11)
	writer.WriteBytes(12)
	writer.WriteString("hello world")

	reader := buffer.NewReader(writer.Bytes())
	v1, _ := reader.ReadBool()
	v2, _ := reader.ReadInt8()
	v3, _ := reader.ReadUint8()
	v4, _ := reader.ReadInt16(binary.BigEndian)
	v5, _ := reader.ReadUint16(binary.BigEndian)
	v6, _ := reader.ReadInt32(binary.BigEndian)
	v7, _ := reader.ReadUint32(binary.BigEndian)
	v8, _ := reader.ReadInt64(binary.BigEndian)
	v9, _ := reader.ReadUint64(binary.BigEndian)
	v10, _ := reader.ReadFloat32(binary.BigEndian)
	v11, _ := reader.ReadFloat64(binary.BigEndian)
	v12, _ := reader.ReadRune(binary.BigEndian)
	v13, _ := reader.ReadByte()
	v14, _ := reader.ReadString(11)

	t.Log(v1, v2, v3, v4, v5, v6, v7, v8, v9, v10, v11, v12, v13, v14)
}

func BenchmarkReader(b *testing.B) {
	writer := buffer.NewWriter(0)
	writer.WriteBools(true)
	writer.WriteInt8s(1)
	writer.WriteUint8s(2)
	writer.WriteInt16s(binary.BigEndian, 3)
	writer.WriteUint16s(binary.BigEndian, 4)
	writer.WriteInt32s(binary.BigEndian, 5)
	writer.WriteUint32s(binary.BigEndian, 6)
	writer.WriteInt64s(binary.BigEndian, 7)
	writer.WriteUint64s(binary.BigEndian, 8)
	writer.WriteFloat32s(binary.BigEndian, 9.20)
	writer.WriteFloat64s(binary.BigEndian, 10.20)
	writer.WriteRunes(binary.BigEndian, 11)
	writer.WriteBytes(12)
	writer.WriteString("hello world")

	reader := buffer.NewReader(writer.Bytes())

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		reader.ReadBool()
		reader.ReadInt8()
		reader.ReadUint8()
		reader.ReadInt16(binary.BigEndian)
		reader.ReadUint16(binary.BigEndian)
		reader.ReadInt32(binary.BigEndian)
		reader.ReadUint32(binary.BigEndian)
		reader.ReadInt64(binary.BigEndian)
		reader.ReadUint64(binary.BigEndian)
		reader.ReadFloat32(binary.BigEndian)
		reader.ReadFloat64(binary.BigEndian)
		reader.ReadRune(binary.BigEndian)
		reader.ReadByte()
		reader.ReadString(11)
		reader.Reset()
	}
}

func BenchmarkBinaryRead(b *testing.B) {
	writer := buffer.NewWriter(0)
	writer.WriteBools(true)
	writer.WriteInt8s(1)
	writer.WriteUint8s(2)
	writer.WriteInt16s(binary.BigEndian, 3)
	writer.WriteUint16s(binary.BigEndian, 4)
	writer.WriteInt32s(binary.BigEndian, 5)
	writer.WriteUint32s(binary.BigEndian, 6)
	writer.WriteInt64s(binary.BigEndian, 7)
	writer.WriteUint64s(binary.BigEndian, 8)
	writer.WriteFloat32s(binary.BigEndian, 9.20)
	writer.WriteFloat64s(binary.BigEndian, 10.20)
	writer.WriteRunes(binary.BigEndian, 11)
	writer.WriteBytes(12)
	writer.WriteString("hello world")

	reader := bytes.NewReader(writer.Bytes())

	b.ResetTimer()

	var (
		v1  bool
		v2  int8
		v3  uint8
		v4  int16
		v5  uint16
		v6  int32
		v7  uint32
		v8  int64
		v9  uint64
		v10 float32
		v11 float64
		v12 rune
		v13 byte
		v14 = make([]byte, 11)
	)

	for i := 0; i < b.N; i++ {
		binary.Read(reader, binary.BigEndian, &v1)
		binary.Read(reader, binary.BigEndian, &v2)
		binary.Read(reader, binary.BigEndian, &v3)
		binary.Read(reader, binary.BigEndian, &v4)
		binary.Read(reader, binary.BigEndian, &v5)
		binary.Read(reader, binary.BigEndian, &v6)
		binary.Read(reader, binary.BigEndian, &v7)
		binary.Read(reader, binary.BigEndian, &v8)
		binary.Read(reader, binary.BigEndian, &v9)
		binary.Read(reader, binary.BigEndian, &v10)
		binary.Read(reader, binary.BigEndian, &v11)
		binary.Read(reader, binary.BigEndian, &v12)
		binary.Read(reader, binary.BigEndian, &v13)
		binary.Read(reader, binary.BigEndian, &v14)
		reader.Seek(0, io.SeekStart)
	}
}
