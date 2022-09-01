/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/1 4:56 下午
 * @Desc: TODO
 */

package zap

import (
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/dobyte/due/log"
)

const (
	defaultFileExt         = ".log"
	defaultOutLevel        = log.LevelWarn
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
		level   zapcore.LevelEnabler
		encoder zapcore.Encoder
		syncer  zapcore.WriteSyncer
	)

	switch o.outLevel {
	case log.LevelDebug:
		level = zapcore.DebugLevel
	case log.LevelInfo:
		level = zapcore.InfoLevel
	case log.LevelWarn:
		level = zapcore.WarnLevel
	case log.LevelError:
		level = zapcore.ErrorLevel
	case log.LevelFatal:
		level = zapcore.FatalLevel
	case log.LevelPanic:
		level = zapcore.PanicLevel
	}

	config := zap.NewProductionEncoderConfig()

	switch o.outFormat {
	case log.JsonFormat:
		encoder = zapcore.NewJSONEncoder(config)
	default:
		encoder = zapcore.NewConsoleEncoder(config)
	}

	l := &logger{
		logger: zap.New(zapcore.NewCore(encoder, syncer, level)),
	}

	return l
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
	l.logger.Sugar().Info("aaa", zap.String("url", "bbb"))
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
