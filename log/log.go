package log

import (
	"io"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
)

// Logger is a logger with multiple logging levels
type Logger interface {
	Info(keyvals ...interface{})
	Debug(keyvals ...interface{})
	Error(keyvals ...interface{})
}

type logger struct {
	log.Logger
	debug bool
}

// NewLogger returns a new logger
func NewLogger(w io.Writer, debug bool) Logger {
	w = log.NewSyncWriter(w)
	kitlogger := log.NewLogfmtLogger(w)

	level.NewFilter(kitlogger, level.AllowAll())
	return &logger{kitlogger, debug}
}

// Info logs keyvals at the info level
func (l *logger) Info(keyvals ...interface{}) {
	level.Info(l).Log(keyvals...)
}

// Debug logs keyvals at the debug level and adds the caller to the log
func (l *logger) Debug(keyvals ...interface{}) {
	if !l.debug {
		return
	}
	logWithCaller := log.With(l, "caller", log.DefaultCaller)
	level.Debug(logWithCaller).Log(keyvals...)
}

// Error logs keyvals at the error level
func (l *logger) Error(keyvals ...interface{}) {
	level.Error(l).Log(keyvals...)
}

// With returns a new Logger with keyvals prepended to log calls
func With(l Logger, keyvals ...interface{}) Logger {
	original, ok := l.(*logger)
	if !ok {
		return l
	}
	return &logger{Logger: log.With(original.Logger, keyvals...), debug: original.debug}
}
