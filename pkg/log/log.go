package log

import (
	loglib "github.com/ygrebnov/log"
)

type Level = loglib.Level
type Field = loglib.Field

const LevelDebug = loglib.LevelDebug

var String = loglib.String

type Logger interface {
	Log(level Level, message string, fields ...Field)
}

func NewSilentLogger() (Logger, error) {
	return loglib.NewSilentLogger()
}
