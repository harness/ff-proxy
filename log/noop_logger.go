package log

import "context"

// NoOpLogger is a type that implements the Logger interface but does nothing
// when it's methods are called
type NoOpLogger struct{}

// NewNoOpLogger returns a NoOpLogger
func NewNoOpLogger() NoOpLogger {
	return NoOpLogger{}
}

// Info does nothing on a NoOpLogger
func (n NoOpLogger) Info(msg string, keyvals ...interface{}) {}

// Debug does nothing on a NoOpLogger
func (n NoOpLogger) Debug(msg string, keyvals ...interface{}) {}

// Error  does nothing on a NoOpLogger
func (n NoOpLogger) Error(msg string, keyvals ...interface{}) {}

// Warn  does nothing on a NoOpLogger
func (n NoOpLogger) Warn(msg string, keyvals ...interface{}) {}

// With does nothing on a NoOpLogger
func (n NoOpLogger) With(keyvals ...interface{}) Logger { return NoOpLogger{} }

// NoOpContextualLogger is a type that implements the Logger interface but does nothing
// when it's methods are called
type NoOpContextualLogger struct{}

// NewNoOpContextualLogger returns a NoOpContextualLogger
func NewNoOpContextualLogger() NoOpContextualLogger {
	return NoOpContextualLogger{}
}

// Info does nothing
func (n NoOpContextualLogger) Info(ctx context.Context, msg string, keyvals ...interface{}) {}

// Debug does nothing
func (n NoOpContextualLogger) Debug(ctx context.Context, msg string, keyvals ...interface{}) {}

// Error  does nothing
func (n NoOpContextualLogger) Error(ctx context.Context, msg string, keyvals ...interface{}) {}

// Warn  does nothing on a NoOpLogger
func (n NoOpContextualLogger) Warn(ctx context.Context, msg string, keyvals ...interface{}) {}

// With does nothing on a NoOpLogger
func (n NoOpContextualLogger) With(keyvals ...interface{}) ContextualLogger {
	return NoOpContextualLogger{}
}
