package buffer_test

import (
	"encoding/binary"
	"io"
	"reflect"
	"testing"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
)

// TestReader_Seek verifies Seek against every whence and the negative cases.
func TestReader_Seek(t *testing.T) {
	data := []byte{1, 2, 3, 4}

	tests := []struct {
		name   string
		offset int64
		whence int
		want   int64
	}{
		{name: "seek start", offset: 2, whence: io.SeekStart, want: 2},
		{name: "seek current", offset: 3, whence: io.SeekCurrent, want: 3},
		{name: "seek end", offset: -1, whence: io.SeekEnd, want: 3},
		{name: "seek end to end", offset: 0, whence: io.SeekEnd, want: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := buffer.NewReader(data)

			got, err := r.Seek(tt.offset, tt.whence)
			if err != nil {
				t.Fatalf("Seek() unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("Seek() = %d, want %d", got, tt.want)
			}
		})
	}

	t.Run("invalid whence", func(t *testing.T) {
		r := buffer.NewReader(data)

		if _, err := r.Seek(0, 99); err == nil {
			t.Error("Seek() with invalid whence expected an error")
		}
	})

	t.Run("negative position", func(t *testing.T) {
		r := buffer.NewReader(data)

		if _, err := r.Seek(-1, io.SeekStart); err == nil {
			t.Error("Seek() with negative position expected an error")
		}
	})
}

// TestReader_Reset verifies that Reset rewinds the read offset.
func TestReader_Reset(t *testing.T) {
	r := buffer.NewReader([]byte{7, 8})

	v1, err := r.ReadUint8()
	if err != nil {
		t.Fatalf("ReadUint8() unexpected error: %v", err)
	}
	if v1 != 7 {
		t.Errorf("ReadUint8() = %d, want 7", v1)
	}

	r.Reset()

	v2, err := r.ReadUint8()
	if err != nil {
		t.Fatalf("ReadUint8() after Reset unexpected error: %v", err)
	}
	if v2 != 7 {
		t.Errorf("ReadUint8() after Reset = %d, want 7", v2)
	}
}

