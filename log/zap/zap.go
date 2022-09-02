/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/1 4:56 下午
 * @Desc: TODO
 */

package zap

import (
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/dobyte/due/log"
	"github.com/dobyte/due/log/zap/internal/encoder"
)

const (
	defaultOutLevel        = log.WarnLevel
	defaultOutFormat       = log.TextFormat
	defaultFileMaxAge      = 7 * 24 * time.Hour
	defaultFileMaxSize     = 100 * 1024 * 1024
	defaultFileCutRule     = log.CutByDay
	defaultTimestampFormat = "2006/01/02 15:04:05.000000"
)

var _ log.Logger = NewLogger()

type Logger struct {
	logger *zap.SugaredLogger
	opts   *options
}

func NewLogger(opts ...Option) *Logger {
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

	l := &Logger{opts: o}

	config := zap.NewProductionEncoderConfig()
	config.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncodeTime = func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendString(t.Format(o.timestampFormat))
	}

	var ed zapcore.Encoder
	switch o.outFormat {
	case log.JsonFormat:
		ed = zapcore.NewJSONEncoder(config)
	default:
		ed = encoder.NewTextEncoder(o.timestampFormat, o.callerFullPath)
	}

	enabler := l.buildLevelEnabler()
	if o.outFile != "" {
		if o.fileClassifyStorage {
			l.logger = zap.New(zapcore.NewTee(
				zapcore.NewCore(ed, l.buildWriteSyncer(log.DebugLevel), enabler),
				zapcore.NewCore(ed, l.buildWriteSyncer(log.InfoLevel), enabler),
				zapcore.NewCore(ed, l.buildWriteSyncer(log.WarnLevel), enabler),
				zapcore.NewCore(ed, l.buildWriteSyncer(log.ErrorLevel), enabler),
				zapcore.NewCore(ed, l.buildWriteSyncer(log.FatalLevel), enabler),
				zapcore.NewCore(ed, l.buildWriteSyncer(log.PanicLevel), enabler),
			)).Sugar()
		} else {
			l.logger = zap.New(zapcore.NewCore(ed, l.buildWriteSyncer(), enabler)).Sugar()
		}
	} else {
		l.logger = zap.New(zapcore.NewCore(ed, zapcore.AddSync(os.Stdout), enabler)).Sugar()
	}

	return l
}

func (l *Logger) buildWriteSyncer(level ...log.Level) zapcore.WriteSyncer {
	var path string
	if len(level) > 0 {

	} else {
		path = l.opts.outFile
	}

	writer, err := log.NewWriter(log.WriterOptions{
		Path:    path,
		MaxAge:  l.opts.fileMaxAge,
		MaxSize: l.opts.fileMaxSize,
		CutRule: l.opts.fileCutRule,
	})
	if err != nil {
		panic(err)
	}

	return zapcore.NewMultiWriteSyncer(zapcore.AddSync(writer), zapcore.AddSync(os.Stdout))
}

func (l *Logger) buildLevelEnabler() zapcore.LevelEnabler {
	return zap.LevelEnablerFunc(func(level zapcore.Level) bool {
		switch l.opts.outLevel {
		case log.DebugLevel:
			return level >= zapcore.DebugLevel
		case log.InfoLevel:
			return level >= zapcore.InfoLevel
		case log.WarnLevel:
			return level >= zapcore.WarnLevel
		case log.ErrorLevel:
			return level >= zapcore.ErrorLevel
		case log.FatalLevel:
			return level == zapcore.FatalLevel || level == zapcore.PanicLevel
		case log.PanicLevel:
			return level == zapcore.PanicLevel
		}

		return false
	})
}

// Debug 打印调试日志
func (l *Logger) Debug(a ...interface{}) {
	l.logger.Debug(a...)
}

// Debugf 打印调试模板日志
func (l *Logger) Debugf(format string, a ...interface{}) {
	l.logger.Debugf(format, a...)
}

// Info 打印信息日志
func (l *Logger) Info(a ...interface{}) {
	l.logger.Info(a...)
}

// Infof 打印信息模板日志
func (l *Logger) Infof(format string, a ...interface{}) {
	l.logger.Infof(format, a...)
}

// Warn 打印警告日志
func (l *Logger) Warn(a ...interface{}) {
	l.logger.Warn(a...)
}

// Warnf 打印警告模板日志
func (l *Logger) Warnf(format string, a ...interface{}) {
	l.logger.Warnf(format, a...)
}

// Error 打印错误日志
func (l *Logger) Error(a ...interface{}) {
	l.logger.Error(a...)
}

// Errorf 打印错误模板日志
func (l *Logger) Errorf(format string, a ...interface{}) {
	l.logger.Errorf(format, a...)
}

// Fatal 打印致命错误日志
func (l *Logger) Fatal(a ...interface{}) {
	l.logger.Fatal(a...)
}

// Fatalf 打印致命错误模板日志
func (l *Logger) Fatalf(format string, a ...interface{}) {
	l.logger.Fatalf(format, a...)
}

// Panic 打印Panic日志
func (l *Logger) Panic(a ...interface{}) {
	l.logger.Panic(a...)
}

// Panicf 打印Panic模板日志
func (l *Logger) Panicf(format string, a ...interface{}) {
	l.logger.Panicf(format, a...)
}

// 同步缓存中的日志
func (l *Logger) Sync() error {
	return l.logger.Sync()
}

// Close 关闭日志
func (l *Logger) Close() error {
	return l.logger.Sync()
}
