package log

import (
	"bytes"
	"strings"
	"testing"
)

func TestDebug(t *testing.T) {
	Writer = &bytes.Buffer{}
	Debug("LvlDebug test")
	Debugf("%s", "LvlDebug format test")
	buffer, _ := Writer.(*bytes.Buffer)
	data := buffer.String()
	if !strings.Contains(data, " - DEBUG - (enman/internal/log.TestDebug): Debug test") {
		t.Error("Debug logging failed")
	}
	if !strings.Contains(data, " - DEBUG - (enman/internal/log.TestDebug): Debug format test") {
		t.Error("exDebug format logging failed")
	}
}

func TestInfo(t *testing.T) {
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
