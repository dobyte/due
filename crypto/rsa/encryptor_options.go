package rsa

import (
	"strings"

	"github.com/dobyte/due/v2/core/hash"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/utils/xconv"
)

const (
	defaultEncryptorHashKey       = "etc.crypto.rsa.encryptor.hash"
	defaultEncryptorPaddingKey    = "etc.crypto.rsa.encryptor.padding"
	defaultEncryptorLabelKey      = "etc.crypto.rsa.encryptor.label"
	defaultEncryptorBlockSizeKey  = "etc.crypto.rsa.encryptor.blockSize"
	defaultEncryptorPublicKeyKey  = "etc.crypto.rsa.encryptor.publicKey"
	defaultEncryptorPrivateKeyKey = "etc.crypto.rsa.encryptor.privateKey"
)

type EncryptorOption func(o *encryptorOptions)

type encryptorOptions struct {
	// Hash algorithm. It supports sha1, sha224, sha256, sha384 and sha512.
	// It defaults to sha256.
	hash hash.Hash

	// Padding scheme. It supports NORMAL and OAEP.
	// It defaults to NORMAL.
	padding EncryptPadding

	// Label. It must stay the same between encryption and decryption.
	// It is empty by default.
	label []byte

	// Size of an encrypted data block in bytes. Because the length of the data that can be
	// encrypted at once is limited, data is split into blocks before encryption.
	// By default the largest block size allowed by the padding scheme is used.
	blockSize int

	// Public key. It may be a file path or a PEM-encoded key string.
	publicKey string

	// Private key. It may be a file path or a PEM-encoded key string.
	privateKey string
}

func defaultEncryptorOptions() *encryptorOptions {
	return &encryptorOptions{
		hash:       hash.Hash(strings.ToLower(etc.Get(defaultEncryptorHashKey).String())),
		padding:    EncryptPadding(strings.ToUpper(etc.Get(defaultEncryptorPaddingKey).String())),
		label:      etc.Get(defaultEncryptorLabelKey).Bytes(),
		blockSize:  etc.Get(defaultEncryptorBlockSizeKey).Int(),
		publicKey:  etc.Get(defaultEncryptorPublicKeyKey).String(),
		privateKey: etc.Get(defaultEncryptorPrivateKeyKey).String(),
	}
}

// WithEncryptorHash sets the hash algorithm used for encryption.
func WithEncryptorHash(hash hash.Hash) EncryptorOption {
	return func(o *encryptorOptions) { o.hash = hash }
}

// WithEncryptorPadding sets the padding scheme used for encryption.
func WithEncryptorPadding(padding EncryptPadding) EncryptorOption {
	return func(o *encryptorOptions) { o.padding = padding }
}

// WithEncryptorLabel sets the label used for encryption.
func WithEncryptorLabel(label string) EncryptorOption {
	return func(o *encryptorOptions) { o.label = xconv.StringToBytes(label) }
}

// WithEncryptorBlockSize sets the size of an encrypted data block.
func WithEncryptorBlockSize(blockSize int) EncryptorOption {
	return func(o *encryptorOptions) { o.blockSize = blockSize }
}

// WithEncryptorPublicKey sets the public key used for encryption.
func WithEncryptorPublicKey(publicKey string) EncryptorOption {
	return func(o *encryptorOptions) { o.publicKey = publicKey }
}

// WithEncryptorPrivateKey sets the private key used for decryption.
func WithEncryptorPrivateKey(privateKey string) EncryptorOption {
	return func(o *encryptorOptions) { o.privateKey = privateKey }
}
