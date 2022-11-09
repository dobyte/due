package ecc

import (
	"crypto/elliptic"
)

type Curve int

const (
	P224 Curve = iota
	P256
	P384
	P521
)

func (c Curve) New() elliptic.Curve {
	switch c {
	case P224:
		return elliptic.P224()
	case P256:
		return elliptic.P256()
	case P384:
		return elliptic.P384()
	case P521:
		return elliptic.P521()
	default:
		return elliptic.P256()
	}
}
