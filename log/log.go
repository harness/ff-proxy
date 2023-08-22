package log

import (
	"context"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger defines a logger with multiple logging levels. When using the logger
// calls to its methods should include a brief message describing what happened
// and then key value pairs that contain additional info e.g.
//
// logger.Error("failed to retrieve data from DB", "user_id", "1234")
type Logger interface {
	// Info logs a message at the info leve with some additional context via the
	// passed key value pairs. It also extracts values from the context and logs
	// them out.
	Info(msg string, keyvals ...interface{})

	// Debug logs a message at the debug level with some additional context via
	// the passed key value pairs. It also extracts values from the context and
	// logs them out.
	Debug(msg string, keyvals ...interface{})

	// Error logs a message at the error level with some additional context via the
	// passed key value pairs. It also extracts values from the context and logs
	// them out.
	Error(msg string, keyvals ...interface{})

	// Warn logs a message at the warning level with some additional context via the
	// passed key value pairs. It also extracts values from the context and logs
	// them out.
	Warn(msg string, keyvals ...interface{})

	// With adds fields of key value pairs to the logging context. When processing
	// pairs the first element is used as the key and the second as the value.
	With(keyvals ...interface{}) Logger
}

// ContextualLogger defines a logger that can extract and log values from a context
// as well as logging a message and keyvals. When using the logger
// calls to its methods should include a brief message describing what happened
// and then key value pairs that contain additional info e.g.
//
// logger.Error(ctx, "failed to retrieve data from DB", "user_id", "1234")
type ContextualLogger interface {
	// Info logs a message at the info leve with some additional context via the
	// passed key value pairs. It also extracts values from the context and logs
	// them out. Key values and values extracted from the context are treated
	// as they are in With.
	Info(ctx context.Context, msg string, keyvals ...interface{})

	// Debug logs a message at the debug level with some additional context via
	// the passed key value pairs. It also extracts values from the context and
	// logs them out. Key values and values extracted from the context are treated
	// as they are in With.
	Debug(ctx context.Context, msg string, keyvals ...interface{})

	// Error logs a message at the error level with some additional context via the
	// passed key value pairs. It also extracts values from the context and logs
	// them out. Key values and values extracted from the context are treated
	// as they are in With.
	Error(ctx context.Context, msg string, keyvals ...interface{})

	// Warn logs a message at the warning level with some additional context via the
	// passed key value pairs. It also extracts values from the context and logs
	// them out. Key values and values extracted from the context are treated
	// as they are in With.
	Warn(ctx context.Context, msg string, keyvals ...interface{})

	// With adds fields of key value pairs to the logging context. When processing
	// pairs the first element is used as the key and the second as the value.
	With(keyvals ...interface{}) ContextualLogger
}

// contextualLogger is a type that wraps a logger and can extract values
// from a context and log them out using the wrapped logger.
type contextualLogger struct {
	logger    Logger
	extractFn func(context.Context) []interface{}
}

// NewContextualLogger creates a contextual logger
func NewContextualLogger(logger Logger, extractFn func(context.Context) []interface{}) ContextualLogger {
	return contextualLogger{logger: logger, extractFn: extractFn}
}

// Info logs a message at the info leve with some additional context via the
// passed key value pairs. It also extracts values from the context and logs
// them out. Key values and values extracted from the context are treated
// as they are in With.
func (s contextualLogger) Info(ctx context.Context, msg string, args ...interface{}) {
	args = append(args, s.extractFn(ctx)...)
	s.logger.Info(msg, args...)
}

// Debug logs a message at the debug level with some additional context via
// the passed key value pairs. It also extracts values from the context and
// logs them out. Key values and values extracted from the context are treated
// as they are in With.
func (s contextualLogger) Debug(ctx context.Context, msg string, args ...interface{}) {
	args = append(args, s.extractFn(ctx)...)
	s.logger.Debug(msg, args...)
}

// Error logs a message at the error level with some additional context via the
// passed key value pairs. It also extracts values from the context and logs
// them out. Key values and values extracted from the context are treated
// as they are in With.
func (s contextualLogger) Error(ctx context.Context, msg string, args ...interface{}) {
	args = append(args, s.extractFn(ctx)...)
	s.logger.Error(msg, args...)
}

// Warn logs a message at the warning level with some additional context via the
// passed key value pairs. It also extracts values from the context and logs
// them out.
func (s contextualLogger) Warn(ctx context.Context, msg string, args ...interface{}) {
	args = append(args, s.extractFn(ctx)...)
	s.logger.Warn(msg, args...)
}

// With adds fields of key value pairs to the logging context. When processing
// pairs the first element is used as the key and the second as the value.
func (s contextualLogger) With(keyvals ...interface{}) ContextualLogger {
	og, ok := s.logger.(StructuredLogger)
	if !ok {
		return s
	}

	og.zl = *og.zl.With(keyvals...)
	s.logger = og
	return s
}

// StructuredLogger implements the Logger interface
type StructuredLogger struct {
	zl zap.SugaredLogger
}

// NewStructuredLogger creates a StructuredLogger
func NewStructuredLogger(level string) (StructuredLogger, error) {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder

	config := zap.Config{
		Level:             zap.NewAtomicLevel(),
		Development:       false,
		DisableCaller:     false,
		DisableStacktrace: true,
		Sampling:          nil,
		Encoding:          "json",
		EncoderConfig:     encoderConfig,
		OutputPaths:       []string{"stdout"},
		ErrorOutputPaths:  []string{"stdout"},
	}

	switch level {
	case "DEBUG":
		config.Level.SetLevel(zapcore.DebugLevel)
		config.Development = true
	case "ERROR":
		config.Level.SetLevel(zapcore.ErrorLevel)
	default:
		config.Level.SetLevel(zapcore.InfoLevel)
	}

	l, err := config.Build(zap.AddCallerSkip(1))
	if err != nil {
		return StructuredLogger{}, err
	}

	return StructuredLogger{*l.Sugar()}, nil
}

// NewStructuredLoggerFromSugar creates a StrucutredLogger from a Sugared Zap Logger
func NewStructuredLoggerFromSugar(s zap.SugaredLogger) StructuredLogger {
	return StructuredLogger{s}
}

// Info logs a message at the info leve with some additional context via the
// passed key value pairs. It also extracts values from the context and logs
// them out. Key values and values extracted from the context are treated
// as they are in With.
func (s StructuredLogger) Info(msg string, keyvals ...interface{}) {
	s.zl.Infow(msg, keyvals...)
}

// Debug logs a message at the debug level with some additional context via
// the passed key value pairs. It also extracts values from the context and
// logs them out. Key values and values extracted from the context are treated
// as they are in With.
func (s StructuredLogger) Debug(msg string, keyvals ...interface{}) {
	s.zl.Debugw(msg, keyvals...)
}

// Error logs a message at the error level with some additional context via the
// passed key value pairs. It also extracts values from the context and logs
// them out. Key values and values extracted from the context are treated
// as they are in With.
func (s StructuredLogger) Error(msg string, keyvals ...interface{}) {
	s.zl.Errorw(msg, keyvals...)
}

// Warn logs a message at the warning level with some additional context via the
// passed key value pairs. It also extracts values from the context and logs
// them out.
func (s StructuredLogger) Warn(msg string, keyvals ...interface{}) {
	s.zl.Warnw(msg, keyvals...)
}

// Sugar returns the underlying zap.SugaredLogger that the StructuredLogger uses
func (s StructuredLogger) Sugar() zap.SugaredLogger {
	return s.zl
}

// With adds fields of key value pairs to the logging context. When processing
// pairs the first element is used as the key and the second as the value.
func (s StructuredLogger) With(keyvals ...interface{}) Logger {
	return StructuredLogger{zl: *s.zl.With(keyvals...)}
}

// ExtractRequestValuesFromContext extracts request values from a context and
// returns them as a slice of strings
func ExtractRequestValuesFromContext(ctx context.Context) []interface{} {
	values := []interface{}{}
	reqID := getRequestID(ctx)
	if reqID != "" {
		values = append(values, "reqID")
		values = append(values, reqID)
	}

	return values
}

type contextKey string

// RequestIDKey is the key we associate with the requestID that we set in the request context
const RequestIDKey contextKey = "requestID"

// getRequestID extracts the requestID value from the context if it exists.
func getRequestID(ctx context.Context) string {
	requestID, _ := ctx.Value(RequestIDKey).(string)
	return requestID
}
