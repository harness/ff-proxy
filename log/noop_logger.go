package log

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
