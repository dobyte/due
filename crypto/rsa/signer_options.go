package rsa

import (
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/core/hash"
	"strings"
)

const (
	defaultSignerHashKey       = "config.crypto.rsa.signer.hash"
	defaultSignerPaddingKey    = "config.crypto.rsa.signer.padding"
	defaultSignerPublicKeyKey  = "config.crypto.rsa.signer.publicKey"
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

	// 公钥。可设置文件路径或公钥串
	publicKey string

	// 私钥。可设置文件路径或私钥串
	privateKey string
}

func defaultSignerOptions() *signerOptions {
	return &signerOptions{
		hash:       hash.Hash(strings.ToLower(config.Get(defaultSignerHashKey).String())),
		padding:    SignPadding(strings.ToUpper(config.Get(defaultSignerPaddingKey).String())),
		publicKey:  config.Get(defaultSignerPublicKeyKey).String(),
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

// WithSignerPublicKey 设置验签公钥
func WithSignerPublicKey(publicKey string) SignerOption {
	return func(o *signerOptions) { o.publicKey = publicKey }
}

// WithSignerPrivateKey 设置解密私钥
func WithSignerPrivateKey(privateKey string) SignerOption {
	return func(o *signerOptions) { o.privateKey = privateKey }
}
