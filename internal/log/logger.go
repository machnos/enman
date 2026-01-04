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
var packageLevels = make(map[string]Level)    // Hierarchical package-specific log levels
var callerLevelCache = make(map[string]Level) // Cache of resolved log levels per caller

// SetPackageLevel sets the log level for a specific package
func SetPackageLevel(packageName string, level Level) {
	packageLevels[packageName] = level
	// Clear cache when package levels change
	callerLevelCache = make(map[string]Level)
}

// SetPackageLevels sets multiple package log levels at once
func SetPackageLevels(levels map[string]Level) {
	packageLevels = levels
	// Clear cache when package levels change
	callerLevelCache = make(map[string]Level)
}

// getEffectiveLevel returns the appropriate log level for a caller
// It checks for package-specific levels first, then falls back to root level
// Results are cached to avoid expensive string parsing on repeated calls
func getEffectiveLevel(caller string) Level {
	// Check cache first
	if level, exists := callerLevelCache[caller]; exists {
		return level
	}
	// Check for exact package matches and parent package matches
	// e.g., for "enman/internal/price_importers/entsoe.(*PriceImporter).ImportPrices"
	// we check: "enman/internal/price_importers/entsoe", "enman/internal/price_importers", etc.

	for len(caller) > 0 {
		// Extract package path by removing function/method names
		lastSlash := -1
		for i := len(caller) - 1; i >= 0; i-- {
			if caller[i] == '/' {
				lastSlash = i
				break
			}
			// Stop at function/method separators
			if caller[i] == '.' || caller[i] == '(' {
				break
			}
		}

		if lastSlash == -1 {
			break
		}

		// Try to find a match for this package level
		packagePath := caller[:lastSlash]
		if level, exists := packageLevels[packagePath]; exists {
			// Cache the result before returning
			callerLevelCache[caller] = level
			return level
		}

		// Move to parent package
		lastSlash = -1
		for i := len(packagePath) - 1; i >= 0; i-- {
			if packagePath[i] == '/' {
				lastSlash = i
				break
			}
		}

		if lastSlash == -1 {
			break
		}
		caller = packagePath[:lastSlash]
	}

	// Fall back to root level if no package-specific level found
	result := ActiveLevel

	// Cache the result
	callerLevelCache[caller] = result
	return result
}

func TraceEnabled() bool {
	pc, _, _, ok := runtime.Caller(1)
	details := runtime.FuncForPC(pc)
	caller := "?"
	if ok && details != nil {
		caller = details.Name()
	}
	return getEffectiveLevel(caller) <= LvlTrace
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
	pc, _, _, ok := runtime.Caller(1)
	details := runtime.FuncForPC(pc)
	caller := "?"
	if ok && details != nil {
		caller = details.Name()
	}
	return getEffectiveLevel(caller) <= LvlDebug
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
	pc, _, _, ok := runtime.Caller(1)
	details := runtime.FuncForPC(pc)
	caller := "?"
	if ok && details != nil {
		caller = details.Name()
	}
	return getEffectiveLevel(caller) <= LvlInfo
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
	pc, _, _, ok := runtime.Caller(1)
	details := runtime.FuncForPC(pc)
	caller := "?"
	if ok && details != nil {
		caller = details.Name()
	}
	return getEffectiveLevel(caller) <= LvlWarning
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
	pc, _, _, ok := runtime.Caller(1)
	details := runtime.FuncForPC(pc)
	caller := "?"
	if ok && details != nil {
		caller = details.Name()
	}
	return getEffectiveLevel(caller) <= LvlError
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
	pc, _, _, ok := runtime.Caller(1)
	details := runtime.FuncForPC(pc)
	caller := "?"
	if ok && details != nil {
		caller = details.Name()
	}
	return getEffectiveLevel(caller) <= LvlFatal
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

	// Check if this log level should be logged based on package-specific or root level
	effectiveLevel := getEffectiveLevel(caller)
	if level < effectiveLevel {
		return
	}

	_, _ = Writer.Write([]byte(fmt.Sprintf("%s - %s - (%s): %s\n", time.Now().Format(dateLayout), level, caller, message)))
}
