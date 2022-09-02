/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/2 3:31 下午
 * @Desc: TODO
 */

package log

import (
	"io"
	"path/filepath"
	"strings"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
)

const (
	defaultFileExt  = "log"
	defaultFileName = "log"
)

type WriterOptions struct {
	Path       string
	MaxAge     time.Duration
	MaxSize    int64
	MaxBackups uint
	CutRule    CutRule
}

func NewWriter(opts WriterOptions) (io.Writer, error) {
	var (
		fileExt      string
		fileName     string
		newFileName  string
		rotationTime time.Duration
	)

	path, file := filepath.Split(opts.Path)

	list := strings.Split(file, ".")
	switch c := len(list); c {
	case 0:
		fileName, fileExt = defaultFileName, defaultFileExt
	case 1:
		fileName, fileExt = file, defaultFileExt
	case 2:
		fileName, fileExt = list[0], list[1]
	default:
		fileName, fileExt = strings.Join(list[:c-1], "."), list[c-1]
	}

	switch opts.CutRule {
	case CutByYear:
		newFileName = fileName + ".%Y." + fileExt
		rotationTime = 365 * 24 * time.Hour
	case CutByMonth:
		newFileName = fileName + ".%Y%m." + fileExt
		rotationTime = 31 * 24 * time.Hour
	case CutByDay:
		newFileName = fileName + ".%Y%m%d." + fileExt
		rotationTime = 24 * time.Hour
	case CutByHour:
		newFileName = fileName + ".%Y%m%d%H." + fileExt
		rotationTime = time.Hour
	case CutByMinute:
		newFileName = fileName + ".%Y%m%d%H%M." + fileExt
		rotationTime = time.Minute
	case CutBySecond:
		newFileName = fileName + ".%Y%m%d%H%M%S." + fileExt
		rotationTime = time.Second
	}

	srcFileName := filepath.Join(path, fileName+"."+fileExt)
	newFileName = filepath.Join(path, newFileName)

	return rotatelogs.New(
		newFileName,
		rotatelogs.WithLinkName(srcFileName),
		rotatelogs.WithMaxAge(opts.MaxAge),
		rotatelogs.WithRotationTime(rotationTime),
		rotatelogs.WithRotationSize(opts.MaxSize),
		rotatelogs.WithRotationCount(opts.MaxBackups),
	)
}
