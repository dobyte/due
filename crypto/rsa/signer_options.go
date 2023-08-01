package rsa

import (
	"github.com/dobyte/due/v2/core/hash"
	"github.com/dobyte/due/v2/etc"
	"strings"
)

const (
	defaultSignerHashKey       = "etc.crypto.rsa.signer.hash"
	defaultSignerPaddingKey    = "etc.crypto.rsa.signer.padding"
	defaultSignerPublicKeyKey  = "etc.crypto.rsa.signer.publicKey"
	defaultSignerPrivateKeyKey = "etc.crypto.rsa.signer.privateKey"
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
		hash:       hash.Hash(strings.ToLower(etc.Get(defaultSignerHashKey).String())),
		padding:    SignPadding(strings.ToUpper(etc.Get(defaultSignerPaddingKey).String())),
		publicKey:  etc.Get(defaultSignerPublicKeyKey).String(),
		privateKey: etc.Get(defaultSignerPrivateKeyKey).String(),
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
