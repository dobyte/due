package xfile

import (
	"os"
	"time"
)

type FileInfo interface {
	// Name 获取文件名称
	Name() string
	// Size 获取文件大小
	Size() int64
	// Mode 获取文件模式
	Mode() os.FileMode
	// IsDir 检测文件是否是目录
	IsDir() bool
	// Sys 获取系统原始数据
	Sys() any
	// CreateTime 获取文件创建时间
	CreateTime() time.Time
	// ModifyTime 获取文件修改时间
	ModifyTime() time.Time
}

type fileStat struct {
	fi os.FileInfo
}

func Stat(filePath string) (FileInfo, error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	return &fileStat{fi: fi}, nil
}

// Name 获取文件名称
func (fs *fileStat) Name() string {
	return fs.fi.Name()
}

// Size 获取文件大小
func (fs *fileStat) Size() int64 {
	return fs.fi.Size()
}

// Mode 获取文件模式
func (fs *fileStat) Mode() os.FileMode {
	return fs.fi.Mode()
}

// ModifyTime 获取文件修改时间
func (fs *fileStat) ModifyTime() time.Time {
	return fs.fi.ModTime()
}

// IsDir 检测文件是否是目录
func (fs *fileStat) IsDir() bool {
	return fs.fi.IsDir()
}

// Sys 获取系统原始数据
func (fs *fileStat) Sys() any {
	return fs.fi.Sys()
}
