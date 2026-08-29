package xvalidate_test

import (
	"testing"

	"github.com/dobyte/due/v2/utils/xvalidate"
)

func TestIsTelephone(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"区号3位带横线7位", "028-5554540", true},
		{"区号4位带横线7位", "0285-5554540", true},
		{"区号3位带横线8位", "028-55545401", true},
		{"区号4位带横线8位", "0285-55545401", true},
		{"纯7位号码", "5554540", true},
		{"纯8位号码", "55545401", true},
		{"区号不带横线", "0285554540", true},
		{"空字符串", "", false},
		{"不足7位", "555454", false},
		{"9位无合法区号", "555454012", false},
		{"含非法字符", "abc-5554540", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xvalidate.IsTelephone(tt.in); got != tt.want {
				t.Fatalf("IsTelephone(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsMobile(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"13段", "13800138000", true},
		{"145段", "14500000000", true},
		{"147段", "14700000000", true},
		{"150段", "15000000000", true},
		{"154段被排除", "15400000000", false},
		{"162段", "16200000000", true},
		{"165段", "16500000000", true},
		{"166段", "16600000000", true},
		{"167段", "16700000000", true},
		{"160段未分配", "16000000000", false},
		{"170段", "17000000000", true},
		{"172段", "17200000000", true},
		{"174段未纳入", "17400000000", false},
		{"171段未纳入", "17100000000", false},
		{"178段", "17800000000", true},
		{"18段", "18000000000", true},
		{"19段", "19000000000", true},
		{"非手机号段", "12345678901", false},
		{"仅10位", "1380013800", false},
		{"12位", "138001380000", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xvalidate.IsMobile(tt.in); got != tt.want {
				t.Fatalf("IsMobile(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsIdCard(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"合法身份证", "512301195011260279", true},
		{"长度不足", "123456", false},
		{"校验位非法字符", "11010119900101001A", false},
		{"月份非法", "110101199013010011", false},
		{"出生年份早于1800", "110101179901010011", false},
		{"出生年份晚于今年", "110101299901010011", false},
		{"校验码不匹配", "110101199001010010", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xvalidate.IsIdCard(tt.in); got != tt.want {
				t.Fatalf("IsIdCard(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsAccount(t *testing.T) {
	tests := []struct {
		name string
		in   string
		min  int
		max  int
		want bool
	}{
		{"合法字母开头", "abc0", 4, 8, true},
		{"数字开头", "0abc", 4, 8, false},
		{"长度不足", "ab0", 4, 8, false},
		{"含点号", "ab.cd", 4, 8, true},
		{"含下划线", "ab_cd", 4, 8, true},
		{"含空格", "ab cd", 4, 8, false},
		{"含连字符", "ab-cd", 4, 8, true},
		{"长度超出", "abcdefghi", 4, 8, false},
		{"min小于1被钳制", "abc", 0, 5, true},
		{"min大于max", "abc", 5, 3, false},
		{"min等于max", "abc", 3, 3, true},
		{"min等于max长度不足", "ab", 3, 3, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xvalidate.IsAccount(tt.in, tt.min, tt.max); got != tt.want {
				t.Fatalf("IsAccount(%q, %d, %d) = %v, want %v", tt.in, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestIsEmail(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"合法邮箱", "yuebanfuxiao@gmail.com", true},
		{"多级域名", "a.b-c_d@sub.example.com", true},
		{"缺少@", "yuebanfuxiao", false},
		{"缺少域名点", "a@b", false},
		{"缺少本地部分", "@gmail.com", false},
		{"多个@", "a@b@c.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xvalidate.IsEmail(tt.in); got != tt.want {
				t.Fatalf("IsEmail(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsUrl(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"http", "http://www.baidu.com", true},
		{"大写HTTP", "HTTP://WWW.BAIDU.COM", true},
		{"https", "https://a.b", true},
		{"ftp", "ftp://x.y", true},
		{"file", "file://x.y", true},
		{"仅有协议", "http://", false},
		{"协议前有前缀", "xhttp://a.b", false},
		{"URL前有空格", "abc http://a.b", false},
		{"URL后有空格", "http://a.b xyz", false},
		{"非法协议", "httpx://a.b", false},
		{"空字符串", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xvalidate.IsUrl(tt.in); got != tt.want {
				t.Fatalf("IsUrl(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsQQ(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"5位QQ", "10000", true},
		{"更长QQ", "123456789", true},
		{"4位过短", "9999", false},
		{"首位为0", "01234", false},
		{"含非数字", "abc", false},
		{"空字符串", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xvalidate.IsQQ(tt.in); got != tt.want {
				t.Fatalf("IsQQ(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsDigit(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"零", "0", true},
		{"正整数", "11", true},
		{"正浮点", "11.1", true},
		{"负整数", "-1", true},
		{"负浮点", "-1.5", true},
		{"负零点几", "-0.5", true},
		{"零点几", "0.5", true},
		{"含字母", "1a2", false},
		{"多个小数点", "1.2.3", false},
		{"小数点结尾", "11.", false},
		{"小数点开头", ".5", false},
		{"前导零", "01", false},
		{"正号", "+1", false},
		{"空字符串", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xvalidate.IsDigit(tt.in); got != tt.want {
				t.Fatalf("IsDigit(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestIsNumber(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		lengths []int
		want    bool
	}{
		{"不限长度合法", "123", nil, true},
		{"不限长度空串", "", nil, false},
		{"不限长度含字母", "12a", nil, false},
		{"固定长度匹配", "123", []int{3}, true},
		{"固定长度不足", "12", []int{3}, false},
		{"固定长度超出", "1234", []int{3}, false},
		{"范围匹配", "123", []int{2, 5}, true},
		{"范围不足", "1", []int{2, 5}, false},
		{"范围超出", "123456", []int{2, 5}, false},
		{"最小大于最大", "123", []int{5, 2}, false},
		{"多余参数被忽略", "123", []int{1, 3, 999}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xvalidate.IsNumber(tt.in, tt.lengths...); got != tt.want {
				t.Fatalf("IsNumber(%q, %v) = %v, want %v", tt.in, tt.lengths, got, tt.want)
			}
		})
	}
}

func TestIn(t *testing.T) {
	tests := []struct {
		name string
		v    any
		set  any
		want bool
	}{
		{"标量命中", "a", []string{"a", "b", "c"}, true},
		{"标量未命中", "d", []string{"a", "b", "c"}, false},
		{"切片命中", []string{"a", "b"}, []string{"a", "b", "c"}, true},
		{"切片未命中", []string{"x", "y"}, []string{"a", "b", "c"}, false},
		{"空切片", []string{}, []string{"a"}, false},
		{"数组作为集合", "a", [3]string{"a", "b", "c"}, true},
		{"集合非切片数组", "a", "abc", false},
		{"集合为整数", "a", 123, false},
		{"空集合", "a", []string{}, false},
		{"集合含不可比较元素仍命中", "a", []any{[]int{1}, "a"}, true},
		{"切片在含不可比较集合中命中", []string{"a"}, []any{[]int{1}, "a"}, true},
		{"不可比较的值", map[string]int{"a": 1}, []string{"a"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xvalidate.In(tt.v, tt.set); got != tt.want {
				t.Fatalf("In(%v, %v) = %v, want %v", tt.v, tt.set, got, tt.want)
			}
		})
	}
}

func TestBetween(t *testing.T) {
	tests := []struct {
		name string
		in   string
		min  int
		max  int
		want bool
	}{
		{"在范围内", "hello", 1, 10, true},
		{"小于最小", "hello", 6, 10, false},
		{"大于最大", "hello", 1, 4, false},
		{"多字节字符", "你好", 1, 2, true},
		{"多字节字符超范围", "你好", 1, 1, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xvalidate.Between(tt.in, tt.min, tt.max); got != tt.want {
				t.Fatalf("Between(%q, %d, %d) = %v, want %v", tt.in, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestLength(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int
		want bool
	}{
		{"长度匹配", "hello", 5, true},
		{"长度不匹配", "hello", 4, false},
		{"多字节字符", "你好", 2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xvalidate.Length(tt.in, tt.n); got != tt.want {
				t.Fatalf("Length(%q, %d) = %v, want %v", tt.in, tt.n, got, tt.want)
			}
		})
	}
}

func TestMinLength(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int
		want bool
	}{
		{"达到最小", "hello", 5, true},
		{"大于最小", "hello", 4, true},
		{"小于最小", "hello", 6, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xvalidate.MinLength(tt.in, tt.n); got != tt.want {
				t.Fatalf("MinLength(%q, %d) = %v, want %v", tt.in, tt.n, got, tt.want)
			}
		})
	}
}

func TestMaxLength(t *testing.T) {
	tests := []struct {
		name string
		in   string
		n    int
		want bool
	}{
		{"达到最大", "hello", 5, true},
		{"小于最大", "hello", 6, true},
		{"大于最大", "hello", 4, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xvalidate.MaxLength(tt.in, tt.n); got != tt.want {
				t.Fatalf("MaxLength(%q, %d) = %v, want %v", tt.in, tt.n, got, tt.want)
			}
		})
	}
}
