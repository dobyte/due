package crypto

type Encryptor interface {
	// Encrypt 加密
	Encrypt(data []byte) ([]byte, error)
}

type Decryptor interface {
	// Decrypt 解密
	Decrypt(data []byte) ([]byte, error)
}
