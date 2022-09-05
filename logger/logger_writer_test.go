package logger

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReleaseMemory(t *testing.T) {

	// This isn't a functional test, and I don't want it to be a gate on
	// future changes.  But I'd like to leave it in place, because I
	// suspect that we may need to confirm similar behaviors in the future,
	// and I think it may be a useful starting point.
	t.SkipNow()

	// we're going to log a few cycles of short messages, then a few cycles of long messages, then short messages again.
	// Hopefully, we can observe a growing, then shrinking, heap.
	log := New(Levels.Debug)
	stdhdl = io.Discard // throw away every message in the logWriter goroutine before reusing

	// run a bunch of messages through the logging system,
	// returning the size of the heap when they're done.
	messagesPerCycle := 10 * NumMessages
	cycle := func(s string) uint64 {
		for i := 0; i < messagesPerCycle; i++ {
			log.Debugf("", s)
		}

		// let the logWriter quiesce
		for len(freeMessages) < NumMessages {
			time.Sleep(time.Millisecond)
		}

		runtime.GC()
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		return ms.Alloc
	}

	heapAfterShortLogs := cycle("hello")

	// Use 50KB log messages for this.  Worst-case, we end up with
	// 10,000 of them, and use up 500MB of real heap.  That should be
	// plenty for us to detect, without clobbering anybody's dev box.
	heapAfterLongLogs := cycle(randString(51200))

	// accept a 10% fluctuation in the total heap size
	if float64(heapAfterLongLogs) > 1024*1024*100 {
		t.Fatalf("heapAfterLongLogs %d greater than 100MB!", heapAfterLongLogs)
	} else {
		fmt.Printf("heapAfterShortLogs: %d\nheapAfterLongLogs:  %d\nSeems acceptable!\n", heapAfterShortLogs, heapAfterLongLogs)
	}
}

func TestSetTee(t *testing.T) {
	defer cleanUpAndReset()
	prefix := "[TestSetTee]"
	message := fmt.Sprintf("log with random string: %s", randString(64))

	teeCh := make(chan string, 5)
	defer close(teeCh)
	wg := new(sync.WaitGroup)
	go func() {
		for teed := range teeCh {
			if !strings.Contains(teed, message) {
				t.Error("expected #{message} but got #{teed}")
			}
			wg.Done()
		}
	}()
	SetTee(teeCh)
	log := New(Levels.Debug)
	wg.Add(1)
	log.Infof(prefix, message)

	wg.Wait()
}

func TestLogNoTee(t *testing.T) {
	defer cleanUpAndReset()
	prefix := "[TestLogNoTee]"
	teedMsg := fmt.Sprintf("teed: %s", randString(64))
	unTeedMsg := fmt.Sprintf("unteed: %s", randString(64))

	teeCh := make(chan string, 5)
	defer close(teeCh)

	wg := new(sync.WaitGroup)
	go func() {
		for teed := range teeCh {
			// ensure only teed messages was teed
			if !strings.Contains(teed, teedMsg) {
				t.Error("teed message not in teed logs")
			}
			if strings.Contains(teed, unTeedMsg) {
				t.Error("un-teed message found in teed logs")
			}
			wg.Done()
		}
	}()
	SetTee(teeCh)

	LogNoTee(Levels.Error, prefix, unTeedMsg)
	log := New(Levels.Debug)
	wg.Add(1)
	log.Infof(prefix, teedMsg)

	wg.Wait()
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func TestSomething(t *testing.T) {
	defer cleanUpAndReset()

	/*
		le := &logEntry{
			lvl:  Levels.Info,
			pre:  randString(3),
			fmt:  "rain is %s, seven is %d",
			fmtV: []interface{}{"wet", 7},
			lc: logCaller{
				File: "",
				Line: 0,
			},
			tee: false,
		}

		msg := &logMessage{
			Buffer: bytes.Buffer{},
			level:  nil,
			time:   time.Time{},
			le:     logEntry{},
		}
	*/

}

func cleanUpAndReset() {
	Drain()
	stdhdl = nil
	logTee = nil
}

func Test_truncate(t *testing.T) {
	var b bytes.Buffer
	fmt.Fprint(&b, "foobar")
	b.Truncate(0)
	t.Log(b.Len(), b)
}

func Test_trimNewLines(t *testing.T) {
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
			trimNewLines(lm)
			trimmed := strings.TrimRight(tt.msg, "\n")
			message := string(lm.Bytes())
			if trimmed != message {
				t.Errorf("%q: %s != %s", tt.name, trimmed, message)
			}
		})
	}
}
