package handshake

import (
	"crypto/cipher"
	"encoding/binary"
	"math"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/core/security/aead"
	"github.com/dobyte/due/v2/errors"
)

// Direction identifies the direction of a message.
type Direction uint8

const (
	// ClientToServer is the direction from the client to the server.
	ClientToServer Direction = 0
	// ServerToClient is the direction from the server to the client.
	ServerToClient Direction = 1
)

const (
	// envelopeHeaderSize is the size of the epoch and counter carried in front of the ciphertext.
	envelopeHeaderSize = 9
	// aadSize is the size of the additional authenticated data built by MakeAAD.
	aadSize = 11
	// nonceSize is the nonce size required by the supported AEAD suites.
	nonceSize = 12
	// saltSize is the size of the nonce salt derived by the handshake.
	saltSize = 4
)

type role uint8

const (
	clientRole role = iota
	serverRole
)

// MakeAAD builds the additional authenticated data bound to the message header.
//
// The encoding is canonical and independent of the packet configuration: version (1 byte),
// direction (1 byte), epoch (1 byte), route (4 bytes, big endian) and seq (4 bytes, big endian).
// Both peers must build it from the same values, so a modified header fails authentication and the
// epoch cannot be rewritten to downgrade the session.
func MakeAAD(direction Direction, epoch uint8, route, seq int32) []byte {
	aad := make([]byte, aadSize)
	writeAAD(aad, direction, epoch, route, seq)
	return aad
}

// writeAAD encodes canonical metadata into an operation-owned scratch buffer.
func writeAAD(aad []byte, direction Direction, epoch uint8, route, seq int32) {
	aad[0] = Version
	aad[1] = byte(direction)
	aad[2] = epoch
	binary.BigEndian.PutUint32(aad[3:7], uint32(route))
	binary.BigEndian.PutUint32(aad[7:11], uint32(seq))
}

// messageScratch is borrowed for one AEAD call, including concurrent calls on the same session.
// Pooling keeps the slices passed through cipher.AEAD from allocating on every message.
type messageScratch struct {
	nonce [nonceSize]byte
	aad   [aadSize]byte
}

var _messageScratchPool = sync.Pool{New: func() any { return new(messageScratch) }}

// Session is the AEAD session established by the handshake.
//
// The sealed payload layout is epoch(1) | counter(8) | ciphertext | tag. The nonce is the 4-byte
// session salt followed by the 8-byte counter, so it is never reused as long as the counter does
// not wrap. The epoch byte allows a future key rotation to keep both keys alive.
type Session struct {
	suite   aead.Suite
	send    cipher.AEAD
	recv    cipher.AEAD
	epoch   uint8
	salt    [saltSize]byte
	sendCtr atomic.Uint64
	recvWin replayWindow
}

// newSession builds a session from the negotiated suite and the derived key material.
func newSession(suite aead.Suite, material *keyMaterial, r role) (*Session, error) {
	var sendKey, recvKey []byte

	if r == clientRole {
		sendKey, recvKey = material.c2sKey, material.s2cKey
	} else {
		sendKey, recvKey = material.s2cKey, material.c2sKey
	}

	send, err := suite.New(sendKey)
	if err != nil {
		return nil, err
	}

	recv, err := suite.New(recvKey)
	if err != nil {
		return nil, err
	}

	if send.NonceSize() != nonceSize || recv.NonceSize() != nonceSize {
		return nil, errors.New("handshake: unsupported nonce size")
	}

	return &Session{
		suite: suite,
		send:  send,
		recv:  recv,
		epoch: 1,
		salt:  material.salt,
	}, nil
}

// Suite returns the negotiated suite.
func (s *Session) Suite() aead.Suite {
	return s.suite
}

// Epoch returns the key epoch carried by the sealed payload.
func (s *Session) Epoch() uint8 {
	return s.epoch
}

// Overhead returns the number of bytes added to the plaintext by Seal.
func (s *Session) Overhead() int {
	return envelopeHeaderSize + s.send.Overhead()
}

// Seal encrypts plaintext and binds it to aad.
func (s *Session) Seal(plaintext, aad []byte) ([]byte, error) {
	scratch := _messageScratchPool.Get().(*messageScratch)
	sealed, err := s.seal(nil, plaintext, aad, scratch.nonce[:])
	_messageScratchPool.Put(scratch)
	return sealed, err
}

// SealMessage appends an encrypted message to dst, binding its direction, route and sequence
// number using MakeAAD's encoding. dst must not overlap plaintext. The caller owns the output.
func (s *Session) SealMessage(dst, plaintext []byte, direction Direction, route, seq int32) ([]byte, error) {
	scratch := _messageScratchPool.Get().(*messageScratch)
	writeAAD(scratch.aad[:], direction, s.epoch, route, seq)
	sealed, err := s.seal(dst, plaintext, scratch.aad[:], scratch.nonce[:])
	_messageScratchPool.Put(scratch)
	return sealed, err
}

