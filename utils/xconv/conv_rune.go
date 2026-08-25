package xconv

// Rune 将任意值转换为符文（等价于 Int32）
// @param val any 待转换的值
// @return @1 rune 转换后的符文
func Rune(val any) rune {
	return Int32(val)
}

// Runes 将任意值转换为符文切片（等价于 Int32s）
// @param val any 待转换的值
// @return @1 []rune 转换后的符文切片
func Runes(val any) []rune {
	return Int32s(val)
}

// RunePointer 将任意值转换为符文指针
// @param val any 待转换的值
// @return @1 *int32 转换后的符文指针
func RunePointer(val any) *int32 {
	v := Rune(val)
	return &v
}

// RunesPointer 将任意值转换为符文切片指针
// @param val any 待转换的值
// @return @1 *[]int32 转换后的符文切片指针
func RunesPointer(val any) *[]int32 {
	v := Runes(val)
	return &v
}
