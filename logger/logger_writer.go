package logger

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"
)

// The csyslog function is necessary here because cgo does not appear
// to be able to call a variadic function directly and syslog has the
// same signature as printf.

// #include <stdlib.h>
// #include <syslog.h>
// void csyslog(int p, const char *m) {
//     syslog(p, "%s", m);
// }
import "C"

const (
	NumMessages   = 10 * 1024 // number of allowed log messages
	STDOUT_FORMAT = "2006-01-02T15:04:05.000 "
)

// container for a pending log message
type logMessage struct {
	bytes.Buffer
	level C.int
	time  time.Time
}

var (
	ErrLogFullBuf           = errors.New("Log message queue is full")
	ErrFreeMessageOverflow  = errors.New("Too many free messages. Overflow of fixed	set.")
	ErrFreeMessageUnderflow = errors.New("Too few free messages. Underflow of fixed	set.")

	// the logName object for syslog to use
	logName       *C.char
	logNameString string

	// the message queue of pending or free messages
	// since only one can be full at a time, the total size will be about 10MB
	messages     chan *logMessage
	freeMessages chan *logMessage

	// mapping of our levels to syslog values
	levelSysLog = map[Level]C.int{
		Levels.Access: C.LOG_INFO,
		Levels.Off:    C.LOG_DEBUG,
		Levels.Panic:  C.LOG_ERR,
		Levels.Error:  C.LOG_ERR,
		Levels.Warn:   C.LOG_WARNING,
		Levels.Info:   C.LOG_INFO,
		Levels.Debug:  C.LOG_DEBUG,
	}

	// mirror of levelMap used to avoid making a new string with '[]' on every log
	// call
	levelMapFmt = map[Level][]byte{
		Levels.Access: []byte("[Access] "),
		Levels.Off:    []byte("[Off] "),
		Levels.Panic:  []byte("[Panic] "),
		Levels.Error:  []byte("[Error] "),
		Levels.Warn:   []byte("[Warn] "),
		Levels.Info:   []byte("[Info] "),
		Levels.Debug:  []byte("[Debug] "),
	}

	customSock net.Conn = nil

	logWriterFinished chan struct{}

	stdhdl io.Writer

	logTee chan string
)

// When called, this will switch over to writting log messages to the defined socket.
func SetCustomSocket(address, network string) (err error) {
	customSock, err = net.Dial(network, address)

	return err
}

func SetStdOut() {
	stdhdl = io.Writer(os.Stdout)
}

func SetStdErr() {
	stdhdl = io.Writer(os.Stderr)
}

func SetTee(tee chan string) {
	logTee = tee
}

// SetLogName sets the indentifier used by syslog for this program
func SetLogName(p string) (err error) {

	logNameString = p
	if stdhdl != nil {
		return
	}

	if logName != nil {
		C.free(unsafe.Pointer(logName))
	}
	logName = C.CString(p)
	_, err = C.openlog(logName, C.LOG_NDELAY|C.LOG_NOWAIT|C.LOG_PID, C.LOG_USER)
	if err != nil {
		atomic.AddUint64(&errCount, 1)
	}

	return err
}

// freeMsg releases the message back to be reused
func freeMsg(msg *logMessage) (err error) {
	msg.Reset()
	select {
	case freeMessages <- msg: // no-op
	default:
		atomic.AddUint64(&errCount, 1)
		return ErrFreeMessageOverflow
	}

	return
}

