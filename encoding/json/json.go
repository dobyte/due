package json

const Name = "json"

var DefaultCodec = &codec{}

type codec struct{}

// Name returns the codec name.
func (codec) Name() string {
	return Name
}

// Marshal encodes v.
func (codec) Marshal(v any) ([]byte, error) {
	return marshal(v)
}

// Append appends the JSON encoding of v to dst.
func (codec) Append(dst []byte, v any) ([]byte, error) {
	return appendEncode(dst, v)
}

// Unmarshal decodes data into v.
func (codec) Unmarshal(data []byte, v any) error {
	return unmarshal(data, v)
}

// UnmarshalString decodes a JSON string into v.
func (codec) UnmarshalString(s string, v any) error {
	return unmarshalString(s, v)
}

// Marshal encodes v using the default codec.
func Marshal(v any) ([]byte, error) {
	return DefaultCodec.Marshal(v)
}

// Unmarshal decodes data into v using the default codec.
func Unmarshal(data []byte, v any) error {
	return DefaultCodec.Unmarshal(data, v)
}

// UnmarshalString decodes a JSON string into v using the default codec.
func UnmarshalString(s string, v any) error {
	return DefaultCodec.UnmarshalString(s, v)
}
