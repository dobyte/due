package flag

import (
	"fmt"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/utils/xconv"
	"log"
	"os"
	"time"
)

var commandLine = newFlagSet(os.Args[1:])

func init() {
	commandLine.parse()
}

func Has(key string) bool {
	return commandLine.has(key)
}

func String(key string, def ...string) string {
	return commandLine.string(key, def...)
}

func Bool(key string, def ...bool) bool {
	return commandLine.bool(key, def...)
}

func Int(key string, def ...int) int {
	return commandLine.int(key, def...)
}

func Int8(key string, def ...int8) int8 {
	return commandLine.int8(key, def...)
}

func Int16(key string, def ...int16) int16 {
	return commandLine.int16(key, def...)
}

func Int32(key string, def ...int32) int32 {
	return commandLine.int32(key, def...)
}

func Int64(key string, def ...int64) int64 {
	return commandLine.int64(key, def...)
}

func Uint(key string, def ...uint) uint {
	return commandLine.uint(key, def...)
}

func Uint8(key string, def ...uint8) uint8 {
	return commandLine.uint8(key, def...)
}

func Uint16(key string, def ...uint16) uint16 {
	return commandLine.uint16(key, def...)
}

func Uint32(key string, def ...uint32) uint32 {
	return commandLine.uint32(key, def...)
}

func Uint64(key string, def ...uint64) uint64 {
	return commandLine.uint64(key, def...)
}

func Float32(key string, def ...float32) float32 {
	return commandLine.float32(key, def...)
}

func Float64(key string, def ...float64) float64 {
	return commandLine.float64(key, def...)
}

func Duration(key string, def ...time.Duration) time.Duration {
	return commandLine.duration(key, def...)
}

type flagSet struct {
	args   []string
	values map[string]string
}

func newFlagSet(args []string) *flagSet {
	f := &flagSet{
		args:   args,
		values: make(map[string]string),
	}
	f.parse()

	return f
}

func (f *flagSet) parse() {
	for {
		seen, err := f.parseOne()
		if seen {
			continue
		}

		if err == nil {
			break
		}

		log.Println(err)
	}
}

func (f *flagSet) parseOne() (bool, error) {
	if len(f.args) == 0 {
		return false, nil
	}

	s := f.args[0]
	if len(s) < 2 || s[0] != '-' {
		return false, nil
	}
	numMinuses := 1
	if s[1] == '-' {
		numMinuses++
		if len(s) == 2 { // "--" terminates the flags
			f.args = f.args[1:]
			return false, nil
		}
	}
	name := s[numMinuses:]
	if len(name) == 0 || name[0] == '-' || name[0] == '=' {
		return false, errors.New(fmt.Sprintf("bad flag syntax: %s", s))
	}

	// it's a flag. does it have an argument?
	f.args = f.args[1:]
	hasValue := false
	value := ""
	for i := 1; i < len(name); i++ { // equals cannot be first
		if name[i] == '=' {
			value = name[i+1:]
			hasValue = true
			name = name[0:i]
			break
		}
	}

	// It must have a value, which might be the next argument.
	if !hasValue && len(f.args) > 0 {
		if s = f.args[0]; s[0] != '-' {
			// value is the next arg
			hasValue = true
			value, f.args = f.args[0], f.args[1:]
		}
	}

	f.values[name] = value

	return true, nil
}

func (f *flagSet) has(key string) bool {
	_, ok := f.values[key]
	return ok
}

func (f *flagSet) string(key string, def ...string) string {
	if val, ok := f.values[key]; ok {
		return val
	}

	if len(def) > 0 {
		return def[0]
	}

	return ""
}

func (f *flagSet) bool(key string, def ...bool) bool {
	if val, ok := f.values[key]; ok {
		if val == "" {
			return true
		}

		return xconv.Bool(val)
	}

	if len(def) > 0 {
		return def[0]
	}

	return false
}

func (f *flagSet) int(key string, def ...int) int {
	if val, ok := f.values[key]; ok {
		return xconv.Int(val)
	}

	if len(def) > 0 {
		return def[0]
	}

	return 0
}

func (f *flagSet) int8(key string, def ...int8) int8 {
	if val, ok := f.values[key]; ok {
		return xconv.Int8(val)
	}

	if len(def) > 0 {
		return def[0]
	}

	return 0
}

func (f *flagSet) int16(key string, def ...int16) int16 {
	if val, ok := f.values[key]; ok {
		return xconv.Int16(val)
	}

	if len(def) > 0 {
		return def[0]
	}

	return 0
}

func (f *flagSet) int32(key string, def ...int32) int32 {
	if val, ok := f.values[key]; ok {
		return xconv.Int32(val)
	}

	if len(def) > 0 {
		return def[0]
	}

	return 0
}

func (f *flagSet) int64(key string, def ...int64) int64 {
	if val, ok := f.values[key]; ok {
		return xconv.Int64(val)
	}

	if len(def) > 0 {
		return def[0]
	}

	return 0
}

func (f *flagSet) uint(key string, def ...uint) uint {
	if val, ok := f.values[key]; ok {
		return xconv.Uint(val)
	}

	if len(def) > 0 {
		return def[0]
	}

	return 0
}

func (f *flagSet) uint8(key string, def ...uint8) uint8 {
	if val, ok := f.values[key]; ok {
		return xconv.Uint8(val)
	}

	if len(def) > 0 {
		return def[0]
	}

	return 0
}

func (f *flagSet) uint16(key string, def ...uint16) uint16 {
	if val, ok := f.values[key]; ok {
		return xconv.Uint16(val)
	}

	if len(def) > 0 {
		return def[0]
	}

	return 0
}

func (f *flagSet) uint32(key string, def ...uint32) uint32 {
	if val, ok := f.values[key]; ok {
		return xconv.Uint32(val)
	}

	if len(def) > 0 {
		return def[0]
	}

	return 0
}

func (f *flagSet) uint64(key string, def ...uint64) uint64 {
	if val, ok := f.values[key]; ok {
		return xconv.Uint64(val)
	}

	if len(def) > 0 {
		return def[0]
	}

	return 0
}

func (f *flagSet) float32(key string, def ...float32) float32 {
	if val, ok := f.values[key]; ok {
		return xconv.Float32(val)
	}

	if len(def) > 0 {
		return def[0]
	}

	return 0
}

func (f *flagSet) float64(key string, def ...float64) float64 {
	if val, ok := f.values[key]; ok {
		return xconv.Float64(val)
	}

	if len(def) > 0 {
		return def[0]
	}

	return 0
}

func (f *flagSet) duration(key string, def ...time.Duration) time.Duration {
	if val, ok := f.values[key]; ok {
		return xconv.Duration(val)
	}

	if len(def) > 0 {
		return def[0]
	}

	return 0
}
