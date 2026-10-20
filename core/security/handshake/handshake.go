package handshake

import (
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"math"
	"slices"

	"github.com/dobyte/due/v2/core/security/aead"
	"github.com/dobyte/due/v2/core/security/aead/aesgcm"
	"github.com/dobyte/due/v2/core/security/aead/chacha20poly1305"
	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/errors"
)

var (
	// ErrReplayMessage is returned when a sealed payload is replayed or is older than the
	// replay window.
	ErrReplayMessage = errors.New("replayed message")
	// ErrKeyExhausted is returned when the nonce counter of a session is exhausted.
	ErrKeyExhausted = errors.New("session key exhausted")
)

const (
	// transcriptLabel domain-separates the handshake transcript.
	transcriptLabel = "due-handshake-v1"
	// keyMaterialSize is the total size of the derived key material.
	keyMaterialSize = 32*4 + saltSize
)

// DefaultSuites returns the AEAD suites offered when none is configured, ordered by local
// preference.
func DefaultSuites() []aead.Suite {
	return []aead.Suite{aesgcm.Suite, chacha20poly1305.Suite}
}

// validSuites reports whether suites can be carried by the handshake: at most 255 entries, which
// is the limit of the suite count field, and no nil suite.
func validSuites(suites []aead.Suite) bool {
	if len(suites) > math.MaxUint8 {
		return false
	}

	for _, suite := range suites {
		if suite == nil {
			return false
		}
	}

	return true
}

// keyMaterial holds the derived traffic keys.
type keyMaterial struct {
	c2sKey    []byte
	s2cKey    []byte
	c2sFinish []byte
	s2cFinish []byte
	salt      [saltSize]byte
}

// Client performs the client side of the handshake for a single connection.
//
// The client only needs the public key of the server, and it verifies the ServerHello signature
// with it. A new client must be created for every connection because the handshake state is
// connection-scoped.
type Client struct {
	err      error
	verifier crypto.Signer
	random   []byte
	priv     *ecdh.PrivateKey
	hello    []byte
	state    uint8
	suites   []aead.Suite
}

// NewClient returns a client side handshake bound to a single connection.
//
// verifier verifies the signature of the server, and it is usually built from the pinned server
// public key.
func NewClient(verifier crypto.Signer, suites ...aead.Suite) *Client {
	if len(suites) == 0 {
		suites = DefaultSuites()
	}

	c := &Client{verifier: verifier, suites: slices.Clone(suites)}

	switch {
	case verifier == nil:
		c.err = errors.New("handshake: verifier must not be nil")
	case !validSuites(suites):
		c.err = errors.New("handshake: invalid aead suites")
	}

	return c
}

// Hello returns the ClientHello frame to send to the server.
func (c *Client) Hello() ([]byte, error) {
	if c.err != nil {
		return nil, c.err
	}

	if c.state != 0 {
		return nil, errors.New("handshake: unexpected state")
	}

	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	random := make([]byte, randomSize)
	if _, err = rand.Read(random); err != nil {
		return nil, err
	}

	suites := make([]uint8, 0, len(c.suites))
	for _, suite := range c.suites {
		suites = append(suites, suite.ID())
	}

	c.priv = priv
	c.random = random
	c.hello = encodeClientHelloBody(random, priv.PublicKey().Bytes(), suites)
	c.state = 1

	return encodeFrame(frameClientHello, c.hello), nil
}

// Handle consumes the ServerHello frame and returns the established session together with the
// Finished frame to send back to the server.
//
// The server signature and the server Finished MAC are verified before the session is returned,
// so a failure means the peer is not the expected server or the handshake was tampered with.
func (c *Client) Handle(frame []byte) (*Session, []byte, error) {
	if c.err != nil {
		return nil, nil, c.err
	}

	if c.state != 1 {
		return nil, nil, errors.New("handshake: unexpected state")
	}

	typ, body, err := decodeFrame(frame)
	if err != nil {
		return nil, nil, err
	}

	if typ != frameServerHello {
		return nil, nil, errors.ErrInvalidMessage
	}

	hello, err := decodeServerHello(body)
	if err != nil {
		return nil, nil, err
	}

	suite := c.findSuite(hello.suiteID)
	if suite == nil {
		return nil, nil, errors.New("handshake: unsupported aead suite")
	}

	signed := transcriptHash([]byte(transcriptLabel), c.hello, hello.prefix)

	ok, err := c.verifier.Verify(signed, hello.signature)
	if err != nil {
		return nil, nil, err
	}

	if !ok {
		return nil, nil, errors.ErrInvalidSignature
	}

	transcript := transcriptHash(c.hello, hello.prefix, hello.signature)

	peer, err := ecdh.X25519().NewPublicKey(hello.publicKey)
	if err != nil {
		return nil, nil, errors.ErrInvalidPublicKey
	}

	shared, err := c.priv.ECDH(peer)
	if err != nil {
		return nil, nil, err
	}

	salt := make([]byte, 0, randomSize*2)
	salt = append(salt, c.random...)
	salt = append(salt, hello.random...)

	material, err := deriveKeyMaterial(shared, salt, transcript)
	if err != nil {
		return nil, nil, err
	}

	if subtle.ConstantTimeCompare(finishMAC(material.s2cFinish, transcript), hello.mac) != 1 {
		return nil, nil, errors.ErrInvalidSignature
	}

	session, err := newSession(suite, material, clientRole)
	if err != nil {
		return nil, nil, err
	}

	c.state = 2

	return session, encodeFrame(frameFinished, finishMAC(material.c2sFinish, transcript)), nil
}

