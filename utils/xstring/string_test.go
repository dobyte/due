package xstring_test

import (
	"testing"

	"github.com/dobyte/due/v2/utils/xstring"
)

func TestFirstCharacterIsUpper(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"空字符串", "", false},
		{"非法UTF-8首字节", string([]byte{0xff}), false},
		{"大写字母", "A", true},
		{"小写字母", "a", false},
		{"数字", "1", false},
		{"中文", "中", false},
		{"合法U+FFFD", "\ufffd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xstring.FirstCharacterIsUpper(tt.s); got != tt.want {
				t.Fatalf("FirstCharacterIsUpper(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestFirstCharacterIsLower(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"空字符串", "", false},
		{"非法UTF-8首字节", string([]byte{0xff}), false},
		{"小写字母", "a", true},
		{"大写字母", "A", false},
		{"数字", "1", false},
		{"中文", "中", false},
		{"合法U+FFFD", "\ufffd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xstring.FirstCharacterIsLower(tt.s); got != tt.want {
				t.Fatalf("FirstCharacterIsLower(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestFirstCharacterIsNumber(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"空字符串", "", false},
		{"非法UTF-8首字节", string([]byte{0xff}), false},
		{"数字", "1", true},
		{"大写字母", "A", false},
		{"中文", "中", false},
		{"合法U+FFFD", "\ufffd", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xstring.FirstCharacterIsNumber(tt.s); got != tt.want {
				t.Fatalf("FirstCharacterIsNumber(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestFirstCharacterIsSymbol(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want bool
	}{
		{"空字符串", "", false},
		{"非法UTF-8首字节", string([]byte{0xff}), false},
		{"合法U+FFFD", "\ufffd", true},
		{"符号", "+", true},
		{"大写字母", "A", false},
		{"数字", "1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xstring.FirstCharacterIsSymbol(tt.s); got != tt.want {
				t.Fatalf("FirstCharacterIsSymbol(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestLength(t *testing.T) {
	tests := []struct {
		name string
		s    string
		want int
	}{
		{"空字符串", "", 0},
		{"纯ASCII", "abc", 3},
		{"纯中文", "你好", 2},
		{"混合", "a你b", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xstring.Length(tt.s); got != tt.want {
				t.Fatalf("Length(%q) = %v, want %v", tt.s, got, tt.want)
			}
		})
	}
}

func TestPaddingPrefix(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		padding string
		length  int
		want    string
	}{
		{"长度小于字符串", "hello", "0", 3, "hello"},
		{"长度等于字符串", "hello", "0", 5, "hello"},
		{"空填充", "1", "", 3, "1"},
		{"单字符填充", "1", "0", 3, "001"},
		{"多字符填充整除", "1", "00", 5, "00001"},
		{"多字符填充有余数", "1", "00", 4, "0001"},
		{"填充长度小于填充串", "1", "000", 3, "001"},
		{"多字节字符串", "好", "0", 5, "0000好"},
		{"多字节填充", "1", "中", 3, "中中1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xstring.PaddingPrefix(tt.s, tt.padding, tt.length); got != tt.want {
				t.Fatalf("PaddingPrefix(%q, %q, %d) = %q, want %q", tt.s, tt.padding, tt.length, got, tt.want)
			}
		})
	}
}

func TestPaddingSuffix(t *testing.T) {
	tests := []struct {
		name    string
		s       string
		padding string
		length  int
		want    string
	}{
		{"长度小于字符串", "hello", "0", 3, "hello"},
		{"长度等于字符串", "hello", "0", 5, "hello"},
		{"空填充", "1", "", 3, "1"},
		{"单字符填充", "1", "0", 3, "100"},
		{"多字符填充整除", "1", "00", 5, "10000"},
		{"多字符填充有余数", "1", "00", 4, "1000"},
		{"填充长度小于填充串", "1", "000", 3, "100"},
		{"多字节字符串", "好", "0", 5, "好0000"},
		{"多字节填充", "1", "中", 3, "1中中"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xstring.PaddingSuffix(tt.s, tt.padding, tt.length); got != tt.want {
				t.Fatalf("PaddingSuffix(%q, %q, %d) = %q, want %q", tt.s, tt.padding, tt.length, got, tt.want)
			}
		})
	}
}

func TestReplace(t *testing.T) {
	tests := []struct {
		name    string
		str     string
		start   int
		count   int
		replace string
		want    string
	}{
		{"负数起始位置", "hello", -1, 2, "*", "hello"},
		{"起始位置等于长度", "hello", 5, 2, "*", "hello"},
		{"起始位置大于长度", "hello", 6, 2, "*", "hello"},
		{"空字符串", "", 0, 1, "*", ""},
		{"负数数量替换到结尾", "hello", 1, -1, "*", "h****"},
		{"数量超长被截断", "hello", 2, 10, "*", "he***"},
		{"数量恰好到结尾", "hello", 2, 3, "*", "he***"},
		{"正常替换", "hello", 1, 2, "*", "h**lo"},
		{"数量为零", "hello", 2, 0, "*", "hello"},
		{"空替换串删除字符", "hello", 1, 2, "", "hlo"},
		{"空替换串到结尾", "hello", 1, -1, "", "h"},
		{"多字节字符串", "你好世界", 1, 2, "*", "你**界"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xstring.Replace(tt.str, tt.start, tt.count, tt.replace); got != tt.want {
				t.Fatalf("Replace(%q, %d, %d, %q) = %q, want %q", tt.str, tt.start, tt.count, tt.replace, got, tt.want)
			}
		})
	}
}
