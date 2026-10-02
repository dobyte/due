package ecc

import (
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/utils/xconv"
)

const (
	defaultEncryptorShareInfo1Key = "etc.crypto.ecc.encryptor.s1"
	defaultEncryptorShareInfo2Key = "etc.crypto.ecc.encryptor.s2"
	defaultEncryptorPublicKeyKey  = "etc.crypto.ecc.encryptor.publicKey"
	defaultEncryptorPrivateKeyKey = "etc.crypto.ecc.encryptor.privateKey"
)

type EncryptorOption func(o *encryptorOptions)

type encryptorOptions struct {
	// Shared information that must stay the same between encryption and decryption.
	// It is empty by default.
	s1 []byte

	// Shared information that must stay the same between encryption and decryption.
	// It is empty by default.
	s2 []byte

	// Public key. It may be a file path or a PEM-encoded key string.
	publicKey string

	// Private key. It may be a file path or a PEM-encoded key string.
	privateKey string
}

func defaultEncryptorOptions() *encryptorOptions {
	return &encryptorOptions{
		s1:         etc.Get(defaultEncryptorShareInfo1Key).Bytes(),
		s2:         etc.Get(defaultEncryptorShareInfo2Key).Bytes(),
		publicKey:  etc.Get(defaultEncryptorPublicKeyKey).String(),
		privateKey: etc.Get(defaultEncryptorPrivateKeyKey).String(),
	}
}

// WithEncryptorShareInfo sets the shared information used by encryption and decryption. Both
// values must stay the same between encryption and decryption.
func WithEncryptorShareInfo(s1, s2 string) EncryptorOption {
	return func(o *encryptorOptions) { o.s1, o.s2 = xconv.StringToBytes(s1), xconv.StringToBytes(s2) }
}

// WithEncryptorPublicKey sets the public key used for encryption.
func WithEncryptorPublicKey(publicKey string) EncryptorOption {
	return func(o *encryptorOptions) { o.publicKey = publicKey }
}

// WithEncryptorPrivateKey sets the private key used for decryption.
func WithEncryptorPrivateKey(privateKey string) EncryptorOption {
	return func(o *encryptorOptions) { o.privateKey = privateKey }
}
