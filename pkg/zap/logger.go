package zap

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const timeFmt = "2006/01/02 15:04:05.000"

const (
	Dev Mode = iota
	Prod
)

type Mode int32

type Config struct {
	Mode  Mode
	Level string
	App   string
	Dir   string
	File  bool
}

// Logger is a logger impl.
type Logger struct {
	log    *zap.Logger
	msgKey string
}

var _ log.Logger = (*Logger)(nil)

// Option is logger option.
type Option func(*Logger)

// Log implements log.Logger
func (l *Logger) Log(level log.Level, keyvals ...interface{}) error {
	if len(keyvals) == 0 {
		return nil
	}
	if len(keyvals)%2 != 0 {
		keyvals = append(keyvals, "!MISSING-VALUE")
	}

	var msg string
	fields := make([]zap.Field, 0, len(keyvals)/2)

	for i := 0; i < len(keyvals); i += 2 {
		key := fmt.Sprintf("%v", keyvals[i])
		val := keyvals[i+1]

		if key == l.msgKey {
			msg = fmt.Sprintf("%v", val)
		} else {
			fields = append(fields, zap.Any(key, val))
		}
	}

	if msg == "" {
		msg = "no message"
	}

	switch level {
	case log.LevelDebug:
		l.log.Debug(msg, fields...)
	case log.LevelInfo:
		l.log.Info(msg, fields...)
	case log.LevelWarn:
		l.log.Warn(msg, fields...)
	case log.LevelError:
		l.log.Error(msg, fields...)
	case log.LevelFatal:
		l.log.Fatal(msg, fields...)
	default:
		l.log.Info(msg, fields...)
	}

	return nil
}

// Sync flushes any buffered log entries.
func (l *Logger) Sync() error {
	return l.log.Sync()
}

// ZapLogger returns the underlying zap logger.
func (l *Logger) ZapLogger() *zap.Logger {
	return l.log
}

// WithMessageKey with message key.
func WithMessageKey(key string) Option {
	return func(l *Logger) {
		l.msgKey = key
	}
}

// NewLogger creates a new logger.
func NewLogger(zapLogger *zap.Logger, opts ...Option) *Logger {
	l := &Logger{
		log:    zapLogger,
		msgKey: "msg",
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// NewLoggerWithConfig creates a new logger from config with options.
func NewLoggerWithConfig(cfg *Config, opts ...Option) *Logger {
	zapLogger := NewZapLogger(cfg)
	return NewLogger(zapLogger, opts...)
}

// NewZapLogger creates a new zap logger from config.
func NewZapLogger(cfg *Config) *zap.Logger {
	if cfg == nil {
		_, _ = fmt.Fprintln(os.Stderr, "logger: using default development logger with nil config")
		cfg = &Config{Mode: Dev, Level: "debug"}
	}
	if cfg.App == "" {
		cfg.App = "app"
	}
	lv := zap.NewAtomicLevel()
	if err := lv.UnmarshalText([]byte(cfg.Level)); err != nil {
		_ = lv.UnmarshalText([]byte("debug"))
		_, _ = fmt.Fprintf(os.Stderr, "logger: invalid log level %q, defaulting to DEBUG\n", cfg.Level)
	}
	cores := []zapcore.Core{
		zapcore.NewCore(
			zapcore.NewConsoleEncoder(encCfg(false)),
			zapcore.Lock(os.Stdout),
			//zapcore.Lock(os.Stderr),
			lv,
		),
	}
	if cfg.File || cfg.Mode == Prod {
		name := filepath.Join(cfg.Dir, cfg.App)
		cores = append(cores, fileCore(name+".log", lv))
		cores = append(cores, fileCore(name+"_error.log", zap.ErrorLevel))
	}
	return zap.New(zapcore.NewTee(cores...), zap.AddCaller(), zap.AddCallerSkip(2))
}

func fileCore(file string, lv zapcore.LevelEnabler) zapcore.Core {
	w := &lumberjack.Logger{
		Filename:   file,
		MaxSize:    100,
		MaxBackups: 7,
		MaxAge:     10,
		Compress:   true,
	}
	return zapcore.NewCore(
		zapcore.NewConsoleEncoder(encCfg(true)),
		zapcore.AddSync(w),
		lv,
	)
}

func encCfg(file bool) zapcore.EncoderConfig {
	cfg := zap.NewProductionEncoderConfig()
	cfg.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString("[" + t.Format(timeFmt) + "]")
	}
	cfg.EncodeCaller = func(c zapcore.EntryCaller, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString("[" + c.FullPath() + "]")
	}
	cfg.ConsoleSeparator = " "
	if !file {
		cfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
		cfg.EncodeCaller = zapcore.FullCallerEncoder
	} else {
		cfg.EncodeLevel = zapcore.CapitalLevelEncoder
		cfg.EncodeCaller = zapcore.FullCallerEncoder
	}
	return cfg
}

func calculateSkip() int {
	pc := make([]uintptr, 8)
	n := runtime.Callers(3, pc) // 调整跳过层数
	if n == 0 {
		return 2
	}

	frames := runtime.CallersFrames(pc[:n])

	for frame, more := frames.Next(); more; frame, more = frames.Next() {
		if strings.Contains(frame.Function, "kratos/v2/log.(*") {
			return 3 // Kratos 日志框架额外跳过
		}
	}
	return 2
}