// TestReader_ReadPluralValues verifies the plural read methods against values produced by the writer.
func TestReader_ReadPluralValues(t *testing.T) {
	w := buffer.NewWriterWithCapacity(0)
	w.WriteBools(true, false)
	w.WriteInt8s(-1, 2)
	w.WriteUint8s(1, 2)
	w.WriteInt16s(binary.BigEndian, -3, 4)
	w.WriteUint16s(binary.BigEndian, 3, 4)
	w.WriteInt32s(binary.BigEndian, -5, 6)
	w.WriteUint32s(binary.BigEndian, 5, 6)
	w.WriteInt64s(binary.BigEndian, -7, 8)
	w.WriteUint64s(binary.BigEndian, 7, 8)
	w.WriteFloat32s(binary.BigEndian, 1.5, 2.5)
	w.WriteFloat64s(binary.BigEndian, 3.5, 4.5)
	w.WriteRunes(binary.BigEndian, 'a', 'b')
	w.WriteBytes(0xAA, 0xBB)
	w.WriteString("hi")

	r := buffer.NewReader(w.Bytes())

	if got, err := r.ReadBools(2); err != nil {
		t.Errorf("ReadBools() unexpected error: %v", err)
	} else if !reflect.DeepEqual(got, []bool{true, false}) {
		t.Errorf("ReadBools() = %v, want %v", got, []bool{true, false})
	}

	if got, err := r.ReadInt8s(2); err != nil {
		t.Errorf("ReadInt8s() unexpected error: %v", err)
	} else if !reflect.DeepEqual(got, []int8{-1, 2}) {
		t.Errorf("ReadInt8s() = %v, want %v", got, []int8{-1, 2})
	}

	if got, err := r.ReadUint8s(2); err != nil {
		t.Errorf("ReadUint8s() unexpected error: %v", err)
	} else if !reflect.DeepEqual(got, []uint8{1, 2}) {
		t.Errorf("ReadUint8s() = %v, want %v", got, []uint8{1, 2})
	}

	if got, err := r.ReadInt16s(binary.BigEndian, 2); err != nil {
		t.Errorf("ReadInt16s() unexpected error: %v", err)
	} else if !reflect.DeepEqual(got, []int16{-3, 4}) {
		t.Errorf("ReadInt16s() = %v, want %v", got, []int16{-3, 4})
	}

	if got, err := r.ReadUint16s(binary.BigEndian, 2); err != nil {
		t.Errorf("ReadUint16s() unexpected error: %v", err)
	} else if !reflect.DeepEqual(got, []uint16{3, 4}) {
		t.Errorf("ReadUint16s() = %v, want %v", got, []uint16{3, 4})
	}

	if got, err := r.ReadInt32s(binary.BigEndian, 2); err != nil {
		t.Errorf("ReadInt32s() unexpected error: %v", err)
	} else if !reflect.DeepEqual(got, []int32{-5, 6}) {
		t.Errorf("ReadInt32s() = %v, want %v", got, []int32{-5, 6})
	}

	if got, err := r.ReadUint32s(binary.BigEndian, 2); err != nil {
		t.Errorf("ReadUint32s() unexpected error: %v", err)
	} else if !reflect.DeepEqual(got, []uint32{5, 6}) {
		t.Errorf("ReadUint32s() = %v, want %v", got, []uint32{5, 6})
	}

	if got, err := r.ReadInt64s(binary.BigEndian, 2); err != nil {
		t.Errorf("ReadInt64s() unexpected error: %v", err)
	} else if !reflect.DeepEqual(got, []int64{-7, 8}) {
		t.Errorf("ReadInt64s() = %v, want %v", got, []int64{-7, 8})
	}

	if got, err := r.ReadUint64s(binary.BigEndian, 2); err != nil {
		t.Errorf("ReadUint64s() unexpected error: %v", err)
	} else if !reflect.DeepEqual(got, []uint64{7, 8}) {
		t.Errorf("ReadUint64s() = %v, want %v", got, []uint64{7, 8})
	}

	if got, err := r.ReadFloat32s(binary.BigEndian, 2); err != nil {
		t.Errorf("ReadFloat32s() unexpected error: %v", err)
	} else if !reflect.DeepEqual(got, []float32{1.5, 2.5}) {
		t.Errorf("ReadFloat32s() = %v, want %v", got, []float32{1.5, 2.5})
	}

	if got, err := r.ReadFloat64s(binary.BigEndian, 2); err != nil {
		t.Errorf("ReadFloat64s() unexpected error: %v", err)
	} else if !reflect.DeepEqual(got, []float64{3.5, 4.5}) {
		t.Errorf("ReadFloat64s() = %v, want %v", got, []float64{3.5, 4.5})
	}

	if got, err := r.ReadRunes(binary.BigEndian, 2); err != nil {
		t.Errorf("ReadRunes() unexpected error: %v", err)
	} else if !reflect.DeepEqual(got, []rune{'a', 'b'}) {
		t.Errorf("ReadRunes() = %v, want %v", got, []rune{'a', 'b'})
	}

	if got, err := r.ReadBytes(2); err != nil {
		t.Errorf("ReadBytes() unexpected error: %v", err)
	} else if !reflect.DeepEqual(got, []byte{0xAA, 0xBB}) {
		t.Errorf("ReadBytes() = %v, want %v", got, []byte{0xAA, 0xBB})
	}

	if got, err := r.ReadString(2); err != nil {
		t.Errorf("ReadString() unexpected error: %v", err)
	} else if got != "hi" {
		t.Errorf("ReadString() = %q, want %q", got, "hi")
	}
}

// TestReader_ReadZeroOrNegativeLength verifies the empty-length read behaviour.
func TestReader_ReadZeroOrNegativeLength(t *testing.T) {
	r := buffer.NewReader([]byte{1, 2, 3})

	if got, err := r.ReadBools(0); err != nil || got != nil {
		t.Errorf("ReadBools(0) = %v, %v, want nil, nil", got, err)
	}
	if got, err := r.ReadInt8s(-1); err != nil || got != nil {
		t.Errorf("ReadInt8s(-1) = %v, %v, want nil, nil", got, err)
	}
	if got, err := r.ReadUint16s(binary.BigEndian, 0); err != nil || got != nil {
		t.Errorf("ReadUint16s(0) = %v, %v, want nil, nil", got, err)
	}
	if got, err := r.ReadBytes(0); err != nil || got != nil {
		t.Errorf("ReadBytes(0) = %v, %v, want nil, nil", got, err)
	}
	if got, err := r.ReadString(0); err != nil || got != "" {
		t.Errorf("ReadString(0) = %q, %v, want \"\", nil", got, err)
	}
}

