package encoding

import (
	"testing"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/log"
)

// panickingLogger is a log.Logger whose fatal methods panic instead of terminating the process.
// It lets the tests cover the fatal branches of the registry.
type panickingLogger struct {
	log.Logger
}

// Fatal panics with a fixed message.
func (l *panickingLogger) Fatal(a ...any) { panic("fatal") }

// Fatalf panics with a fixed message.
func (l *panickingLogger) Fatalf(format string, a ...any) { panic("fatal") }

// Close reports success without doing anything.
func (l *panickingLogger) Close() error { return nil }

// mockCodec is an in-memory Codec used to exercise the registry.
type mockCodec struct {
	name string
}

// Name returns the codec name.
func (m *mockCodec) Name() string { return m.name }

// Marshal returns the codec name as the encoded payload.
func (m *mockCodec) Marshal(v any) ([]byte, error) { return []byte(m.name), nil }

// Unmarshal does nothing.
func (m *mockCodec) Unmarshal(data []byte, v any) error { return nil }

// TestInvokeRegisteredCodec verifies that the codecs registered at init time can be looked up.
func TestInvokeRegisteredCodec(t *testing.T) {
	if got := Invoke(json.DefaultCodec.Name()); got != Codec(json.DefaultCodec) {
		t.Error("expect the json codec to be registered")
	}
}

// TestRegisterAndInvokeCodec verifies the codec registration and lookup round trip.
func TestRegisterAndInvokeCodec(t *testing.T) {
	codec := &mockCodec{name: "mock-codec"}
	Register(codec)

	got := Invoke("mock-codec")
	if got != Codec(codec) {
		t.Fatal("expect the registered codec to be returned")
	}

	data, err := got.Marshal(nil)
	if err != nil {
		t.Fatalf("marshal failed, err: %v", err)
	}
	if string(data) != "mock-codec" {
		t.Errorf("invalid marshaled data, expect: mock-codec, actual: %s", data)
	}
	if err := got.Unmarshal(data, nil); err != nil {
		t.Errorf("unmarshal failed, err: %v", err)
	}
}

// TestRegisterCodecOverwrite verifies that registering the same name again overwrites the previous
// codec.
func TestRegisterCodecOverwrite(t *testing.T) {
	first := &mockCodec{name: "mock-overwrite-codec"}
	second := &mockCodec{name: "mock-overwrite-codec"}

	Register(first)
	Register(second)

	if got := Invoke("mock-overwrite-codec"); got != Codec(second) {
		t.Error("expect the newly registered codec to win")
	}
}

// TestRegisterNilCodec verifies that registering a nil codec is fatal.
func TestRegisterNilCodec(t *testing.T) {
	orig := log.GetLogger()
	log.SetLogger(&panickingLogger{})
	defer log.SetLogger(orig)

	defer func() {
		if recover() == nil {
			t.Error("expect a panic when registering a nil codec")
		}
	}()

	Register(nil)
}

// TestRegisterCodecWithoutName verifies that registering a codec without a name is fatal.
func TestRegisterCodecWithoutName(t *testing.T) {
	orig := log.GetLogger()
	log.SetLogger(&panickingLogger{})
	defer log.SetLogger(orig)

	defer func() {
		if recover() == nil {
			t.Error("expect a panic when registering a codec without a name")
		}
	}()

	Register(&mockCodec{name: ""})
}

// TestInvokeUnregisteredCodec verifies that looking up an unknown codec is fatal.
func TestInvokeUnregisteredCodec(t *testing.T) {
	orig := log.GetLogger()
	log.SetLogger(&panickingLogger{})
	defer log.SetLogger(orig)

	defer func() {
		if recover() == nil {
			t.Error("expect a panic when invoking an unregistered codec")
		}
	}()

	Invoke("mock-missing-codec")
}
