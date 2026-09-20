package log_test

import (
	"due-benchmark/log"
	"os"
	"testing"

	"github.com/arthurkiller/rollingwriter"
	duelog "github.com/dobyte/due/v2/log"
	duelogfile "github.com/dobyte/due/v2/log/file"
	gologger "github.com/donnie4w/go-logger/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// go test -bench=^Benchmark -benchmem

const (
	outDir    = "./temp"
	debugText = ">>>>>>this is debug message>>>>>>this is debug message"
)

var (
	stdSerialLogger             *log.StdLogger
	stdParallelLogger           *log.StdLogger
	zapSerialLogger             *zap.Logger
	zapParallelLogger           *zap.Logger
	zapSerialSugaredLogger      *zap.SugaredLogger
	zapParallelSugaredLogger    *zap.SugaredLogger
	dueSerialLogger             duelog.Logger
	dueParallelLogger           duelog.Logger
	rollingWriterSerialLogger   rollingwriter.RollingWriter
	rollingWriterParallelLogger rollingwriter.RollingWriter
	goLoggerSerialLogger        *gologger.Logging
	goLoggerParallelLogger      *gologger.Logging
)

func init() {
	if _, err := os.Stat(outDir); err != nil {
		_ = os.MkdirAll(outDir, 0755)
	}

	stdSerialLogger = log.NewStdLogger("./temp/s_std.log")

	stdParallelLogger = log.NewStdLogger("./temp/p_std.log")

	zapSerialLogger = zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(&lumberjack.Logger{
			Filename:   "./temp/s_zap.log",
			MaxSize:    500, // MB
			MaxBackups: 3,
			MaxAge:     28, // 天
			Compress:   true,
		}),
		zap.DebugLevel,
	))

	zapParallelLogger = zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(&lumberjack.Logger{
			Filename:   "./temp/p_zap.log",
			MaxSize:    500, // MB
			MaxBackups: 3,
			MaxAge:     28, // 天
			Compress:   true,
		}),
		zap.DebugLevel,
	))

	zapSerialSugaredLogger = zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(&lumberjack.Logger{
			Filename:   "./temp/s_zap_suger.log",
			MaxSize:    500, // MB
			MaxBackups: 3,
			MaxAge:     28, // 天
			Compress:   true,
		}),
		zap.DebugLevel,
	)).Sugar()

	zapParallelSugaredLogger = zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewDevelopmentEncoderConfig()),
		zapcore.AddSync(&lumberjack.Logger{
			Filename:   "./temp/p_zap_suger.log",
			MaxSize:    500, // MB
			MaxBackups: 3,
			MaxAge:     28, // 天
			Compress:   true,
		}),
		zap.DebugLevel,
	)).Sugar()

	dueSerialLogger = duelog.NewLogger(
		duelog.WithLevel(duelog.LevelDebug),
		duelog.WithTerminals(duelog.TerminalFile),
		duelog.WithTimeFormat("2006/01/02 15:04:05"),
		duelog.WithSyncers(duelogfile.NewSyncer(
			duelogfile.WithPath("./temp/s_due.log"),
			duelogfile.WithMaxSize(500*1024*1024),
		)),
	)

	dueParallelLogger = duelog.NewLogger(
		duelog.WithLevel(duelog.LevelDebug),
		duelog.WithTerminals(duelog.TerminalFile),
		duelog.WithTimeFormat("2006/01/02 15:04:05"),
		duelog.WithSyncers(duelogfile.NewSyncer(
			duelogfile.WithPath("./temp/p_due.log"),
			duelogfile.WithMaxSize(500*1024*1024),
		)),
	)

	rollingWriterSerialLogger, _ = rollingwriter.NewWriterFromConfig(&rollingwriter.Config{
		LogPath:       "./temp",
		FileName:      "s_rollingwriter",
		RollingPolicy: rollingwriter.WithoutRolling,
		WriterMode:    "lock",
	})

	rollingWriterParallelLogger, _ = rollingwriter.NewWriterFromConfig(&rollingwriter.Config{
		LogPath:       "./temp",
		FileName:      "p_rollingwriter",
		RollingPolicy: rollingwriter.WithoutRolling,
		WriterMode:    "lock",
	})

	goLoggerSerialLogger = gologger.NewLogger()
	goLoggerSerialLogger.SetRollingFile("./temp", "s_gologger.log", 500, gologger.MB)
	goLoggerSerialLogger.SetConsole(false)

	goLoggerParallelLogger = gologger.NewLogger()
	goLoggerParallelLogger.SetRollingFile("./temp", "p_gologger.log", 500, gologger.MB)
	goLoggerParallelLogger.SetConsole(false)
}

func Benchmark_Std_SerialIO(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = stdSerialLogger.Write([]byte(debugText + "\n"))
	}
}

func Benchmark_Std_ParallelIO(b *testing.B) {
	b.SetParallelism(20)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = stdParallelLogger.Write([]byte(debugText + "\n"))
		}
	})
}

func Benchmark_Zap_SerialIO(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		zapSerialLogger.Debug(debugText)
	}
	zapSerialLogger.Sync()
}

func Benchmark_Zap_ParallelIO(b *testing.B) {
	b.SetParallelism(20)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			zapParallelLogger.Info(debugText)
		}
	})
	zapParallelLogger.Sync()
}

func Benchmark_Zap_Suger_SerialIO(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		zapSerialSugaredLogger.Debug(debugText)
	}
	zapSerialSugaredLogger.Sync()
}

func Benchmark_Zap_Suger_ParallelIO(b *testing.B) {
	b.SetParallelism(20)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			zapParallelSugaredLogger.Info(debugText)
		}
	})
	zapParallelSugaredLogger.Sync()
}

func Benchmark_Due_SerialIO(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dueSerialLogger.Debug(debugText)
	}
}

func Benchmark_Due_ParallelIO(b *testing.B) {
	b.SetParallelism(20)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			dueParallelLogger.Debug(debugText)
		}
	})
}

func Benchmark_RollingWriter_SerialIO(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rollingWriterSerialLogger.Write([]byte(debugText))
	}
}

func Benchmark_RollingWriter_ParallelIO(b *testing.B) {
	b.SetParallelism(20)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = rollingWriterParallelLogger.Write([]byte(debugText))
		}
	})
}

func Benchmark_GoLogger_SerialIO(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		goLoggerSerialLogger.Debug(debugText)
	}
}

func Benchmark_GoLogger_ParallelIO(b *testing.B) {
	b.SetParallelism(20)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			goLoggerParallelLogger.Debug(debugText)
		}
	})
}
