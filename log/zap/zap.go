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
	"github.com/dobyte/due/mode"
)

const (
	defaultOutLevel        = log.InfoLevel
	defaultOutFormat       = log.TextFormat
	defaultFileMaxAge      = 7 * 24 * time.Hour
	defaultFileMaxSize     = 100 * 1024 * 1024
	defaultFileCutRule     = log.CutByDay
	defaultTimestampFormat = "2006/01/02 15:04:05.000000"
)

const defaultNoneLevel log.Level = 0

var _ log.Logger = NewLogger()

var levelMap map[zapcore.Level]log.Level

func init() {
	levelMap = map[zapcore.Level]log.Level{
		zap.DebugLevel: log.DebugLevel,
		zap.InfoLevel:  log.InfoLevel,
		zap.WarnLevel:  log.WarnLevel,
		zap.ErrorLevel: log.ErrorLevel,
		zap.FatalLevel: log.FatalLevel,
		zap.PanicLevel: log.PanicLevel,
	}
}

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

	var (
		fileEncoder     zapcore.Encoder
		terminalEncoder zapcore.Encoder
	)
	switch o.outFormat {
	case log.JsonFormat:
		fileEncoder = encoder.NewJsonEncoder(o.timestampFormat, o.callerFormat)
		terminalEncoder = fileEncoder
	default:
		fileEncoder = encoder.NewTextEncoder(o.timestampFormat, o.callerFormat, false)
		terminalEncoder = encoder.NewTextEncoder(o.timestampFormat, o.callerFormat, true)
	}

	options := make([]zap.Option, 0, 3)
	options = append(options, zap.AddCaller())
	switch o.stackLevel {
	case log.DebugLevel:
		options = append(options, zap.AddStacktrace(zapcore.DebugLevel), zap.AddCallerSkip(1+o.callerSkip))
	case log.InfoLevel:
		options = append(options, zap.AddStacktrace(zapcore.InfoLevel), zap.AddCallerSkip(1+o.callerSkip))
	case log.WarnLevel:
		options = append(options, zap.AddStacktrace(zapcore.WarnLevel), zap.AddCallerSkip(1+o.callerSkip))
	case log.ErrorLevel:
		options = append(options, zap.AddStacktrace(zapcore.ErrorLevel), zap.AddCallerSkip(1+o.callerSkip))
	case log.FatalLevel:
		options = append(options, zap.AddStacktrace(zapcore.FatalLevel), zap.AddCallerSkip(1+o.callerSkip))
	case log.PanicLevel:
		options = append(options, zap.AddStacktrace(zapcore.PanicLevel), zap.AddCallerSkip(1+o.callerSkip))
	}

	l := &Logger{opts: o}

	var cores []zapcore.Core
	if o.outFile != "" {
		if o.enableLeveledStorage {
			cores = append(cores,
				zapcore.NewCore(fileEncoder, l.buildWriteSyncer(log.DebugLevel), l.buildLevelEnabler(log.DebugLevel)),
				zapcore.NewCore(fileEncoder, l.buildWriteSyncer(log.InfoLevel), l.buildLevelEnabler(log.InfoLevel)),
				zapcore.NewCore(fileEncoder, l.buildWriteSyncer(log.WarnLevel), l.buildLevelEnabler(log.WarnLevel)),
				zapcore.NewCore(fileEncoder, l.buildWriteSyncer(log.ErrorLevel), l.buildLevelEnabler(log.ErrorLevel)),
				zapcore.NewCore(fileEncoder, l.buildWriteSyncer(log.FatalLevel), l.buildLevelEnabler(log.FatalLevel)),
				zapcore.NewCore(fileEncoder, l.buildWriteSyncer(log.PanicLevel), l.buildLevelEnabler(log.PanicLevel)),
			)
		} else {
			cores = append(cores, zapcore.NewCore(fileEncoder, l.buildWriteSyncer(defaultNoneLevel), l.buildLevelEnabler(defaultNoneLevel)))
		}
	}

	if mode.IsDebugMode() {
		cores = append(cores, zapcore.NewCore(terminalEncoder, zapcore.AddSync(os.Stdout), l.buildLevelEnabler(defaultNoneLevel)))
	}

	if len(cores) >= 0 {
		l.logger = zap.New(zapcore.NewTee(cores...), options...).Sugar()
	}

	return l
}

func (l *Logger) buildWriteSyncer(level log.Level) zapcore.WriteSyncer {
	writer, err := log.NewWriter(log.WriterOptions{
		Path:    l.opts.outFile,
		Level:   level,
		MaxAge:  l.opts.fileMaxAge,
		MaxSize: l.opts.fileMaxSize,
		CutRule: l.opts.fileCutRule,
	})
	if err != nil {
		panic(err)
	}

	return zapcore.AddSync(writer)
}

func (l *Logger) buildLevelEnabler(level log.Level) zapcore.LevelEnabler {
	return zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		if v := levelMap[lvl]; l.opts.outLevel != defaultNoneLevel {
			return v >= l.opts.outLevel && (level == defaultNoneLevel || (level >= l.opts.outLevel && v >= level))
		} else {
			return level == defaultNoneLevel || v >= level
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

// 同步缓存中的日志
func (l *Logger) Sync() error {
	return l.logger.Sync()
}

// Close 关闭日志
func (l *Logger) Close() error {
	return l.logger.Sync()
}
