package xpath

import (
	"os"
	"path/filepath"
)

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

type FileInfo interface {
	os.FileInfo
	IsFile() bool
}

type fileStat struct {
	os.FileInfo
}

func Stat(path string) (FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	return &fileStat{FileInfo: info}, nil
}

func (fs *fileStat) IsFile() bool {
	return !fs.IsDir()
}