// TestReader_ReadUnexpectedEOF verifies every read reports ErrUnexpectedEOF on an empty buffer.
func TestReader_ReadUnexpectedEOF(t *testing.T) {
	reads := []struct {
		name string
		fn   func(r *buffer.Reader) error
	}{
		{name: "ReadBool", fn: func(r *buffer.Reader) error { _, err := r.ReadBool(); return err }},
		{name: "ReadBools", fn: func(r *buffer.Reader) error { _, err := r.ReadBools(1); return err }},
		{name: "ReadInt8", fn: func(r *buffer.Reader) error { _, err := r.ReadInt8(); return err }},
		{name: "ReadInt8s", fn: func(r *buffer.Reader) error { _, err := r.ReadInt8s(1); return err }},
		{name: "ReadUint8", fn: func(r *buffer.Reader) error { _, err := r.ReadUint8(); return err }},
		{name: "ReadUint8s", fn: func(r *buffer.Reader) error { _, err := r.ReadUint8s(1); return err }},
		{name: "ReadInt16", fn: func(r *buffer.Reader) error { _, err := r.ReadInt16(binary.BigEndian); return err }},
		{name: "ReadInt16s", fn: func(r *buffer.Reader) error { _, err := r.ReadInt16s(binary.BigEndian, 1); return err }},
		{name: "ReadUint16", fn: func(r *buffer.Reader) error { _, err := r.ReadUint16(binary.BigEndian); return err }},
		{name: "ReadUint16s", fn: func(r *buffer.Reader) error { _, err := r.ReadUint16s(binary.BigEndian, 1); return err }},
		{name: "ReadInt32", fn: func(r *buffer.Reader) error { _, err := r.ReadInt32(binary.BigEndian); return err }},
		{name: "ReadInt32s", fn: func(r *buffer.Reader) error { _, err := r.ReadInt32s(binary.BigEndian, 1); return err }},
		{name: "ReadUint32", fn: func(r *buffer.Reader) error { _, err := r.ReadUint32(binary.BigEndian); return err }},
		{name: "ReadUint32s", fn: func(r *buffer.Reader) error { _, err := r.ReadUint32s(binary.BigEndian, 1); return err }},
		{name: "ReadInt64", fn: func(r *buffer.Reader) error { _, err := r.ReadInt64(binary.BigEndian); return err }},
		{name: "ReadInt64s", fn: func(r *buffer.Reader) error { _, err := r.ReadInt64s(binary.BigEndian, 1); return err }},
		{name: "ReadUint64", fn: func(r *buffer.Reader) error { _, err := r.ReadUint64(binary.BigEndian); return err }},
		{name: "ReadUint64s", fn: func(r *buffer.Reader) error { _, err := r.ReadUint64s(binary.BigEndian, 1); return err }},
		{name: "ReadFloat32", fn: func(r *buffer.Reader) error { _, err := r.ReadFloat32(binary.BigEndian); return err }},
		{name: "ReadFloat32s", fn: func(r *buffer.Reader) error { _, err := r.ReadFloat32s(binary.BigEndian, 1); return err }},
		{name: "ReadFloat64", fn: func(r *buffer.Reader) error { _, err := r.ReadFloat64(binary.BigEndian); return err }},
		{name: "ReadFloat64s", fn: func(r *buffer.Reader) error { _, err := r.ReadFloat64s(binary.BigEndian, 1); return err }},
		{name: "ReadRune", fn: func(r *buffer.Reader) error { _, err := r.ReadRune(binary.BigEndian); return err }},
		{name: "ReadRunes", fn: func(r *buffer.Reader) error { _, err := r.ReadRunes(binary.BigEndian, 1); return err }},
		{name: "ReadByte", fn: func(r *buffer.Reader) error { _, err := r.ReadByte(); return err }},
		{name: "ReadBytes", fn: func(r *buffer.Reader) error { _, err := r.ReadBytes(1); return err }},
		{name: "ReadString", fn: func(r *buffer.Reader) error { _, err := r.ReadString(1); return err }},
	}

	for _, tt := range reads {
		t.Run(tt.name, func(t *testing.T) {
			r := buffer.NewReader(nil)

			if err := tt.fn(r); err != errors.ErrUnexpectedEOF {
				t.Errorf("%s() error = %v, want %v", tt.name, err, errors.ErrUnexpectedEOF)
			}
		})
	}
}

// TestReader_ReadShortBuffer verifies that a partially filled buffer still reports ErrUnexpectedEOF.
func TestReader_ReadShortBuffer(t *testing.T) {
	r := buffer.NewReader([]byte{1})

	if _, err := r.ReadString(2); err != errors.ErrUnexpectedEOF {
		t.Errorf("ReadString(2) error = %v, want %v", err, errors.ErrUnexpectedEOF)
	}
}
