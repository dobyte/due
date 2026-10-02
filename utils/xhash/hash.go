package xhash

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"hash/fnv"
	"io"
)

// MD5 computes the MD5 hash of str.
func MD5(str string) string {
	h := md5.New()
	_, _ = io.WriteString(h, str)
	return hex.EncodeToString(h.Sum(nil))
}

// SHA256 computes the SHA-256 hash of data, or its HMAC-SHA256 when key is not empty.
func SHA256(data string, key string) string {
	if key != "" {
		h := hmac.New(sha256.New, []byte(key))
		h.Write([]byte(data))
		return hex.EncodeToString(h.Sum(nil))
	}

	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}

// FNV32 computes the FNV-32 hash of key.
func FNV32(key string) uint32 {
	h := fnv.New32()
	h.Write([]byte(key))
	return h.Sum32()
}

// FNV32a computes the FNV-32a hash of key.
func FNV32a(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}

// FNV64 computes the FNV-64 hash of key.
func FNV64(key string) uint64 {
	h := fnv.New64()
	h.Write([]byte(key))
	return h.Sum64()
}

// FNV64a computes the FNV-64a hash of key.
func FNV64a(key string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	return h.Sum64()
}
