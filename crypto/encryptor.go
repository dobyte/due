package crypto

type Encryptor interface {
	// Name returns the name of the encryptor.
	Name() string
	// Encrypt encrypts data.
	Encrypt(data []byte) ([]byte, error)
	// Decrypt decrypts data.
	Decrypt(data []byte) ([]byte, error)
}
