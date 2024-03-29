/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/1 4:56 下午
 * @Desc: TODO
 */

package zap

import (
	"os"

	"github.com/symsimmy/due/log/utils"
	"github.com/symsimmy/due/log/zap/internal/encoder"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _ utils.Logger = NewLogger()

type Logger struct {
	logger *zap.SugaredLogger
	opts   *options
}

func NewLogger(opts ...Option) *Logger {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	var (
		fileEncoder     zapcore.Encoder
		terminalEncoder zapcore.Encoder
	)
	switch o.format {
	case utils.JsonFormat:
		fileEncoder = encoder.NewJsonEncoder(o.timeFormat, o.callerFullPath)
		terminalEncoder = fileEncoder
	default:
		fileEncoder = encoder.NewTextEncoder(o.timeFormat, o.callerFullPath, false)
		terminalEncoder = encoder.NewTextEncoder(o.timeFormat, o.callerFullPath, true)
	}

	options := make([]zap.Option, 0, 3)
	options = append(options, zap.AddCaller())
	switch o.stackLevel {
	case utils.DebugLevel:
		options = append(options, zap.AddStacktrace(zapcore.DebugLevel), zap.AddCallerSkip(1+o.callerSkip))
	case utils.InfoLevel:
		options = append(options, zap.AddStacktrace(zapcore.InfoLevel), zap.AddCallerSkip(1+o.callerSkip))
	case utils.WarnLevel:
		options = append(options, zap.AddStacktrace(zapcore.WarnLevel), zap.AddCallerSkip(1+o.callerSkip))
	case utils.ErrorLevel:
		options = append(options, zap.AddStacktrace(zapcore.ErrorLevel), zap.AddCallerSkip(1+o.callerSkip))
	case utils.FatalLevel:
		options = append(options, zap.AddStacktrace(zapcore.FatalLevel), zap.AddCallerSkip(1+o.callerSkip))
	case utils.PanicLevel:
		options = append(options, zap.AddStacktrace(zapcore.PanicLevel), zap.AddCallerSkip(1+o.callerSkip))
	}

	l := &Logger{opts: o}

	var cores []zapcore.Core
	if o.file != "" {
		if o.classifiedStorage {
			cores = append(cores,
				zapcore.NewCore(fileEncoder, &zapcore.BufferedWriteSyncer{WS: l.buildWriteSyncer(utils.DebugLevel), Size: o.bufferSize * 2 * 1024}, l.buildLevelEnabler(utils.DebugLevel)),
				zapcore.NewCore(fileEncoder, &zapcore.BufferedWriteSyncer{WS: l.buildWriteSyncer(utils.InfoLevel), Size: o.bufferSize * 1024}, l.buildLevelEnabler(utils.InfoLevel)),
				zapcore.NewCore(fileEncoder, &zapcore.BufferedWriteSyncer{WS: l.buildWriteSyncer(utils.WarnLevel), Size: o.bufferSize * 1024}, l.buildLevelEnabler(utils.WarnLevel)),
				zapcore.NewCore(fileEncoder, &zapcore.BufferedWriteSyncer{WS: l.buildWriteSyncer(utils.ErrorLevel), Size: o.bufferSize * 1024}, l.buildLevelEnabler(utils.ErrorLevel)),
				zapcore.NewCore(fileEncoder, &zapcore.BufferedWriteSyncer{WS: l.buildWriteSyncer(utils.FatalLevel), Size: o.bufferSize * 1024}, l.buildLevelEnabler(utils.FatalLevel)),
				zapcore.NewCore(fileEncoder, &zapcore.BufferedWriteSyncer{WS: l.buildWriteSyncer(utils.PanicLevel), Size: o.bufferSize * 1024}, l.buildLevelEnabler(utils.PanicLevel)),
			)
		} else {
			cores = append(cores, zapcore.NewCore(fileEncoder, &zapcore.BufferedWriteSyncer{WS: l.buildWriteSyncer(utils.NoneLevel), Size: o.bufferSize * 1024}, l.buildLevelEnabler(utils.NoneLevel)))
		}
	}

	if o.stdout {
		cores = append(cores, zapcore.NewCore(terminalEncoder, zapcore.AddSync(os.Stdout), l.buildLevelEnabler(utils.NoneLevel)))
	}

	if len(cores) >= 0 {
		l.logger = zap.New(zapcore.NewTee(cores...), options...).Sugar()
	}

	return l
}

func (l *Logger) buildWriteSyncer(level utils.Level) zapcore.WriteSyncer {
	writer, err := utils.NewWriter(utils.WriterOptions{
		Path:    l.opts.file,
		Level:   level,
		MaxAge:  l.opts.fileMaxAge,
		MaxSize: l.opts.fileMaxSize * 1024 * 1024,
		CutRule: l.opts.fileCutRule,
	})
	if err != nil {
		panic(err)
	}

	return zapcore.AddSync(writer)
}

func (l *Logger) buildLevelEnabler(level utils.Level) zapcore.LevelEnabler {
	return zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		if v := levelMap[lvl]; l.opts.level != utils.NoneLevel {
			return v >= l.opts.level && (level == utils.NoneLevel || (level >= l.opts.level && v >= level))
		} else {
			return level == utils.NoneLevel || v >= level
		}
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

// Sync 同步缓存中的日志
func (l *Logger) Sync() error {
	return l.logger.Sync()
}

// Close 关闭日志
func (l *Logger) Close() error {
	return l.logger.Sync()
}

func (l *Logger) ChangeLevel(level string) {
	l.opts.level = utils.ParseLevel(level)
}
