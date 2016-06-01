package api

import (
	"io"

	"github.com/Sirupsen/logrus"
	"github.com/labstack/echo/log"
)

type logrusLoggerWrapper struct {
	l *logrus.Logger
}

func wrapperLogger(logger *logrus.Logger) log.Logger {
	if logger == nil {
		logger = logrus.StandardLogger()
	}

	return &logrusLoggerWrapper{l: logger}
}

func (p logrusLoggerWrapper) SetOutput(io.Writer) {
	return
}

func (p logrusLoggerWrapper) SetLevel(level uint8) {
	p.l.Level = logrus.Level(level)
	return
}

func (p logrusLoggerWrapper) Print(args ...interface{}) {
	p.l.Print(args...)
	return
}

func (p logrusLoggerWrapper) Printf(format string, args ...interface{}) {
	p.l.Printf(format, args...)
	return
}

func (p logrusLoggerWrapper) Debug(args ...interface{}) {
	p.l.Debug(args...)
	return
}

func (p logrusLoggerWrapper) Debugf(format string, args ...interface{}) {
	p.l.Debugf(format, args...)
	return
}

func (p logrusLoggerWrapper) Info(args ...interface{}) {
	p.l.Info(args...)
	return
}

func (p logrusLoggerWrapper) Infof(format string, args ...interface{}) {
	p.l.Infof(format, args...)
	return
}

func (p logrusLoggerWrapper) Warn(args ...interface{}) {
	p.l.Warn(args...)
	return
}

func (p logrusLoggerWrapper) Warnf(format string, args ...interface{}) {
	p.l.Warnf(format, args...)
	return
}

func (p logrusLoggerWrapper) Error(args ...interface{}) {
	p.l.Error(args...)
	return
}

func (p logrusLoggerWrapper) Errorf(format string, args ...interface{}) {
	p.l.Errorf(format, args...)
	return
}

func (p logrusLoggerWrapper) Fatal(args ...interface{}) {
	p.l.Fatal(args)
	return
}

func (p logrusLoggerWrapper) Fatalf(format string, args ...interface{}) {
	p.l.Fatalf(format, args)
	return
}