// queueMsg adds a message to the pending messages channel. It will drop the
// message and return an error if the channel is full.
func queueMsg(lvl Level, prefix, format string, v ...interface{}) (err error) {
	atomic.AddUint64(&logCount, 1)

	var msg *logMessage

	// get a message if possible
	select {
	case msg = <-freeMessages: // got a message-struct; proceed
	default:
		// no messages left, drop
		atomic.AddUint64(&dropCount, 1)
		return
	}

	msg.time = time.Now()

	// render the message: level prefix, message body, C null terminator
	msg.level = levelSysLog[lvl]
	_, file, line, _ := runtime.Caller(4)
	for _, s := range []string{
		// Most to least specific
		"vendor/github.com/kentik/",
		"vendor/github.com/",
		"vendor/",
		"build/input/",
	} {
		idx := strings.Index(file, s)
		if idx >= 0 {
			file = file[idx+len(s):]
			break
		}
	}
	if _, err = msg.Write(levelMapFmt[lvl]); err != nil {
		atomic.AddUint64(&errCount, 1)
		freeMsg(msg)
		return
	}
	if _, err = fmt.Fprintf(msg, "%s", prefix); err != nil {
		atomic.AddUint64(&errCount, 1)
		freeMsg(msg)
		return
	}
	fmt.Fprintf(msg, "<%s: %d> ", file, line)
	if _, err = fmt.Fprintf(msg, format, v...); err != nil {
		atomic.AddUint64(&errCount, 1)
		freeMsg(msg)
		return
	}
	if err = msg.WriteByte(0); err != nil {
		atomic.AddUint64(&errCount, 1)
		freeMsg(msg)
		return
	}

	// queue the message
	select {
	case messages <- msg:
		// no-op
	default:
		// this should never happen since there is an exact number of messages
		atomic.AddUint64(&errCount, 1)
		return ErrLogFullBuf
	}

	return
}

// Send to a tee
func printTee(msg *logMessage) (err error) {
	// remove C null-termination byte
	message := string(msg.Bytes()[:msg.Len()-1])
	message = strings.TrimRight(message, "\n")
	select {
	case logTee <- fmt.Sprintf("%s%s%s", msg.time.Format(STDOUT_FORMAT), logNameString, message):
	default:
		err = fmt.Errorf("Log Tee Full")
	}
	return
}

// Just print mesg to stdout
func printStd(msg *logMessage) (err error) {
	// remove C null-termination byte
	message := string(msg.Bytes()[:msg.Len()-1])
	message = strings.TrimRight(message, "\n")
	fmt.Fprintf(stdhdl, "%s%s%s\n", msg.time.Format(STDOUT_FORMAT), logNameString, message)
	return
}

// write a message to syslog. This is a concrete, blocking event.
func write(msg *logMessage) (err error) {
	start := (*C.char)(unsafe.Pointer(&msg.Bytes()[0]))
	if _, err = C.csyslog(C.LOG_USER|msg.level, start); err != nil {
		atomic.AddUint64(&errCount, 1)
	}
	return
}

// write a message to a pre-defined custom socket. This is a concrete, blocking event.
// Writes out using the syslog rfc5424 format.
func writeCustomSocket(msg *logMessage) (err error) {
	if _, err = customSock.Write(bytes.Join([][]byte{[]byte(fmt.Sprintf("<%d>", C.LOG_USER|msg.level)),
		msg.Bytes()}, []byte(""))); err != nil {
		atomic.AddUint64(&errCount, 1)
	}
	return
}

// logWriter will write out messages to syslog. It may block if something breaks
// within the syslog call.
func logWriter() {
	for msg := range messages {
		if logTee != nil {
			printTee(msg)
		}

		if stdhdl != nil {
			printStd(msg)
		} else if customSock == nil {
			write(msg)
		} else {
			writeCustomSocket(msg)
		}
		freeMsg(msg)
	}
	if customSock != nil {
		customSock.Close()
	}
	close(logWriterFinished)
}

// Close shuts down the logger system. After Close is called, any additional
// logs will panic. Only call this if you are completely done.
func Close(ctx context.Context) error {
	close(messages)
	select {
	case <-logWriterFinished:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// DrainContext blocks until it sees no pending messages or the context is canceled.
// Pending messages may never run out if another goroutine is constantly
// writing.
func DrainContext(ctx context.Context) error {
	for ctx.Err() == nil && len(messages) > 0 {
		innerCtx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
		<-innerCtx.Done() // Wait for 10ms and check len(messages) again
		cancel()
	}
	return ctx.Err()
}

// Drain is like DrainContext, but you didn't want to write context.Background().
// Outside of tests, you want to use DrainContext.
func Drain() {
	_ = DrainContext(context.Background())
}

func setup() {
	stdhdl = nil
	messages = make(chan *logMessage, NumMessages)
	freeMessages = make(chan *logMessage, NumMessages)
	msgArr := make([]logMessage, NumMessages)
	for i := range msgArr {
		if err := freeMsg(&msgArr[i]); err != nil {
			break
		}
	}

	logWriterFinished = make(chan struct{}, 1)
	go logWriter()
}

func init() {
	setup()
}
