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
	handler.Close()
}

func (l *ContextLogger) SetLevel(ctx string, level log.Level) {
	switch ctx {
	case GlobalLoggerCtx:
		l.GlobalLogger.Level = level
	default:
		clogger := l.getContextLogger(ctx)
		if clogger != nil {
			clogger.Level = level
		}
	}
}

func (l *ContextLogger) SetGlobalLoggerLevel(level log.Level) {
	l.SetLevel(GlobalLoggerCtx, level)
}

func (l *ContextLogger) SetLevelFromString(ctx, s string) error {
	level, err := log.ParseLevel(s)
	if err != nil {
		return err
	}

	l.SetLevel(ctx, level)

	return nil
}

func (l *ContextLogger) SetGlobalLoggerLevelFromString(s string) error {
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
	logger := l.getContextLogger(ctx)
	if logger == nil {
		return l.GlobalLogger.WithField("ctx", ctx)
	}
	return logger.WithField("ctx", ctx)
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
