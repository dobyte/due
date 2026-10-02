package log

import (
	"reflect"

	"github.com/dobyte/due/v2/etc"
)

const (
	defaultLevel        = LevelInfo
	defaultStackLevel   = LevelError
	defaultTimeFormat   = "2006/01/02 15:04:05.000000"
	defaultCallSkip     = 2
	defaultCallFullPath = false
)

const (
	defaultLevelKey        = "etc.log.level"
	defaultTerminalsKey    = "etc.log.terminals"
	defaultStackLevelKey   = "etc.log.stackLevel"
	defaultTimeFormatKey   = "etc.log.timeFormat"
	defaultCallSkipKey     = "etc.log.callSkip"
	defaultCallFullPathKey = "etc.log.callFullPath"
)

var defaultTerminals = []Terminal{TerminalConsole, TerminalFile}

// Option configures a logger.
type Option func(o *options)

type options struct {
	level        Level    // Output level
	syncers      []Syncer // Log syncers
	terminals    any      // Output terminals
	stackLevel   Level    // Log level from which the stack is emitted
	callSkip     int      // Number of stack frames to skip when emitting the stack
	callFullPath bool     // Whether to emit the full path of the caller file for the stack
	timeFormat   string   // Time format in the standard library layout, default 2006/01/02 15:04:05.000000
}

func defaultOptions() *options {
	opts := &options{
		level:        Level(etc.Get(defaultLevelKey, defaultLevel).String()),
		terminals:    defaultTerminals,
		stackLevel:   Level(etc.Get(defaultStackLevelKey, defaultStackLevel).String()),
		timeFormat:   etc.Get(defaultTimeFormatKey, defaultTimeFormat).String(),
		callSkip:     etc.Get(defaultCallSkipKey, defaultCallSkip).Int(),
		callFullPath: etc.Get(defaultCallFullPathKey, defaultCallFullPath).Bool(),
	}

	switch value := etc.Get(defaultTerminalsKey); value.Kind() {
	case reflect.Slice, reflect.Array:
		terminals := make([]Terminal, 0)

		if err := value.Scan(&terminals); err != nil || len(terminals) == 0 {
			opts.terminals = defaultTerminals
		} else {
			opts.terminals = terminals
		}
	case reflect.Map:
		terminals := make(map[Terminal][]Level)

		if err := value.Scan(&terminals); err != nil || len(terminals) == 0 {
			opts.terminals = defaultTerminals
		} else {
			opts.terminals = terminals
		}
	default:
		opts.terminals = defaultTerminals
	}

	return opts
}

// WithLevel sets the output level of the logger.
func WithLevel(level Level) Option {
	return func(o *options) { o.level = level }
}

// WithSyncers sets the log syncers.
func WithSyncers(syncers ...Syncer) Option {
	return func(o *options) { o.syncers = syncers }
}

// WithTerminals sets the output terminals of the logger.
func WithTerminals[T Terminal | []Terminal | map[Terminal][]Level](terminals ...T) Option {
	return func(o *options) {
		switch v := any(terminals).(type) {
		case []Terminal:
			o.terminals = v
		case [][]Terminal:
			if len(v) > 0 {
				o.terminals = v[0]
			}
		case []map[Terminal][]Level:
			if len(v) > 0 {
				o.terminals = v[0]
			}
		}
	}
}

// WithStackLevel sets the log level from which the stack is emitted.
func WithStackLevel(level Level) Option {
	return func(o *options) { o.stackLevel = level }
}

// WithTimeFormat sets the time format of the log output.
func WithTimeFormat(timeFormat string) Option {
	return func(o *options) { o.timeFormat = timeFormat }
}

// WithCallSkip sets the number of stack frames to skip when emitting the stack.
func WithCallSkip(skip int) Option {
	return func(o *options) { o.callSkip = skip }
}

// WithCallFullPath sets whether to emit the full path of the caller file for the stack.
func WithCallFullPath(fullPath bool) Option {
	return func(o *options) { o.callFullPath = fullPath }
}
