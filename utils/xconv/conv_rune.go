package xconv

func Rune(val any) rune {
	return Int32(val)
}

func Runes(val any) []rune {
	return Int32s(val)
}

func RunePointer(val any) *int32 {
	v := Rune(val)
	return &v
}

func RunesPointer(val any) *[]int32 {
	v := Runes(val)
	return &v
}
