package xos

import (
	"os"
)

func Create(name string) (*os.File, error) {
	//dir := filepath.Dir(name)

	//fmt.Println(dir)

	err := os.Mkdir("./pprof/server", 0777)
	if err != nil {
		return nil, err
	}

	return os.Create(name)
}

// IsDir 是否是目录
func IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
