package log

// NoOpLogger is a type that implements the Logger interface but does nothing
// when it's methods are called
type NoOpLogger struct{}

// NewNoOpLogger returns a NoOpLogger
func NewNoOpLogger() NoOpLogger {
	return NoOpLogger{}
}

// Info does nothing on a NoOpLogger
func (n NoOpLogger) Info(keyvals ...interface{}) {}

// Debug does nothing on a NoOpLogger
func (n NoOpLogger) Debug(keyvals ...interface{}) {}

// Error  does nothing on a NoOpLogger
func (n NoOpLogger) Error(keyvals ...interface{}) {}
