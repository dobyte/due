package xos

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dobyte/due/v2/core/stat"
)

// Stat 获取文件信息
// @param filePath string 文件路径
// @return @1 stat.FileInfo 文件信息
// @return @2 error 错误信息
func Stat(filePath string) (stat.FileInfo, error) {
	return stat.Stat(filePath)
}

// IsDir 判断路径是否为目录
// @param path string 路径
// @return @1 bool 是否为目录
func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// IsFile 判断路径是否为文件
// @param path string 路径
// @return @1 bool 是否为文件
func IsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Split 将路径分割成目录、文件名、文件名（不含后缀）、后缀
// @param path string 路径
// @return @1 string 目录部分
// @return @2 string 文件名部分（含后缀）
// @return @3 string 文件名（不含后缀）
// @return @4 string 后缀
func Split(path string) (dir, file, name, ext string) {
	dir, file = filepath.Split(path)
	for i := len(file) - 1; i >= 0 && !os.IsPathSeparator(file[i]); i-- {
		if file[i] == '.' {
			name = file[:i]
			ext = file[i+1:]
			return
		}
	}
	return
}

// WriteFile 写文件，目录不存在时自动创建
// @param file string 文件路径
// @param data []byte 待写入的数据
// @return @1 error 错误信息
func WriteFile(file string, data []byte) error {
	path := filepath.Dir(file)

	if !IsDir(path) {
		err := os.MkdirAll(path, fs.ModePerm)
		if err != nil {
			return err
		}
	}

	return os.WriteFile(file, data, fs.ModePerm)
}
