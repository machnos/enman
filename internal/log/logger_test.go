package log

import (
	"bytes"
	"strings"
	"testing"
)

func TestTrace(t *testing.T) {
	ActiveLevel = LvlTrace
	Writer = &bytes.Buffer{}
	Trace("Trace test")
	Tracef("%s", "Trace format test")
	buffer, _ := Writer.(*bytes.Buffer)
	data := buffer.String()
	if !strings.Contains(data, " - TRACE - (enman/internal/log.TestTrace): Trace test") {
		t.Error("Trace logging failed")
	}
	if !strings.Contains(data, " - TRACE - (enman/internal/log.TestTrace): Trace format test") {
		t.Error("Trace format logging failed")
	}
}

func TestDebug(t *testing.T) {
	ActiveLevel = LvlDebug
	Writer = &bytes.Buffer{}
	Debug("Debug test")
	Debugf("%s", "Debug format test")
	buffer, _ := Writer.(*bytes.Buffer)
	data := buffer.String()
	if !strings.Contains(data, " - DEBUG - (enman/internal/log.TestDebug): Debug test") {
		t.Error("Debug logging failed")
	}
	if !strings.Contains(data, " - DEBUG - (enman/internal/log.TestDebug): Debug format test") {
		t.Error("Debug format logging failed")
	}
}

func TestInfo(t *testing.T) {
	ActiveLevel = LvlInfo
	Writer = &bytes.Buffer{}
	Info("Info test")
	Infof("%s", "Info format test")
	buffer, _ := Writer.(*bytes.Buffer)
	data := buffer.String()
	if !strings.Contains(data, " - INFO - (enman/internal/log.TestInfo): Info test") {
		t.Error("Info logging failed")
	}
	if !strings.Contains(data, " - INFO - (enman/internal/log.TestInfo): Info format test") {
		t.Error("Info format logging failed")
	}
}

func TestWarning(t *testing.T) {
	ActiveLevel = LvlWarning
	Writer = &bytes.Buffer{}
	Warning("Warning test")
	Warningf("%s", "Warning format test")
	buffer, _ := Writer.(*bytes.Buffer)
	data := buffer.String()
	if !strings.Contains(data, " - WARNING - (enman/internal/log.TestWarning): Warning test") {
		t.Error("Warning logging failed")
	}
	if !strings.Contains(data, " - WARNING - (enman/internal/log.TestWarning): Warning format test") {
		t.Error("Warning format logging failed")
	}
}

func TestError(t *testing.T) {
	ActiveLevel = LvlError
	Writer = &bytes.Buffer{}
	Error("Error test")
	Errorf("%s", "Error format test")
	buffer, _ := Writer.(*bytes.Buffer)
	data := buffer.String()
	if !strings.Contains(data, " - ERROR - (enman/internal/log.TestError): Error test") {
		t.Error("Error logging failed")
	}
	if !strings.Contains(data, " - ERROR - (enman/internal/log.TestError): Error format test") {
		t.Error("Error format logging failed")
	}
}

func TestFatal(t *testing.T) {
	ActiveLevel = LvlFatal
	Writer = &bytes.Buffer{}
	Fatal("Fatal test")
	Fatalf("%s", "Fatal format test")
	buffer, _ := Writer.(*bytes.Buffer)
	data := buffer.String()
	if !strings.Contains(data, " - FATAL - (enman/internal/log.TestFatal): Fatal test") {
		t.Error("Fatal logging failed")
	}
	if !strings.Contains(data, " - FATAL - (enman/internal/log.TestFatal): Fatal format test") {
		t.Error("Fatal format logging failed")
	}
}
