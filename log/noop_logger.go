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
func (n NoOpLogger) Info(_ string, _ ...interface{}) {}

// Debug does nothing on a NoOpLogger
func (n NoOpLogger) Debug(_ string, _ ...interface{}) {}

// Error  does nothing on a NoOpLogger
func (n NoOpLogger) Error(_ string, _ ...interface{}) {}

// Warn  does nothing on a NoOpLogger
func (n NoOpLogger) Warn(_ string, _ ...interface{}) {}

// With does nothing on a NoOpLogger
func (n NoOpLogger) With(_ ...interface{}) Logger { return NoOpLogger{} }

// NoOpContextualLogger is a type that implements the Logger interface but does nothing
// when it's methods are called
type NoOpContextualLogger struct{}

// NewNoOpContextualLogger returns a NoOpContextualLogger
func NewNoOpContextualLogger() NoOpContextualLogger {
	return NoOpContextualLogger{}
}

// Info does nothing
func (n NoOpContextualLogger) Info(_ context.Context, _ string, _ ...interface{}) {}

// Debug does nothing
func (n NoOpContextualLogger) Debug(_ context.Context, _ string, _ ...interface{}) {}

// Error  does nothing
func (n NoOpContextualLogger) Error(_ context.Context, _ string, _ ...interface{}) {}

// Warn  does nothing on a NoOpLogger
func (n NoOpContextualLogger) Warn(_ context.Context, _ string, _ ...interface{}) {}

// With does nothing on a NoOpLogger
func (n NoOpContextualLogger) With(_ ...interface{}) ContextualLogger {
	return NoOpContextualLogger{}
}
