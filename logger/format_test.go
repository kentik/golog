package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	tTime    = "2022-08-25T00:55:13.578 "
	tLogName = "./kproxy [2189:versa_br2_our1:404055] [2510919]"
)

var (
	level = Levels.Info
	le    = logEntry{
		lvl:  level,
		pre:  "[CHF] ",
		fmt:  "%q -> versa device",
		fmtV: []interface{}{"versa appliance"},
		lc: logCaller{
			File: "/_/pkg/client/snmp/metrics/device_metrics.go",
			Line: 115,
		},
	}
	msg   = fmt.Sprintf(le.fmt, le.fmtV...)
	tm, _ = time.Parse(STDOUT_FORMAT, tTime)
)

func Test_asString(t *testing.T) {
	stdhdl = io.Writer(os.Stdout)
	_ = SetLogName(tLogName)
	// defer reset

	lm := &logMessage{le: le, time: tm}

	err := lm.asString()
	if err != nil {
		t.Errorf("err: %v", err)
	}

	levelStr := string(levelMapFmt[level])
	caller := fmt.Sprintf("<%s: %d> ", le.lc.File, le.lc.Line)
	logStr := tTime + tLogName + levelStr + le.pre + caller + msg
	if logStr != lm.String() {
		t.Errorf("%s != %s", logStr, lm.String())
	}
}

func Test_asJSON(t *testing.T) {
	if err := SetLogName(tLogName); err != nil {
		t.Errorf("err: %v", err)
	}

	lm := &logMessage{le: le, time: tm}
	if err := lm.asJSON(); err != nil {
		t.Errorf("err: %v", err)
	}

	actual := &logEntryStructured{}
	_ = json.NewDecoder(lm).Decode(actual)
	expected := logEntryStructured{
		Time:    tm,
		Level:   "info",
		Prefix:  "[CHF]",
		Message: msg,
		Caller:  le.lc.String(),
		LogName: tLogName,
	}
	if expected != *actual {
		t.Errorf("expected:%v != actual:%v", expected, *actual)
	}
}
func Test_rightTrimNewLines(t *testing.T) {
	msg := randString(100)
	longMsg := randString(1000)
	tests := []struct {
		name string
		msg  string
	}{
		{name: "trim 0", msg: msg},
		{name: "trim 1", msg: msg + "\n"},
		{name: "trim 3", msg: msg + "\n\n\n"},
		{name: "with leader trim 0", msg: "\n\n\n" + msg},
		{name: "with leader trim 1", msg: "\n\n" + msg + "\n"},
		{name: "with leader trim 2", msg: "\n" + msg + "\n\n"},
		{name: "empty trim 0", msg: ""},
		{name: "empty trim 1", msg: "\n"},
		{name: "empty trim n", msg: "\n\n\n\n\n\n\n\n\n"},
		{name: "long trim 0", msg: longMsg},
		{name: "long trim 1", msg: longMsg + "\n"},
		{name: "long trim 4", msg: longMsg + "\n\n\n\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lm := &logMessage{Buffer: bytes.Buffer{}}
			if _, err := fmt.Fprint(lm, tt.msg); err != nil {
				t.Errorf("%q: Fprintf returned %v", tt.name, err)
			}
			lm.rightTrimNewLines()
			trimmed := strings.TrimRight(tt.msg, "\n")
			message := string(lm.Bytes())
			if trimmed != message {
				t.Errorf("%q: %s != %s", tt.name, trimmed, message)
			}
		})
	}
}

func Test_setSendJSON(t *testing.T) {
	tests := []struct {
		name      string
		envVarVal string
		sendJSON  bool
	}{
		{name: "empty", envVarVal: "", sendJSON: true},
		{name: "json", envVarVal: "json", sendJSON: true},
		{name: "jSoN", envVarVal: "jSoN", sendJSON: true},
		{name: "JSON", envVarVal: "JSON", sendJSON: true},
		{name: "!json", envVarVal: "!json", sendJSON: false},
		{name: "string", envVarVal: "string", sendJSON: false},
		{name: "random", envVarVal: randString(4), sendJSON: false},
		{name: "json prefix", envVarVal: "json" + randString(4), sendJSON: false},
		{name: "JsOn inside", envVarVal: randString(4) + "JsOn" + randString(4), sendJSON: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Setenv(KentikLogFmt, tt.envVarVal)
			setSendJSON()
			if sendJSON != tt.sendJSON {
				t.Errorf("%s: expected:%v, actual:%v", tt.name, tt.sendJSON, sendJSON)
			}
		})
	}

	// last test: environment variable unset
	_ = os.Unsetenv(KentikLogFmt)
	setSendJSON()
	if !sendJSON {
		t.Errorf("unset %s: %v", KentikLogFmt, sendJSON)
	}
}

func Benchmark_asJSON(b *testing.B) {
	for i := 0; i < b.N; i++ {
		lm := &logMessage{le: le, time: tm}
		_ = lm.asJSON()
	}
}

func Benchmark_asString(b *testing.B) {
	stdhdl = io.Writer(os.Stdout)
	for i := 0; i < b.N; i++ {
		lm := &logMessage{le: le, time: tm}
		_ = lm.asString()
	}
}
