package clog

import (
	"fmt"
	"io"
	"sync"

	"github.com/apex/log"
)

type ContextLogger struct {
	GlobalLogger   *log.Logger
	ContextLoggers sync.Map
}

const GlobalLoggerCtx = "global"

func NewContextLogger(globalLoggerWriter io.WriteCloser) *ContextLogger {
	return &ContextLogger{
		GlobalLogger: &log.Logger{
			Handler: NewHandler(globalLoggerWriter),
			Level:   log.InfoLevel,
		},
	}
}

func (l *ContextLogger) AddLoggingContext(ctx string, w io.WriteCloser) {
	logger := &log.Logger{
		Handler: NewHandler(w),
		Level:   log.InfoLevel,
	}
	l.ContextLoggers.Store(ctx, logger)
}

func (l *ContextLogger) RemoveLoggingContext(ctx string) {
	logger, ok := l.ContextLoggers.LoadAndDelete(ctx)
	if !ok {
		return
	}

	handler := loggerInterfaceToHandler(logger)
	handler.CloseWriter()
}

func (l *ContextLogger) SetLevel(ctx string, level log.Level) (log.Level, error) {
	switch ctx {
	case GlobalLoggerCtx:
		l.GlobalLogger.Level = level
		return level, nil
	default:
		clogger := l.getContextLogger(ctx)
		if clogger != nil {
			clogger.Level = level
			return level, nil
		}

		return log.InfoLevel, fmt.Errorf("no such ctx %s", ctx)
	}
}

func (l *ContextLogger) SetGlobalLoggerLevel(level log.Level) (log.Level, error) {
	return l.SetLevel(GlobalLoggerCtx, level)
}

func (l *ContextLogger) SetLevelFromString(ctx, s string) (log.Level, error) {
	level, err := log.ParseLevel(s)
	if err != nil {
		return log.InfoLevel, err
	}

	return l.SetLevel(ctx, level)
}

func (l *ContextLogger) SetGlobalLoggerLevelFromString(s string) (log.Level, error) {
	return l.SetLevelFromString(GlobalLoggerCtx, s)
}

func (l *ContextLogger) SetOutput(ctx string, w io.WriteCloser) error {
	handler := l.getContextLoggerHandler(ctx)
	if handler == nil {
		return fmt.Errorf("no such context %s", ctx)
	}

	handler.SetOutput(w)
	return nil
}

func (l *ContextLogger) SetGlobalOutput(w io.WriteCloser) error {
	return l.SetOutput(GlobalLoggerCtx, w)
}

func (l *ContextLogger) UsingCtx(ctx string) *log.Entry {
	// Shortcut - if ctx == GlobalLoggerCtx then just the user global logger context.
	if ctx == GlobalLoggerCtx {
		return l.GlobalLogger.WithField("ctx", GlobalLoggerCtx)
	}

	// Look up the logger by the context key. If no logger is found then use the
	// global logger context.
	logger := l.getContextLogger(ctx)
	if logger != nil {
		return logger.WithField("ctx", ctx)
	}

	// If we are here then ctx wasn't equal to GlobalLoggerCtx, and no logger context
	// was found for the ctx given. In this case we use the global logger context, but
	// we set ctx to the given key unless the given key is blank. If its blank set it
	// to GlobalLoggerCtx.
	//
	// If possible we use the given ctx key, even if a logger wasn't found for it. This
	// may help debugging by preserving some context that can be used when looking at
	// the log.
	if ctx == "" {
		// If the ctx key is blank then set it to "global" so that there is
		// a key associated with the ctx.
		ctx = GlobalLoggerCtx
	}
	return l.GlobalLogger.WithField("ctx", ctx)
}

func (l *ContextLogger) Global() *log.Entry {
	return l.UsingCtx(GlobalLoggerCtx)
}

func (l *ContextLogger) getContextLogger(ctx string) *log.Logger {
	logger, ok := l.ContextLoggers.Load(ctx)
	if !ok {
		return nil
	}

	return castToLogger(logger)
}

func castToLogger(logger interface{}) *log.Logger {
	clogger, ok := logger.(*log.Logger)
	if !ok {
		return nil
	}

	return clogger
}

func loggerInterfaceToHandler(logger interface{}) *Handler {
	clogger := castToLogger(logger)
	if clogger == nil {
		return nil
	}

	h, ok := clogger.Handler.(*Handler)
	if !ok {
		return nil
	}

	return h
}

func (l *ContextLogger) getContextLoggerHandler(ctx string) *Handler {
	clogger := l.getContextLogger(ctx)
	return loggerInterfaceToHandler(clogger)
}
