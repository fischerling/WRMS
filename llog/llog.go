package llog

import (
	"fmt"
	"log"
		   "runtime/debug"
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
			Level = LogLevel(i + 1)
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

func Log(l LogLevel, format string, a ...any) {
	if l > Level {
		return
	}

	msg := fmt.Sprintf(format, a...)
	msg = fmt.Sprintf("[%s] %s", l.String(), msg)

	if l == l_Fatal {
		   debug.PrintStack()
		log.Fatalln(msg)
	} else {
		log.Println(msg)
	}
}

func Fatal(format string, a ...any) {
	Log(l_Fatal, format, a...)
}

func Error(format string, a ...any) {
	Log(l_Error, format, a...)
}

func Warning(format string, a ...any) {
	Log(l_Warning, format, a...)
}

func Info(format string, a ...any) {
	Log(l_Info, format, a...)
}

func Debug(format string, a ...any) {
	Log(l_Debug, format, a...)
}

func DDebug(format string, a ...any) {
	Log(l_DDebug, format, a...)
}
