package xos

import (
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dobyte/due/v2/core/stat"
)

// Stat 获取文件信息
func Stat(filePath string) (stat.FileInfo, error) {
	return stat.Stat(filePath)
}

// IsDir 是否是目录
func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// IsFile 是否是文件
func IsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Split 将路径分割成目录、文件、文件名、后缀
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

// WriteFile 写文件
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
