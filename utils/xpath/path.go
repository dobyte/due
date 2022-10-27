package xpath

import (
	"os"
)

// IsDir 是否是目录
func IsDir(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return info.IsDir(), nil
}

// IsFile 是否是文件
func IsFile(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		return false, nil
	}

	return !info.IsDir(), nil
}
