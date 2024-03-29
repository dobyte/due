/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/1 4:56 下午
 * @Desc: TODO
 */

package zap

import (
	"context"
	"fmt"
	"github.com/symsimmy/due/log/utils"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"runtime"

	"github.com/symsimmy/due/log/zap/internal/encoder"
)

var _ utils.Logger = NewLogger()

var (
	levelMap      map[zapcore.Level]utils.Level
	loggerChannel = make(chan *logItem, 8192)
)

type logItem struct {
	args     []any
	template string
	level    zapcore.Level
	ctx      context.Context
}

func init() {
	levelMap = map[zapcore.Level]utils.Level{
		zap.DebugLevel: utils.DebugLevel,
		zap.InfoLevel:  utils.InfoLevel,
		zap.WarnLevel:  utils.WarnLevel,
		zap.ErrorLevel: utils.ErrorLevel,
		zap.FatalLevel: utils.FatalLevel,
		zap.PanicLevel: utils.PanicLevel,
	}
}

type AsyncLogger struct {
	logger *zap.SugaredLogger
	opts   *options
}

func NewAsyncLogger(opts ...Option) *AsyncLogger {
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
		fileEncoder = encoder.NewAsyncTextEncoder(o.timeFormat, o.callerFullPath, false)
		terminalEncoder = encoder.NewAsyncTextEncoder(o.timeFormat, o.callerFullPath, true)
	}

	l := &AsyncLogger{opts: o}

	var cores []zapcore.Core
	if o.file != "" {
		if o.classifiedStorage {
			cores = append(cores,
				zapcore.NewCore(fileEncoder, &zapcore.BufferedWriteSyncer{WS: l.buildWriteSyncer(utils.DebugLevel), Size: o.bufferSize * 1024}, l.buildLevelEnabler(utils.DebugLevel)),
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
		l.logger = zap.New(zapcore.NewTee(cores...)).Sugar()
	}

	go func() {
		for {
			select {
			case log := <-loggerChannel:
				l.logByLevel(log.level, log.template, log.args)
			}
		}
	}()

	return l
}

func (l *AsyncLogger) buildWriteSyncer(level utils.Level) zapcore.WriteSyncer {
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

func (l *AsyncLogger) buildLevelEnabler(level utils.Level) zapcore.LevelEnabler {
	return zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		if v := levelMap[lvl]; l.opts.level != utils.NoneLevel {
			return v >= l.opts.level && (level == utils.NoneLevel || (level >= l.opts.level && v >= level))
		} else {
			return level == utils.NoneLevel || v >= level
		}
	})
}

// Debug 打印调试日志
func (l *AsyncLogger) Debug(a ...interface{}) {
	if l.opts.asyncOutputCaller {
		newArgs := make([]interface{}, len(a)+1)
		_, file, line, _ := runtime.Caller(2)
		newArgs[0] = fmt.Sprintf("%s:%d", file, line)
		copy(newArgs[1:], a)
		loggerChannel <- &logItem{level: zapcore.DebugLevel, template: "", args: newArgs}
	} else {
		loggerChannel <- &logItem{level: zapcore.DebugLevel, template: "", args: a}
	}
}

// Debugf 打印调试模板日志
func (l *AsyncLogger) Debugf(format string, a ...interface{}) {
	if l.opts.asyncOutputCaller {
		newArgs := make([]interface{}, len(a)+1)
		_, file, line, _ := runtime.Caller(2)
		newArgs[0] = fmt.Sprintf("%s:%d", file, line)
		copy(newArgs[1:], a)
		loggerChannel <- &logItem{level: zapcore.DebugLevel, template: "%s " + format, args: newArgs}
	} else {
		loggerChannel <- &logItem{level: zapcore.DebugLevel, template: format, args: a}
	}
}

// Info 打印信息日志
func (l *AsyncLogger) Info(a ...interface{}) {
	if l.opts.asyncOutputCaller {
		newArgs := make([]interface{}, len(a)+1)
		_, file, line, _ := runtime.Caller(3)
		newArgs[0] = fmt.Sprintf("%s:%d", file, line)
		copy(newArgs[1:], a)
		loggerChannel <- &logItem{level: zapcore.InfoLevel, template: "", args: newArgs}
	} else {
		loggerChannel <- &logItem{level: zapcore.InfoLevel, template: "", args: a}
	}
}

// Infof 打印信息模板日志
func (l *AsyncLogger) Infof(format string, a ...interface{}) {
	if l.opts.asyncOutputCaller {
		newArgs := make([]interface{}, len(a)+1)
		_, file, line, _ := runtime.Caller(2)
		newArgs[0] = fmt.Sprintf("%s:%d", file, line)
		copy(newArgs[1:], a)
		loggerChannel <- &logItem{level: zapcore.InfoLevel, template: "%s " + format, args: newArgs}
	} else {
		loggerChannel <- &logItem{level: zapcore.InfoLevel, template: format, args: a}
	}
}

// Warn 打印警告日志
func (l *AsyncLogger) Warn(a ...interface{}) {
	if l.opts.asyncOutputCaller {
		newArgs := make([]interface{}, len(a)+1)
		_, file, line, _ := runtime.Caller(2)
		newArgs[0] = fmt.Sprintf("%s:%d", file, line)
		copy(newArgs[1:], a)
		loggerChannel <- &logItem{level: zapcore.WarnLevel, template: "", args: newArgs}
	} else {
		loggerChannel <- &logItem{level: zapcore.WarnLevel, template: "", args: a}
	}
}

