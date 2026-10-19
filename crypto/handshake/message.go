package handshake

import (
	"encoding/binary"

	"github.com/dobyte/due/v2/errors"
)

const (
	// Version is the handshake protocol version.
	Version uint8 = 1

	// Frame types.
	frameClientHello uint8 = 1
	frameServerHello uint8 = 2
	frameFinished    uint8 = 3

	// Fixed field sizes.
	randomSize      = 32
	publicKeySize   = 32
	macSize         = 32
	frameHeaderSize = 4

	// Size of the fixed part of the ClientHello body: random | public key | suite count.
	clientHelloFixedSize = randomSize + publicKeySize + 1
	// Size of the part of the ServerHello body covered by the signature: suite | random | key.
	serverHelloPrefixSize = 1 + randomSize + publicKeySize
)

// clientHello is the parsed ClientHello body.
type clientHello struct {
	random    []byte
	publicKey []byte
	suites    []uint8
	body      []byte
}

// serverHello is the parsed ServerHello body.
type serverHello struct {
	suiteID   uint8
	random    []byte
	publicKey []byte
	signature []byte
	mac       []byte
	prefix    []byte
}

// encodeFrame wraps a frame body with the version, the type and the body length.
func encodeFrame(typ uint8, body []byte) []byte {
	frame := make([]byte, frameHeaderSize+len(body))
	frame[0] = Version
	frame[1] = typ
	binary.BigEndian.PutUint16(frame[2:frameHeaderSize], uint16(len(body)))
	copy(frame[frameHeaderSize:], body)

	return frame
}

// decodeFrame validates a frame and returns its type and body.
func decodeFrame(frame []byte) (uint8, []byte, error) {
	if len(frame) < frameHeaderSize {
		return 0, nil, errors.ErrInvalidMessage
	}

	if frame[0] != Version {
		return 0, nil, errors.ErrInvalidMessage
	}

	if int(binary.BigEndian.Uint16(frame[2:frameHeaderSize])) != len(frame)-frameHeaderSize {
		return 0, nil, errors.ErrInvalidMessage
	}

	return frame[1], frame[frameHeaderSize:], nil
}

// encodeClientHelloBody encodes the ClientHello body.
func encodeClientHelloBody(random, publicKey []byte, suites []uint8) []byte {
	body := make([]byte, 0, clientHelloFixedSize+len(suites))
	body = append(body, random...)
	body = append(body, publicKey...)
	body = append(body, uint8(len(suites)))
	body = append(body, suites...)

	return body
}

// decodeClientHello parses the ClientHello body.
func decodeClientHello(body []byte) (*clientHello, error) {
	if len(body) < clientHelloFixedSize {
		return nil, errors.ErrInvalidMessage
	}

	count := int(body[clientHelloFixedSize-1])
	if count == 0 || len(body) != clientHelloFixedSize+count {
		return nil, errors.ErrInvalidMessage
	}

	return &clientHello{
		random:    body[:randomSize],
		publicKey: body[randomSize : randomSize+publicKeySize],
		suites:    body[clientHelloFixedSize:],
		body:      body,
	}, nil
}

// encodeServerHelloBody encodes the ServerHello body.
func encodeServerHelloBody(suiteID uint8, random, publicKey, signature, mac []byte) ([]byte, error) {
	if len(random) != randomSize || len(publicKey) != publicKeySize || len(mac) != macSize {
		return nil, errors.ErrInvalidMessage
	}

	body := make([]byte, 0, serverHelloPrefixSize+2+len(signature)+macSize)
	body = append(body, suiteID)
	body = append(body, random...)
	body = append(body, publicKey...)
	body = binary.BigEndian.AppendUint16(body, uint16(len(signature)))
	body = append(body, signature...)
	body = append(body, mac...)

	return body, nil
}

// decodeServerHello parses the ServerHello body.
func decodeServerHello(body []byte) (*serverHello, error) {
	if len(body) < serverHelloPrefixSize+2+macSize {
		return nil, errors.ErrInvalidMessage
	}

	signatureSize := int(binary.BigEndian.Uint16(body[serverHelloPrefixSize : serverHelloPrefixSize+2]))
	if len(body) != serverHelloPrefixSize+2+signatureSize+macSize {
		return nil, errors.ErrInvalidMessage
	}

	signatureStart := serverHelloPrefixSize + 2

	return &serverHello{
		suiteID:   body[0],
		random:    body[1 : 1+randomSize],
		publicKey: body[1+randomSize : serverHelloPrefixSize],
		signature: body[signatureStart : signatureStart+signatureSize],
		mac:       body[signatureStart+signatureSize:],
		prefix:    body[:serverHelloPrefixSize],
	}, nil
}