// seal encrypts with operation-owned nonce storage and appends the envelope to dst.
func (s *Session) seal(dst, plaintext, aad, nonce []byte) ([]byte, error) {
	counter, err := s.nextCounter()
	if err != nil {
		return nil, err
	}

	copy(nonce[:saltSize], s.salt[:])
	binary.BigEndian.PutUint64(nonce[saltSize:], counter)

	start := len(dst)
	// Reserve the whole envelope at once so nil dst also needs only one output allocation.
	size := start + envelopeHeaderSize + len(plaintext) + s.send.Overhead()
	if cap(dst) < size {
		grown := make([]byte, start, size)
		copy(grown, dst)
		dst = grown
	}
	sealed := dst[:start+envelopeHeaderSize]
	sealed[start] = s.epoch
	binary.BigEndian.PutUint64(sealed[start+1:], counter)
	return s.send.Seal(sealed, nonce, plaintext, aad), nil
}

// Open authenticates and decrypts a sealed payload.
//
// The replay window is advanced only after the payload is authenticated, so forged packets cannot
// consume window slots.
func (s *Session) Open(sealed, aad []byte) ([]byte, error) {
	scratch := _messageScratchPool.Get().(*messageScratch)
	plaintext, err := s.open(nil, sealed, aad, scratch.nonce[:])
	_messageScratchPool.Put(scratch)
	return plaintext, err
}

// OpenMessage authenticates a message and appends its plaintext to dst, binding its metadata
// using MakeAAD's encoding. dst must not overlap sealed, except that sealed[9:9] may be used for
// in-place decryption. On error no plaintext is returned and dst's backing memory may be modified.
// Replay state advances only after authentication succeeds, as in Open.
func (s *Session) OpenMessage(dst, sealed []byte, direction Direction, route, seq int32) ([]byte, error) {
	scratch := _messageScratchPool.Get().(*messageScratch)
	writeAAD(scratch.aad[:], direction, s.epoch, route, seq)
	plaintext, err := s.open(dst, sealed, scratch.aad[:], scratch.nonce[:])
	_messageScratchPool.Put(scratch)
	return plaintext, err
}

// open decrypts with operation-owned nonce storage and preserves the replay window ordering.
func (s *Session) open(dst, sealed, aad, nonce []byte) ([]byte, error) {
	if len(sealed) < envelopeHeaderSize+s.recv.Overhead() {
		return nil, errors.ErrInvalidMessage
	}

	if sealed[0] != s.epoch {
		return nil, errors.ErrInvalidMessage
	}

	counter := binary.BigEndian.Uint64(sealed[1:envelopeHeaderSize])

	copy(nonce[:saltSize], s.salt[:])
	binary.BigEndian.PutUint64(nonce[saltSize:], counter)

	plaintext, err := s.recv.Open(dst, nonce, sealed[envelopeHeaderSize:], aad)
	if err != nil {
		return nil, errors.ErrInvalidMessage
	}

	if !s.recvWin.advance(counter) {
		return nil, ErrReplayMessage
	}

	return plaintext, nil
}

// nextCounter returns the next nonce counter and reports exhaustion before it can wrap.
func (s *Session) nextCounter() (uint64, error) {
	for {
		current := s.sendCtr.Load()
		if current == math.MaxUint64 {
			return 0, ErrKeyExhausted
		}

		if s.sendCtr.CompareAndSwap(current, current+1) {
			return current, nil
		}
	}
}

// replayWindow is a 64-bit sliding replay window.
type replayWindow struct {
	mu      sync.Mutex
	highest uint64
	bits    uint64
	inited  bool
}

// advance records counter and reports whether it is fresh. Counters older than the window are
// rejected as well.
func (w *replayWindow) advance(counter uint64) bool {
	w.mu.Lock()

	if !w.inited {
		w.inited = true
		w.highest = counter
		w.bits = 1
		w.mu.Unlock()
		return true
	}

	if counter > w.highest {
		shift := counter - w.highest
		if shift >= 64 {
			w.bits = 0
		} else {
			w.bits <<= shift
		}

		w.bits |= 1
		w.highest = counter
		w.mu.Unlock()
		return true
	}

	diff := w.highest - counter
	if diff >= 64 || w.bits&(1<<diff) != 0 {
		w.mu.Unlock()
		return false
	}

	w.bits |= 1 << diff
	w.mu.Unlock()

	return true
}