// Warnf 打印警告模板日志
func (l *AsyncLogger) Warnf(format string, a ...interface{}) {
	if l.opts.asyncOutputCaller {
		newArgs := make([]interface{}, len(a)+1)
		_, file, line, _ := runtime.Caller(2)
		newArgs[0] = fmt.Sprintf("%s:%d", file, line)
		copy(newArgs[1:], a)
		loggerChannel <- &logItem{level: zapcore.WarnLevel, template: "%s " + format, args: newArgs}
	} else {
		loggerChannel <- &logItem{level: zapcore.WarnLevel, template: format, args: a}
	}
}

// Error 打印错误日志
func (l *AsyncLogger) Error(a ...interface{}) {
	if l.opts.asyncOutputCaller {
		newArgs := make([]interface{}, len(a)+1)
		_, file, line, _ := runtime.Caller(2)
		newArgs[0] = fmt.Sprintf("%s:%d", file, line)
		copy(newArgs[1:], a)
		loggerChannel <- &logItem{level: zapcore.ErrorLevel, template: "", args: newArgs}
	} else {
		loggerChannel <- &logItem{level: zapcore.ErrorLevel, template: "", args: a}
	}
}

// Errorf 打印错误模板日志
func (l *AsyncLogger) Errorf(format string, a ...interface{}) {
	if l.opts.asyncOutputCaller {
		newArgs := make([]interface{}, len(a)+1)
		_, file, line, _ := runtime.Caller(2)
		newArgs[0] = fmt.Sprintf("%s:%d", file, line)
		copy(newArgs[1:], a)
		loggerChannel <- &logItem{level: zapcore.ErrorLevel, template: "%s " + format, args: newArgs}
	} else {
		loggerChannel <- &logItem{level: zapcore.ErrorLevel, template: format, args: a}
	}
}

// Fatal 打印致命错误日志
func (l *AsyncLogger) Fatal(a ...interface{}) {
	if l.opts.asyncOutputCaller {
		newArgs := make([]interface{}, len(a)+1)
		_, file, line, _ := runtime.Caller(2)
		newArgs[0] = fmt.Sprintf("%s:%d", file, line)
		copy(newArgs[1:], a)
		loggerChannel <- &logItem{level: zapcore.FatalLevel, template: "", args: newArgs}
	} else {
		loggerChannel <- &logItem{level: zapcore.FatalLevel, template: "", args: a}
	}
}

// Fatalf 打印致命错误模板日志
func (l *AsyncLogger) Fatalf(format string, a ...interface{}) {
	if l.opts.asyncOutputCaller {
		newArgs := make([]interface{}, len(a)+1)
		_, file, line, _ := runtime.Caller(2)
		newArgs[0] = fmt.Sprintf("%s:%d", file, line)
		copy(newArgs[1:], a)
		loggerChannel <- &logItem{level: zapcore.FatalLevel, template: "%s " + format, args: newArgs}
	} else {
		loggerChannel <- &logItem{level: zapcore.FatalLevel, template: format, args: a}
	}
}

// Panic 打印Panic日志
func (l *AsyncLogger) Panic(a ...interface{}) {
	if l.opts.asyncOutputCaller {
		newArgs := make([]interface{}, len(a)+1)
		_, file, line, _ := runtime.Caller(2)
		newArgs[0] = fmt.Sprintf("%s:%d", file, line)
		copy(newArgs[1:], a)
		loggerChannel <- &logItem{level: zapcore.PanicLevel, template: "", args: newArgs}
	} else {
		loggerChannel <- &logItem{level: zapcore.PanicLevel, template: "", args: a}
	}
}

// Panicf 打印Panic模板日志
func (l *AsyncLogger) Panicf(format string, a ...interface{}) {
	if l.opts.asyncOutputCaller {
		newArgs := make([]interface{}, len(a)+1)
		_, file, line, _ := runtime.Caller(2)
		newArgs[0] = fmt.Sprintf("%s:%d", file, line)
		copy(newArgs[1:], a)
		loggerChannel <- &logItem{level: zapcore.PanicLevel, template: format, args: newArgs}
	} else {
		loggerChannel <- &logItem{level: zapcore.PanicLevel, template: format, args: a}
	}
}

// Sync 同步缓存中的日志
func (l *AsyncLogger) Sync() error {
	return l.logger.Sync()
}

// Close 关闭日志
func (l *AsyncLogger) Close() error {
	return l.logger.Sync()
}

func (l *AsyncLogger) logByLevel(level zapcore.Level, template string, args []any) {
	switch level {
	case zapcore.DebugLevel:
		l.logger.Debugf(template, args...)
	case zapcore.InfoLevel:
		l.logger.Infof(template, args...)
	case zapcore.WarnLevel:
		l.logger.Warnf(template, args...)
	case zapcore.ErrorLevel:
		l.logger.Errorf(template, args...)
	case zapcore.PanicLevel:
		l.logger.Panicf(template, args...)
	case zapcore.FatalLevel:
		l.logger.Fatalf(template, args...)
	}
}

func (l *AsyncLogger) ChangeLevel(level string) {
	l.opts.level = utils.ParseLevel(level)
}

func (l *AsyncLogger) ChangeAsyncOutputCaller(asyncOutputCaller bool) {
	l.opts.asyncOutputCaller = asyncOutputCaller
}
