/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/1 4:56 下午
 * @Desc: TODO
 */

package zap

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/dobyte/due/log"
	"github.com/dobyte/due/log/zap/internal/encoder"
)

const (
	defaultFileExt         = "log"
	defaultOutLevel        = log.WarnLevel
	defaultOutFormat       = log.TextFormat
	defaultFileMaxAge      = 7 * 24 * time.Hour
	defaultFileMaxSize     = 100 * 1024 * 1024
	defaultFileCutRule     = log.CutByDay
	defaultTimestampFormat = "2006/01/02 15:04:05.000000"
)

var _ log.Logger = NewLogger()

type logger struct {
	logger *zap.Logger
}

type WriterOptions struct {
	Path       string
	MaxAge     time.Duration
	MaxSize    int64
	MaxBackups uint
	CutRule    log.CutRule
}

func NewLogger(opts ...Option) log.Logger {
	o := &options{
		outLevel:        defaultOutLevel,
		outFormat:       defaultOutFormat,
		fileMaxAge:      defaultFileMaxAge,
		fileMaxSize:     defaultFileMaxSize,
		fileCutRule:     defaultFileCutRule,
		timestampFormat: defaultTimestampFormat,
	}
	for _, opt := range opts {
		opt(o)
	}

	var (
		level  zapcore.LevelEnabler
		ed     zapcore.Encoder
		syncer zapcore.WriteSyncer
	)

	switch o.outLevel {
	case log.DebugLevel:
		level = zapcore.DebugLevel
	case log.InfoLevel:
		level = zapcore.InfoLevel
	case log.WarnLevel:
		level = zapcore.WarnLevel
	case log.ErrorLevel:
		level = zapcore.ErrorLevel
	case log.FatalLevel:
		level = zapcore.FatalLevel
	case log.PanicLevel:
		level = zapcore.PanicLevel
	}

	config := zap.NewProductionEncoderConfig()
	config.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncodeTime = func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(t.Format(o.timestampFormat))
	}

	switch o.outFormat {
	case log.JsonFormat:
		ed = zapcore.NewJSONEncoder(config)
	default:
		ed = encoder.NewTextEncoder(o.timestampFormat, o.callerFullPath)
	}

	if o.outFile != "" {
		writer, err := NewWriter(WriterOptions{
			Path:       o.outFile,
			MaxAge:     o.fileMaxAge,
			MaxSize:    o.fileMaxSize,
			MaxBackups: o.fileMaxBackups,
			CutRule:    o.fileCutRule,
		})
		if err != nil {
			panic(err)
		}

		syncer = zapcore.NewMultiWriteSyncer(zapcore.AddSync(writer), zapcore.AddSync(os.Stdout))
	} else {
		syncer = zapcore.AddSync(os.Stdout)
	}

	l := &logger{logger: zap.New(zapcore.NewCore(ed, syncer, level))}

	return l
}

func NewWriter(opts WriterOptions) (io.Writer, error) {
	var (
		path, file   = filepath.Split(opts.Path)
		list         = strings.Split(file, ".")
		fileExt      string
		fileName     string
		newFileName  string
		rotationTime time.Duration
	)

	switch c := len(list); c {
	case 1:
		fileName, fileExt = file, defaultFileExt
	case 2:
		fileName, fileExt = list[0], list[1]
	default:
		fileName, fileExt = strings.Join(list[:c-1], "."), list[c-1]
	}

	switch opts.CutRule {
	case log.CutByYear:
		newFileName = fileName + ".%Y." + fileExt
		rotationTime = 365 * 24 * time.Hour
	case log.CutByMonth:
		newFileName = fileName + ".%Y%m." + fileExt
		rotationTime = 31 * 24 * time.Hour
	case log.CutByDay:
		newFileName = fileName + ".%Y%m%d." + fileExt
		rotationTime = 24 * time.Hour
	case log.CutByHour:
		newFileName = fileName + ".%Y%m%d%H." + fileExt
		rotationTime = time.Hour
	case log.CutByMinute:
		newFileName = fileName + ".%Y%m%d%H%M." + fileExt
		rotationTime = time.Minute
	case log.CutBySecond:
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

// Trace 打印事件调试日志
func (l *logger) Trace(a ...interface{}) {

}

// Tracef 打印事件调试模板日志
func (l *logger) Tracef(format string, a ...interface{}) {
	//l.logger.Log(format, a...)
}

// Debug 打印调试日志
func (l *logger) Debug(a ...interface{}) {
	//defer l.logger.Sync()
	l.logger.Sugar().Debug("aaa", zap.String("url", "bbb"))
}

// Debugf 打印调试模板日志
func (l *logger) Debugf(format string, a ...interface{}) {}

// Info 打印信息日志
func (l *logger) Info(a ...interface{}) {}

// Infof 打印信息模板日志
func (l *logger) Infof(format string, a ...interface{}) {}

// Warn 打印警告日志
func (l *logger) Warn(a ...interface{}) {}

// Warnf 打印警告模板日志
func (l *logger) Warnf(format string, a ...interface{}) {}

// Error 打印错误日志
func (l *logger) Error(a ...interface{}) {}

// Errorf 打印错误模板日志
func (l *logger) Errorf(format string, a ...interface{}) {}

// Fatal 打印致命错误日志
func (l *logger) Fatal(a ...interface{}) {}

// Fatalf 打印致命错误模板日志
func (l *logger) Fatalf(format string, a ...interface{}) {}

// Panic 打印Panic日志
func (l *logger) Panic(a ...interface{}) {}

// Panicf 打印Panic模板日志
func (l *logger) Panicf(format string, a ...interface{}) {}
