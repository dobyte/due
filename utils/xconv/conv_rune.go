package xconv

// Rune converts val to a rune (equivalent to [Int32]).
func Rune(val any) rune {
	return Int32(val)
}

// Runes converts val to a rune slice (equivalent to [Int32s]).
func Runes(val any) []rune {
	return Int32s(val)
}

// RunePointer converts val to a pointer to rune.
func RunePointer(val any) *int32 {
	v := Rune(val)
	return &v
}

// RunesPointer converts val to a pointer to a rune slice.
func RunesPointer(val any) *[]int32 {
	v := Runes(val)
	return &v
}
