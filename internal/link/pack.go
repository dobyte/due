package link

import (
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/encoding"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/packet"
)

// defaultMessageCapacity is the payload capacity reserved for codecs that cannot report their
// encoded size in advance. Larger payloads fall back to the segmented packer.
const defaultMessageCapacity = 1024

// packMessage packs a message, encoding directly into a pooled buffer when possible. extraHeadroom
// reserves space for a transport header prepended after the message packet header.
func packMessage(codec encoding.Codec, encryptor crypto.Encryptor, message *Message, encrypt bool, extraHeadroom int) (buffer.Buffer, error) {
	headroom := packet.MessageBufferHeadroom() + extraHeadroom

	if headroom > 0 && canPackInPlace(message.Data, encryptor, encrypt) {
		if appender, ok := codec.(encoding.Appender); ok {
			if packed, ok, err := packMessageInPlace(appender, message, headroom); ok || err != nil {
				return packed, err
			}
		}
	}

	data, err := packBuffer(codec, encryptor, message.Data, encrypt)
	if err != nil {
		return nil, err
	}

	return packet.PackMessage(message.Route, message.Seq, data)
}

// canPackInPlace reports whether the payload can be encoded into a pooled buffer with headroom.
func canPackInPlace(data any, encryptor crypto.Encryptor, encrypt bool) bool {
	if data == nil || (encrypt && encryptor != nil) {
		return false
	}

	_, isBytes := data.([]byte)

	return !isBytes
}

// packMessageInPlace encodes into a pooled buffer and prepends the header in place. It reports false
// without an error when the codec grew the slice outside the pooled buffer.
func packMessageInPlace(appender encoding.Appender, message *Message, headroom int) (buffer.Buffer, bool, error) {
	payload := buffer.MallocBytesWithHeadroom(headroom, estimatePayloadSize(message.Data))
	if payload == nil {
		return nil, false, nil
	}

	dst := payload.Bytes()[:0]
	capacity := cap(dst)

	encoded, err := appender.Append(dst, message.Data)
	if err != nil {
		payload.Release()
		return nil, true, err
	}

	if len(encoded) > capacity || !sameBuffer(dst, encoded) {
		payload.Release()

		packed, err := packet.PackMessage(message.Route, message.Seq, encoded)

		return packed, true, err
	}

	if !payload.Resize(len(encoded)) {
		payload.Release()
		return nil, true, errors.ErrMessageTooLarge
	}

	packed, err := packet.PackMessageBuffer(message.Route, message.Seq, payload)
	if err != nil {
		payload.Release()
		return nil, true, err
	}

	return packed, true, nil
}

// estimatePayloadSize returns the capacity hint used to select the pooled buffer grade.
func estimatePayloadSize(data any) int {
	if s, ok := data.(string); ok {
		return max(len(s)+2, defaultMessageCapacity)
	}

	return defaultMessageCapacity
}

// sameBuffer reports whether encoded still uses the backing array of dst.
func sameBuffer(dst, encoded []byte) bool {
	if len(encoded) == 0 {
		return true
	}

	if cap(dst) == 0 {
		return false
	}

	return &encoded[0] == &dst[:1][0]
}

// packBuffer encodes and optionally encrypts a payload without packing the header.
func packBuffer(codec encoding.Codec, encryptor crypto.Encryptor, message any, encrypt bool) ([]byte, error) {
	if message == nil {
		return nil, nil
	}

	if v, ok := message.([]byte); ok {
		return v, nil
	}

	data, err := codec.Marshal(message)
	if err != nil {
		return nil, err
	}

	if encrypt && encryptor != nil {
		return encryptor.Encrypt(data)
	}

	return data, nil
}
