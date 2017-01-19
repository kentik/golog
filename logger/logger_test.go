package logger

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"testing"
)

// TestNilLogger tests that you can safely call log methods on a nil logger.
// This is convenient, for example, when you'd like to test code without
// creating and passing in a logger.
func TestNilLogger(t *testing.T) {
	var log *Logger

	log.Debugf("prefix", "Hello %s", "there")
	log.Infof("prefix", "Hello %s", "there")
	log.Warnf("prefix", "Hello %s", "there")
	log.Errorf("prefix", "Hello %s", "there")
	log.Panicf("prefix", "Hello %s", "there")
}

func TestRemoveNewline(t *testing.T) {
	buf := bytes.Buffer{}
	output = &buf
	defer func() {
		output = io.Writer(os.Stdout)
	}()

	log := New(Levels.Debug)
	SetStdOut()

	log.Debugf("", "testing")
	log.Debugf("", "testing\n")
	log.Debugf("", "testing\n\n\n\n")

	Drain()

	if !regexp.MustCompile("^[^\n]*testing\n[^\n]*testing\n[^\n]*testing\n$").Match(buf.Bytes()) {
		t.Error("Expected testing\\n * 3")
	}
}