// findSuite returns the offered suite with the given wire identifier.
func (c *Client) findSuite(id uint8) aead.Suite {
	for _, suite := range c.suites {
		if suite.ID() == id {
			return suite
		}
	}

	return nil
}

// Server performs the server side of the handshake. It is stateless and safe for concurrent use;
// the per-connection state lives in Pending.
type Server struct {
	err    error
	signer crypto.Signer
	suites []aead.Suite
}

// NewServer returns a server side handshake.
//
// signer signs the ServerHello so that the client can verify it with the pinned server public
// key.
func NewServer(signer crypto.Signer, suites ...aead.Suite) *Server {
	if len(suites) == 0 {
		suites = DefaultSuites()
	}

	s := &Server{signer: signer, suites: slices.Clone(suites)}

	switch {
	case signer == nil:
		s.err = errors.New("handshake: signer must not be nil")
	case !validSuites(suites):
		s.err = errors.New("handshake: invalid aead suites")
	}

	return s
}

// Pending is a derived but unconfirmed session.
type Pending struct {
	session    *Session
	transcript []byte
	c2sFinish  []byte
}

// Accept consumes a ClientHello frame and returns the pending session together with the
// ServerHello frame to send back to the client.
//
// The session must not be used to protect traffic until Confirm succeeds.
func (s *Server) Accept(frame []byte) (*Pending, []byte, error) {
	if s.err != nil {
		return nil, nil, s.err
	}

	typ, body, err := decodeFrame(frame)
	if err != nil {
		return nil, nil, err
	}

	if typ != frameClientHello {
		return nil, nil, errors.ErrInvalidMessage
	}

	hello, err := decodeClientHello(body)
	if err != nil {
		return nil, nil, err
	}

	suite := s.selectSuite(hello.suites)
	if suite == nil {
		return nil, nil, errors.New("handshake: no common aead suite")
	}

	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}

	random := make([]byte, randomSize)
	if _, err = rand.Read(random); err != nil {
		return nil, nil, err
	}

	publicKey := priv.PublicKey().Bytes()

	prefix := make([]byte, 0, serverHelloPrefixSize)
	prefix = append(prefix, suite.ID())
	prefix = append(prefix, random...)
	prefix = append(prefix, publicKey...)

	signature, err := s.signer.Sign(transcriptHash([]byte(transcriptLabel), body, prefix))
	if err != nil {
		return nil, nil, err
	}

	transcript := transcriptHash(body, prefix, signature)

	peer, err := ecdh.X25519().NewPublicKey(hello.publicKey)
	if err != nil {
		return nil, nil, errors.ErrInvalidPublicKey
	}

	shared, err := priv.ECDH(peer)
	if err != nil {
		return nil, nil, err
	}

	salt := make([]byte, 0, randomSize*2)
	salt = append(salt, hello.random...)
	salt = append(salt, random...)

	material, err := deriveKeyMaterial(shared, salt, transcript)
	if err != nil {
		return nil, nil, err
	}

	session, err := newSession(suite, material, serverRole)
	if err != nil {
		return nil, nil, err
	}

	reply, err := encodeServerHelloBody(suite.ID(), random, publicKey, signature, finishMAC(material.s2cFinish, transcript))
	if err != nil {
		return nil, nil, err
	}

	return &Pending{
		session:    session,
		transcript: transcript,
		c2sFinish:  material.c2sFinish,
	}, encodeFrame(frameServerHello, reply), nil
}

// Confirm consumes the client Finished frame and returns the confirmed session.
func (s *Server) Confirm(pending *Pending, frame []byte) (*Session, error) {
	if s.err != nil {
		return nil, s.err
	}

	if pending == nil || pending.session == nil {
		return nil, errors.ErrInvalidMessage
	}

	typ, body, err := decodeFrame(frame)
	if err != nil {
		return nil, err
	}

	if typ != frameFinished || len(body) != macSize {
		return nil, errors.ErrInvalidMessage
	}

	if subtle.ConstantTimeCompare(finishMAC(pending.c2sFinish, pending.transcript), body) != 1 {
		return nil, errors.ErrInvalidSignature
	}

	return pending.session, nil
}

// selectSuite returns the first local suite that the client also offers.
func (s *Server) selectSuite(offered []uint8) aead.Suite {
	for _, suite := range s.suites {
		for _, id := range offered {
			if suite.ID() == id {
				return suite
			}
		}
	}

	return nil
}

// deriveKeyMaterial derives the directional traffic keys from the shared secret.
func deriveKeyMaterial(shared, salt, transcript []byte) (*keyMaterial, error) {
	out, err := hkdf.Key(sha256.New, shared, salt, transcriptLabel+string(transcript), keyMaterialSize)
	if err != nil {
		return nil, err
	}

	material := &keyMaterial{
		c2sKey:    out[0:32],
		s2cKey:    out[32:64],
		c2sFinish: out[64:96],
		s2cFinish: out[96:128],
	}
	copy(material.salt[:], out[128:keyMaterialSize])

	return material, nil
}

// transcriptHash hashes the concatenation of parts.
func transcriptHash(parts ...[]byte) []byte {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write(part)
	}

	return h.Sum(nil)
}

// finishMAC computes the key confirmation MAC over the transcript.
func finishMAC(key, transcript []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(transcript)

	return mac.Sum(nil)
}
