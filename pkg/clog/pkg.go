package clog

import (
	"io"
	"os"

	"github.com/apex/log"
)

var clogger = NewContextLogger(os.Stdout)

func AddLoggingContext(ctx string, w io.WriteCloser) {
	clogger.AddLoggingContext(ctx, w)
}

func RemoveLoggingContext(ctx string) {
	clogger.RemoveLoggingContext(ctx)
}

func SetLevel(ctx string, level log.Level) {
	clogger.SetLevel(ctx, level)
}

func SetGlobalLoggerLevel(level log.Level) {
	clogger.SetGlobalLoggerLevel(level)
}

func SetLevelFromString(ctx, s string) error {
	return clogger.SetLevelFromString(ctx, s)
}

func SetGlobalLoggerLevelFromString(s string) error {
	return clogger.SetGlobalLoggerLevelFromString(s)
}

func SetOutput(ctx string, w io.WriteCloser) error {
	return clogger.SetOutput(ctx, w)
}

func SetGlobalOutput(w io.WriteCloser) error {
	return clogger.SetGlobalOutput(w)
}

func UsingCtx(ctx string) *log.Entry {
	return clogger.UsingCtx(ctx)
}

func Global() *log.Entry {
	return clogger.Global()
}
