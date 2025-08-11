/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/1 4:56 下午
 * @Desc: TODO
 */

package zap

import (
	"fmt"
	"os"

	"github.com/dobyte/due/log/zap/v2/internal/encoder"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/mode"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var _ log.Logger = NewLogger()

var levelMap map[zapcore.Level]log.Level

func init() {
	levelMap = map[zapcore.Level]log.Level{
		zap.DebugLevel:  log.DebugLevel,
		zap.InfoLevel:   log.InfoLevel,
		zap.WarnLevel:   log.WarnLevel,
		zap.ErrorLevel:  log.ErrorLevel,
		zap.FatalLevel:  log.FatalLevel,
		zap.DPanicLevel: log.PanicLevel,
	}
}

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
	case log.JsonFormat:
		fileEncoder = encoder.NewJsonEncoder(o.timeFormat, o.callerFullPath)
		terminalEncoder = fileEncoder
	default:
		fileEncoder = encoder.NewTextEncoder(o.timeFormat, o.callerFullPath, false)
		terminalEncoder = encoder.NewTextEncoder(o.timeFormat, o.callerFullPath, true)
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
	if o.file != "" {
		if o.classifiedStorage {
			cores = append(cores,
				zapcore.NewCore(fileEncoder, l.buildWriteSyncer(log.DebugLevel), l.buildLevelEnabler(log.DebugLevel)),
				zapcore.NewCore(fileEncoder, l.buildWriteSyncer(log.InfoLevel), l.buildLevelEnabler(log.InfoLevel)),
				zapcore.NewCore(fileEncoder, l.buildWriteSyncer(log.WarnLevel), l.buildLevelEnabler(log.WarnLevel)),
				zapcore.NewCore(fileEncoder, l.buildWriteSyncer(log.ErrorLevel), l.buildLevelEnabler(log.ErrorLevel)),
				zapcore.NewCore(fileEncoder, l.buildWriteSyncer(log.FatalLevel), l.buildLevelEnabler(log.FatalLevel)),
				zapcore.NewCore(fileEncoder, l.buildWriteSyncer(log.PanicLevel), l.buildLevelEnabler(log.PanicLevel)),
			)
		} else {
			cores = append(cores, zapcore.NewCore(fileEncoder, l.buildWriteSyncer(log.NoneLevel), l.buildLevelEnabler(log.NoneLevel)))
		}
	}

	if mode.IsDebugMode() && o.stdout {
		cores = append(cores, zapcore.NewCore(terminalEncoder, zapcore.AddSync(os.Stdout), l.buildLevelEnabler(log.NoneLevel)))
	}

	if len(cores) >= 0 {
		l.logger = zap.New(zapcore.NewTee(cores...), options...).Sugar()
	}

	return l
}

func (l *Logger) buildWriteSyncer(level log.Level) zapcore.WriteSyncer {
	writer, err := log.NewWriter(log.WriterOptions{
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

func (l *Logger) buildLevelEnabler(level log.Level) zapcore.LevelEnabler {
	return zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		if v := levelMap[lvl]; l.opts.level != log.NoneLevel {
			return v >= l.opts.level && (level == log.NoneLevel || (level >= l.opts.level && v >= level))
		} else {
			return level == log.NoneLevel || v >= level
		}
	})
}

// 打印日志
func (l *Logger) print(level log.Level, stack bool, a ...any) {
	if l.logger == nil {
		return
	}

	var msg string
	if len(a) == 1 {
		if str, ok := a[0].(string); ok {
			msg = str
		} else {
			msg = fmt.Sprint(a...)
		}
	} else {
		msg = fmt.Sprint(a...)
	}

	switch level {
	case log.DebugLevel:
		l.logger.Debugw(msg, encoder.StackFlag, stack)
	case log.InfoLevel:
		l.logger.Infow(msg, encoder.StackFlag, stack)
	case log.WarnLevel:
		l.logger.Warnw(msg, encoder.StackFlag, stack)
	case log.ErrorLevel:
		l.logger.Errorw(msg, encoder.StackFlag, stack)
	case log.FatalLevel:
		l.logger.Fatalw(msg, encoder.StackFlag, stack)
	case log.PanicLevel:
		l.logger.DPanicw(msg, encoder.StackFlag, stack)
	}
}

// Print 打印日志，不打印堆栈信息
func (l *Logger) Print(level log.Level, a ...any) {
	l.print(level, false, a...)
}

// Printf 打印模板日志，不打印堆栈信息
func (l *Logger) Printf(level log.Level, format string, a ...any) {
	l.print(level, false, fmt.Sprintf(format, a...))
}

// Debug 打印调试日志
func (l *Logger) Debug(a ...any) {
	l.print(log.DebugLevel, true, a...)
}

// Debugf 打印调试模板日志
func (l *Logger) Debugf(format string, a ...any) {
	l.print(log.DebugLevel, true, fmt.Sprintf(format, a...))
}

// Info 打印信息日志
func (l *Logger) Info(a ...any) {
	l.print(log.InfoLevel, true, a...)
}

// Infof 打印信息模板日志
func (l *Logger) Infof(format string, a ...any) {
	l.print(log.InfoLevel, true, fmt.Sprintf(format, a...))
}

// Warn 打印警告日志
func (l *Logger) Warn(a ...any) {
	l.print(log.WarnLevel, true, a...)
}

// Warnf 打印警告模板日志
func (l *Logger) Warnf(format string, a ...any) {
	l.print(log.WarnLevel, true, fmt.Sprintf(format, a...))
}

// Error 打印错误日志
func (l *Logger) Error(a ...any) {
	l.print(log.ErrorLevel, true, a...)
}

// Errorf 打印错误模板日志
func (l *Logger) Errorf(format string, a ...any) {
	l.print(log.ErrorLevel, true, fmt.Sprintf(format, a...))
}

// Fatal 打印致命错误日志
func (l *Logger) Fatal(a ...any) {
	l.print(log.FatalLevel, true, a...)
}

// Fatalf 打印致命错误模板日志
func (l *Logger) Fatalf(format string, a ...any) {
	l.print(log.FatalLevel, true, fmt.Sprintf(format, a...))
}

// Panic 打印Panic日志
func (l *Logger) Panic(a ...any) {
	l.print(log.PanicLevel, true, a...)
}

// Panicf 打印Panic模板日志
func (l *Logger) Panicf(format string, a ...any) {
	l.print(log.PanicLevel, true, fmt.Sprintf(format, a...))
}

// Sync 同步缓存中的日志
func (l *Logger) Sync() error {
	return l.logger.Sync()
}

// Close 关闭日志
func (l *Logger) Close() error {
	return l.logger.Sync()
}
