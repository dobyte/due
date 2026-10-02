package rsa

const Name = "rsa"

// EncryptPadding is the padding scheme used for encryption.
type EncryptPadding string

const (
	NORMAL EncryptPadding = "NORMAL" // RSA_PKCS1_PADDING; the block size for chunked encryption is modulus length - 11
	OAEP   EncryptPadding = "OAEP"   // RSA_PKCS1_OAEP_PADDING; the block size for chunked encryption is public modulus length - 2*hash length - 2
)

// SignPadding is the padding scheme used for signing.
type SignPadding string

const (
	PKCS SignPadding = "PKCS" // RSA PKCS #1 v1.5
	PSS  SignPadding = "PSS"  // RSA PSS
)
