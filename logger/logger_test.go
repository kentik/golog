package logger

import (
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
