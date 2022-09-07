package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

type logEntryStructured struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Prefix  string    `json:"prefix"`
	Message string    `json:"message"`
	Caller  string    `json:"caller"`
	LogName string    `json:"logName"`
}

const KentikLogFmt = "KENTIK_LOG_FMT"

var sendJSON = true

// setSendJSON is called by init. It sets sendJSON to `true`, if environment
// variable KentikLogFmt is unset|empty|"json"|"JSON", `false` otherwise.
func setSendJSON() {
	logFormat := os.Getenv(KentikLogFmt)
	sendJSON = logFormat == "" || strings.ToLower(logFormat) == "json"
}

// asString creates a JSON-structured string entry in the receiver's buffer.
func (lm *logMessage) asJSON() error {
	le := lm.le
	les := logEntryStructured{
		Time:    lm.time,
		Level:   strings.ToLower(le.lvl.String()),
		Prefix:  strings.Trim(le.pre, " "),
		Message: fmt.Sprintf(le.fmt, le.fmtV...),
		Caller:  le.lc.String(),
		LogName: logNameString,
	}
	return json.NewEncoder(lm).Encode(les)
}

// asString creates a non-JSON log string entry in the receiver's buffer.
//
// It encapsulates most of the message formatting that was in queueMsg and some that was in printStd.
func (lm *logMessage) asString() (err error) {
	// for unknown reasons, only printStd pre-pended the time and log name
	if stdhdl != nil {
		_, err = fmt.Fprintf(lm, "%s%s", lm.time.Format(STDOUT_FORMAT), logNameString)
		if err != nil {
			return
		}
	}

	// this is the equivalent formatting that was in queueMsg, excluding the C-string termination
	le := lm.le
	_, err = fmt.Fprintf(lm, "%s%s<%s: %d> ", levelMapFmt[le.lvl], le.pre, le.lc.File, le.lc.Line)
	if err != nil {
		return
	}
	if _, err = fmt.Fprintf(lm, le.fmt, le.fmtV...); err != nil {
		return
	}

	// for unknown reasons, only printStd trimmed new lines
	if stdhdl != nil {
		lm.rightTrimNewLines()
	}
	return
}

// rightTrimNewLines trims off any/all '\n' from the end logMessage's buffer
func (lm *logMessage) rightTrimNewLines() {
	bs := lm.Bytes()
	l := len(bs)
	bsEnd := l - 1 // last index of bs
	var cnt int
	for cnt = 0; cnt < l && bs[bsEnd-cnt] == '\n'; cnt++ {
	}
	if cnt > 0 {
		lm.Truncate(l - cnt)
	}
}
