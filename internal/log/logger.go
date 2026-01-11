package log

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
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

var (
	ActiveLevel                   = LvlInfo
	Writer           io.Writer    = os.Stdout
	packageLevels                 = make(map[string]Level) // Hierarchical package-specific log levels
	callerLevelCache              = make(map[string]Level) // Cache of resolved log levels per caller
	logMutex         sync.RWMutex                          // Protects concurrent access to log state
)

// SetActiveLevel sets the root log level for all packages
func SetActiveLevel(level Level) {
	logMutex.Lock()
	defer logMutex.Unlock()
	ActiveLevel = level
	// Invalidate cache when root level changes
	callerLevelCache = make(map[string]Level)
}

// SetPackageLevel sets the log level for a specific package
func SetPackageLevel(packageName string, level Level) {
	logMutex.Lock()
	defer logMutex.Unlock()
	packageLevels[packageName] = level
	// Clear cache when package levels change
	callerLevelCache = make(map[string]Level)
}

// SetPackageLevels sets multiple package log levels at once
func SetPackageLevels(levels map[string]Level) {
	logMutex.Lock()
	defer logMutex.Unlock()
	packageLevels = levels
	// Clear cache when package levels change
	callerLevelCache = make(map[string]Level)
}

// SetWriter changes the output writer for log messages
func SetWriter(w io.Writer) {
	logMutex.Lock()
	defer logMutex.Unlock()
	if w != nil {
		Writer = w
	}
}

// getCaller extracts the function/method name from the call stack
// depth: number of frames to skip (2 for direct calls, 3+ for wrapper functions)
func getCaller(depth int) string {
	pc, _, _, ok := runtime.Caller(depth)
	if !ok {
		return "?"
	}
	details := runtime.FuncForPC(pc)
	if details == nil {
		return "?"
	}
	return details.Name()
}

// getEffectiveLevel returns the appropriate log level for a caller
// It checks for package-specific levels first, then falls back to root level
// Results are cached to avoid expensive string parsing on repeated calls
func getEffectiveLevel(caller string) Level {
	// Fast path: check cache without upgrading locks
	logMutex.RLock()
	if level, exists := callerLevelCache[caller]; exists {
		logMutex.RUnlock()
		return level
	}

	// Take a snapshot of packageLevels and ActiveLevel while holding read lock
	packageLevelsCopy := make(map[string]Level)
	for k, v := range packageLevels {
		packageLevelsCopy[k] = v
	}
	activeLevel := ActiveLevel
	logMutex.RUnlock()

	// Compute the effective level outside the lock
	level := computeEffectiveLevel(caller, packageLevelsCopy, activeLevel)

	// Cache the result with a brief write lock
	logMutex.Lock()
	callerLevelCache[caller] = level
	logMutex.Unlock()

	return level
}

// computeEffectiveLevel determines the log level for a caller based on package-specific
// and root level settings. This function is lock-free and can be called outside the critical section.
//
// For a caller like "enman/internal/domain.(*OptimalChargingPeriodCalculator).updateChargingPeriods", it checks:
// 1. "enman/internal/domain.(*OptimalChargingPeriodCalculator).updateChargingPeriods" (exact match)
// 2. "enman/internal/domain.(*OptimalChargingPeriodCalculator)" (receiver type)
// 3. "enman/internal/domain" (full package)
// 4. "enman/internal" (parent package)
// 5. "enman" (root package)
// Finally falls back to activeLevel if no match found
func computeEffectiveLevel(caller string, packageLevels map[string]Level, activeLevel Level) Level {
	// Check exact match first
	if level, exists := packageLevels[caller]; exists {
		return level
	}

	// Find the package path by locating the first dot that separates package from type/function
	// e.g., "enman/internal/domain.(*OptimalChargingPeriodCalculator).updateChargingPeriods" -> find first "."
	dotIndex := -1
	for i := 0; i < len(caller); i++ {
		if caller[i] == '.' {
			dotIndex = i
			break
		}
	}

	// If no dot found, the entire caller is likely just a function name, fallback to root level
	if dotIndex == -1 {
		return activeLevel
	}

	// Extract the package path (everything before the first dot)
	fullPackagePath := caller[:dotIndex]

	// Check for receiver type (everything up to and including the closing parenthesis)
	// e.g., "enman/internal/domain.(*OptimalChargingPeriodCalculator)" from the full caller
	// Look for the last closing parenthesis after the first dot
	lastCloseParen := -1
	for i := dotIndex; i < len(caller); i++ {
		if caller[i] == ')' {
			lastCloseParen = i
			break
		}
	}

	// If we found a closing parenthesis, check for receiver type match
	if lastCloseParen > dotIndex {
		receiverType := caller[:lastCloseParen+1]
		if level, exists := packageLevels[receiverType]; exists {
			return level
		}
	}

	// Try to match progressively shorter package paths
	// e.g., "enman/internal/domain" -> "enman/internal" -> "enman"
	currentPath := fullPackagePath
	for len(currentPath) > 0 {
		// Check if this package path exists in packageLevels
		if level, exists := packageLevels[currentPath]; exists {
			return level
		}

		// Move to parent package by finding the last slash
		lastSlash := -1
		for i := len(currentPath) - 1; i >= 0; i-- {
			if currentPath[i] == '/' {
				lastSlash = i
				break
			}
		}

		// If no slash found, we've reached the root package level
		if lastSlash == -1 {
			break
		}

		// Move up one level in the package hierarchy
		currentPath = currentPath[:lastSlash]
	}

	// Fall back to root level if no package-specific level found
	return activeLevel
}

