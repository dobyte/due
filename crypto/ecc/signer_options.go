package ecc

import (
	"strings"

	"github.com/dobyte/due/v2/core/hash"
	"github.com/dobyte/due/v2/etc"
)

const (
	defaultSignerHashKey       = "etc.crypto.ecc.signer.hash"
	defaultSignerDelimiterKey  = "etc.crypto.ecc.signer.delimiter"
	defaultSignerPublicKeyKey  = "etc.crypto.ecc.signer.publicKey"
	defaultSignerPrivateKeyKey = "etc.crypto.ecc.signer.privateKey"
)

type SignerOption func(o *signerOptions)

type signerOptions struct {
	// Hash algorithm. It supports sha1, sha224, sha256, sha384 and sha512.
	// It defaults to sha256.
	hash hash.Hash

	// Signature delimiter.
	delimiter string

	// Public key. It may be a file path or a PEM-encoded key string.
	publicKey string

	// Private key. It may be a file path or a PEM-encoded key string.
	privateKey string
}

func defaultSignerOptions() *signerOptions {
	return &signerOptions{
		hash:       hash.Hash(strings.ToLower(etc.Get(defaultSignerHashKey).String())),
		delimiter:  etc.Get(defaultSignerDelimiterKey, " ").String(),
		publicKey:  etc.Get(defaultSignerPublicKeyKey).String(),
		privateKey: etc.Get(defaultSignerPrivateKeyKey).String(),
	}
}

// WithSignerHash sets the hash algorithm used for signing.
func WithSignerHash(hash hash.Hash) SignerOption {
	return func(o *signerOptions) { o.hash = hash }
}

// WithSignerDelimiter sets the delimiter that separates the two parts of a signature.
func WithSignerDelimiter(delimiter string) SignerOption {
	return func(o *signerOptions) { o.delimiter = delimiter }
}

// WithSignerPublicKey sets the public key used for verification.
func WithSignerPublicKey(publicKey string) SignerOption {
	return func(o *signerOptions) { o.publicKey = publicKey }
}

// WithSignerPrivateKey sets the private key used for signing.
func WithSignerPrivateKey(privateKey string) SignerOption {
	return func(o *signerOptions) { o.privateKey = privateKey }
}
