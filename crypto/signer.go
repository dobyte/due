package crypto

type Signer interface {
	// Name returns the name of the signer.
	Name() string
	// Sign signs data.
	Sign(data []byte) ([]byte, error)
	// Verify verifies the signature of data.
	Verify(data []byte, signature []byte) (bool, error)
}
