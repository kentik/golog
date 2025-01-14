package logger

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"runtime"
	"strings"
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
	SetStdOut()
	prefix := "[TestSetTee]"
	message := fmt.Sprintf("log with random string: %s", randString(64))

	teeCh := make(chan string, 5)
	done := make(chan bool)
	go func() {
		for teed := range teeCh {
			if !strings.Contains(teed, message) {
				t.Error("expected #{message} but got #{teed}")
			}
			done <- true
		}
	}()
	SetTee(teeCh)
	log := New(Levels.Debug)
	log.Infof(prefix, message)

	_ = <-done
	Drain()
	close(done)
	close(teeCh)
	logTee = nil
}

func TestLogNoTee(t *testing.T) {
	SetStdOut()
	prefix := "[TestLogNoTee]"
	teedMsg := fmt.Sprintf("teed: %s", randString(64))
	unTeedMsg := fmt.Sprintf("unteed: %s", randString(64))

	teeCh := make(chan string, 5)
	done := make(chan bool)
	teeBuf := bytes.Buffer{}
	go func() {
		for teed := range teeCh {
			teeBuf.WriteString(teed)
			done <- true
		}
	}()
	SetTee(teeCh)

	LogNoTee(Levels.Error, prefix, unTeedMsg)
	log := New(Levels.Debug)
	log.Infof(prefix, teedMsg)

	<-done
	Drain()
	close(done)
	close(teeCh)
	logTee = nil

	// ensure only teed messages was teed
	teeLogs := teeBuf.String()
	if !strings.Contains(teeLogs, teedMsg) {
		t.Error("teed message not in teed logs")
	}
	if strings.Contains(teeLogs, unTeedMsg) {
		t.Error("un-teed message found in teed logs")
	}
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
