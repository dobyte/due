package compress

type Compressor interface {
	Encode(input []byte) ([]byte, error)
	Decode(input []byte) ([]byte, error)
}
