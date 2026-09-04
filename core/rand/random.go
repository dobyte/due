package rand

type Random interface {
	Int() int
	IntN(n int) int
	Int32() int32
	Int32N(n int32) int32
	Int64() int64
	Int64N(n int64) int64
	Uint() uint
	UintN(n uint) uint
	Uint32() uint32
	Uint32N(n uint32) uint32
	Uint64() uint64
	Uint64N(n uint64) uint64
	Float32() float32
	Float64() float64
	ExpFloat64() float64
	NormFloat64() float64
	Perm(n int) []int
	Shuffle(n int, swap func(i, j int))
}
