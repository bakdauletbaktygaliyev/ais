package logger

import (
	"context"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type contextKey string

const loggerKey contextKey = "logger"

// Logger wraps zap.Logger with contextual helpers.
type Logger struct {
	*zap.Logger
}

// New creates a new Logger with the specified level and format.
func New(level, format string) (*Logger, error) {
	var zapLevel zapcore.Level
	if err := zapLevel.UnmarshalText([]byte(level)); err != nil {
		zapLevel = zapcore.InfoLevel
	}

	var encoder zapcore.Encoder
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderCfg.EncodeLevel = zapcore.LowercaseLevelEncoder

	if format == "console" {
		encoderCfg.EncodeLevel = zapcore.LowercaseColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	}

	core := zapcore.NewCore(
		encoder,
		zapcore.NewMultiWriteSyncer(zapcore.AddSync(os.Stdout)),
		zapLevel,
	)

	zapLogger := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	return &Logger{zapLogger}, nil
}

// WithContext stores the logger in a context.
func WithContext(ctx context.Context, l *Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

// FromContext retrieves the logger from a context, falling back to a no-op logger.
func FromContext(ctx context.Context) *Logger {
	if l, ok := ctx.Value(loggerKey).(*Logger); ok {
		return l
	}
	l, _ := New("info", "json")
	return l
}

// With returns a child logger with additional structured fields.
func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{l.Logger.With(fields...)}
}

// WithRepoID returns a child logger tagged with a repository ID.
func (l *Logger) WithRepoID(repoID string) *Logger {
	return l.With(zap.String("repo_id", repoID))
}

// WithComponent returns a child logger tagged with a component name.
func (l *Logger) WithComponent(component string) *Logger {
	return l.With(zap.String("component", component))
}

// WithRequestID returns a child logger tagged with a request ID.
func (l *Logger) WithRequestID(requestID string) *Logger {
	return l.With(zap.String("request_id", requestID))
}

// Sync flushes any buffered log entries. Applications should call this at exit.
func (l *Logger) Sync() {
	_ = l.Logger.Sync()
}
