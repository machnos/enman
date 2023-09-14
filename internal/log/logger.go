package log

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"time"
)

const (
	dateLayout = "2006-01-02T15:04:05.000-0700"
)

type Level uint8

const (
	LvlTrace   Level = 1
	LvlDebug   Level = 2
	LvlInfo    Level = 4
	LvlWarning Level = 6
	LvlError   Level = 8
	LvlFatal   Level = 10
	LvlOff     Level = 12
)

func (l Level) String() string {
	switch l {
	case LvlTrace:
		return "TRACE"
	case LvlDebug:
		return "DEBUG"
	case LvlInfo:
		return "INFO"
	case LvlWarning:
		return "WARNING"
	case LvlError:
		return "ERROR"
	case LvlFatal:
		return "FATAL"
	case LvlOff:
		return "OFF"
	}
	return fmt.Sprintf("%d", l)
}

var ActiveLevel = LvlInfo
var Writer io.Writer = os.Stdout

func TraceEnabled() bool {
	return ActiveLevel <= LvlTrace
}

func Trace(message string) {
	if !TraceEnabled() {
		return
	}
	log(LvlTrace, message)
}

func Tracef(format string, a ...any) {
	if !TraceEnabled() {
		return
	}
	log(LvlTrace, fmt.Sprintf(format, a...))
}

func DebugEnabled() bool {
	return ActiveLevel <= LvlDebug
}

func Debug(message string) {
	if !DebugEnabled() {
		return
	}
	log(LvlDebug, message)
}

func Debugf(format string, a ...any) {
	if !DebugEnabled() {
		return
	}
	log(LvlDebug, fmt.Sprintf(format, a...))
}

func InfoEnabled() bool {
	return ActiveLevel <= LvlInfo
}

func Info(message string) {
	if !InfoEnabled() {
		return
	}
	log(LvlInfo, message)
}

func Infof(format string, a ...any) {
	if !InfoEnabled() {
		return
	}
	log(LvlInfo, fmt.Sprintf(format, a...))
}

func WarningEnabled() bool {
	return ActiveLevel <= LvlWarning
}

func Warning(message string) {
	if !WarningEnabled() {
		return
	}
	log(LvlWarning, message)
}

func Warningf(format string, a ...any) {
	if !WarningEnabled() {
		return
	}
	log(LvlWarning, fmt.Sprintf(format, a...))
}

func ErrorEnabled() bool {
	return ActiveLevel <= LvlError
}

func Error(message string) {
	if !ErrorEnabled() {
		return
	}
	log(LvlError, message)
}

func Errorf(format string, a ...any) {
	if !ErrorEnabled() {
		return
	}
	log(LvlError, fmt.Sprintf(format, a...))
}

func FatalEnabled() bool {
	return ActiveLevel <= LvlFatal
}

func Fatal(message string) {
	if !FatalEnabled() {
		return
	}
	log(LvlFatal, message)
}

func Fatalf(format string, a ...any) {
	if !FatalEnabled() {
		return
	}
	log(LvlFatal, fmt.Sprintf(format, a...))
}

func log(level Level, message string) {
	pc, _, _, ok := runtime.Caller(2)
	details := runtime.FuncForPC(pc)
	caller := "?"
	if ok && details != nil {
		caller = details.Name()
	}
	_, _ = Writer.Write([]byte(fmt.Sprintf("%s - %s - (%s): %s\n", time.Now().Format(dateLayout), level, caller, message)))
}
