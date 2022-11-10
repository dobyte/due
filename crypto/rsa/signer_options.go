package rsa

import (
	"github.com/dobyte/due/crypto/hash"
	"strings"

	"github.com/dobyte/due/config"
)

const (
	defaultSignerHashKey       = "config.crypto.rsa.signer.hash"
	defaultSignerPaddingKey    = "config.crypto.rsa.signer.padding"
	defaultSignerPrivateKeyKey = "config.crypto.rsa.signer.privateKey"
)

type SignerOption func(o *signerOptions)

type signerOptions struct {
	// hash算法。支持sha1、sha224、sha256、sha384、sha512
	// 默认为sha256
	hash hash.Hash

	// 填充规则。支持PKCS和PSS
	// 默认为PSS
	padding SignPadding

	// 私钥。可设置文件路径或私钥串
	privateKey string
}

func defaultSignerOptions() *signerOptions {
	return &signerOptions{
		hash:       hash.Hash(strings.ToLower(config.Get(defaultSignerHashKey).String())),
		padding:    SignPadding(strings.ToUpper(config.Get(defaultSignerPaddingKey).String())),
		privateKey: config.Get(defaultSignerPrivateKeyKey).String(),
	}
}

// WithSignerHash 设置加密hash算法
func WithSignerHash(hash hash.Hash) SignerOption {
	return func(o *signerOptions) { o.hash = hash }
}

// WithSignerPadding 设置加密填充规则
func WithSignerPadding(padding SignPadding) SignerOption {
	return func(o *signerOptions) { o.padding = padding }
}

// WithSignerPrivateKey 设置解密私钥
func WithSignerPrivateKey(privateKey string) SignerOption {
	return func(o *signerOptions) { o.privateKey = privateKey }
}