func TraceEnabled() bool {
	caller := getCaller(2)
	return getEffectiveLevel(caller) <= LvlTrace
}

func Trace(message string) {
	caller := getCaller(2)
	if getEffectiveLevel(caller) > LvlTrace {
		return
	}
	logWithCaller(LvlTrace, caller, message)
}

func Tracef(format string, a ...any) {
	caller := getCaller(2)
	if getEffectiveLevel(caller) > LvlTrace {
		return
	}
	logWithCaller(LvlTrace, caller, fmt.Sprintf(format, a...))
}

func DebugEnabled() bool {
	caller := getCaller(2)
	return getEffectiveLevel(caller) <= LvlDebug
}

func Debug(message string) {
	caller := getCaller(2)
	if getEffectiveLevel(caller) > LvlDebug {
		return
	}
	logWithCaller(LvlDebug, caller, message)
}

func Debugf(format string, a ...any) {
	caller := getCaller(2)
	if getEffectiveLevel(caller) > LvlDebug {
		return
	}
	logWithCaller(LvlDebug, caller, fmt.Sprintf(format, a...))
}

func InfoEnabled() bool {
	caller := getCaller(2)
	return getEffectiveLevel(caller) <= LvlInfo
}

func Info(message string) {
	caller := getCaller(2)
	if getEffectiveLevel(caller) > LvlInfo {
		return
	}
	logWithCaller(LvlInfo, caller, message)
}

func Infof(format string, a ...any) {
	caller := getCaller(2)
	if getEffectiveLevel(caller) > LvlInfo {
		return
	}
	logWithCaller(LvlInfo, caller, fmt.Sprintf(format, a...))
}

func WarningEnabled() bool {
	caller := getCaller(2)
	return getEffectiveLevel(caller) <= LvlWarning
}

func Warning(message string) {
	caller := getCaller(2)
	if getEffectiveLevel(caller) > LvlWarning {
		return
	}
	logWithCaller(LvlWarning, caller, message)
}

func Warningf(format string, a ...any) {
	caller := getCaller(2)
	if getEffectiveLevel(caller) > LvlWarning {
		return
	}
	logWithCaller(LvlWarning, caller, fmt.Sprintf(format, a...))
}

func ErrorEnabled() bool {
	caller := getCaller(2)
	return getEffectiveLevel(caller) <= LvlError
}

func Error(message string) {
	caller := getCaller(2)
	if getEffectiveLevel(caller) > LvlError {
		return
	}
	logWithCaller(LvlError, caller, message)
}

func Errorf(format string, a ...any) {
	caller := getCaller(2)
	if getEffectiveLevel(caller) > LvlError {
		return
	}
	logWithCaller(LvlError, caller, fmt.Sprintf(format, a...))
}

func FatalEnabled() bool {
	caller := getCaller(2)
	return getEffectiveLevel(caller) <= LvlFatal
}

func Fatal(message string) {
	caller := getCaller(2)
	if getEffectiveLevel(caller) > LvlFatal {
		return
	}
	logWithCaller(LvlFatal, caller, message)
}

func Fatalf(format string, a ...any) {
	caller := getCaller(2)
	if getEffectiveLevel(caller) > LvlFatal {
		return
	}
	logWithCaller(LvlFatal, caller, fmt.Sprintf(format, a...))
}

// logWithCaller writes a log message with the caller information
// Thread-safe with mutex protection and fallback to stderr on write errors
func logWithCaller(level Level, caller string, message string) {
	logEntry := fmt.Sprintf("%s - %s - (%s): %s\n", time.Now().Format(dateLayout), level, caller, message)

	logMutex.Lock()
	writer := Writer
	logMutex.Unlock()

	// Attempt to write to the configured writer
	if _, err := writer.Write([]byte(logEntry)); err != nil {
		// Fallback to stderr if primary writer fails
		_, _ = os.Stderr.WriteString(fmt.Sprintf("[LOG ERROR] Failed to write to primary logger: %v\n", err))
		_, _ = os.Stderr.WriteString(logEntry)
	}
}
