package llog

import (
	"fmt"
	"log"
)

type LogLevel int

const (
	l_Fatal LogLevel = iota + 1
	l_Error
	l_Warning
	l_Info
	l_Debug
	l_DDebug
)

var Level LogLevel = l_DDebug

func SetLogLevelFromString(level string) {
	for i, l := range getLevelNames() {
		if l == level {
			Level = LogLevel(i)
			return
		}
	}
	Error(fmt.Sprintf("Invalid log level: %s", level))
}

func getLevelNames() []string {
	return []string{"Fatal", "Error", "Warning", "Info", "Debug", "DDebug"}
}

func (l LogLevel) IsValid() bool {
	return l >= l_Fatal && l <= l_DDebug
}

func (l LogLevel) String() string {
	if !l.IsValid() {
		return fmt.Sprintf("LogLevel(%d)", int(l))
	}

	return getLevelNames()[l-1]
}

func Log(l LogLevel, msg string) {
	msg = fmt.Sprintf("[%s] %s", l.String(), msg)

	if l == l_Fatal {
		log.Fatalln(msg)
	}

	if l <= Level {
		log.Println(msg)
	}
}

func Fatal(msg string) {
	Log(l_Fatal, msg)
}

func Error(msg string) {
	Log(l_Error, msg)
}

func Warning(msg string) {
	Log(l_Warning, msg)
}

func Info(msg string) {
	Log(l_Info, msg)
}

func Debug(msg string) {
	Log(l_Debug, msg)
}

func DDebug(msg string) {
	Log(l_DDebug, msg)
}
