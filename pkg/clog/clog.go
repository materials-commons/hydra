package clog

import (
	"os"

	"github.com/apex/log"
)

var Log log.Interface = &log.Logger{
	Handler: NewHandler(os.Stdout),
	Level:   log.InfoLevel,
}

func SetLevel(l log.Level) {

}

func SetLevelFromString(s string) error {
	return nil
}

func WithFields(fields log.Fielder) *log.Entry {
	return nil
}

func WithField(key string, value interface{}) *log.Entry {
	return nil
}

func Debug(ctx string, msg string) {

}

func Info(ctx string, msg string) {

}

func Warn(ctx string, msg string) {

}

func Error(ctx string, msg string) {

}

func Fatal(ctx string, msg string) {

}

func Debugf(ctx string, msg string, v ...interface{}) {

}
