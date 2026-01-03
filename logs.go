// Copyright 2018 Author: Ruslan Bikchentaev. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package logs output logs and advanced debug information
package logs

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/pkg/errors"
)

var (
	fDebug   = flag.Bool("debug", false, "debug mode")
	fStatus  = flag.Bool("status", true, "status mode")
	logErr   = NewWrapKitLogger(colors[ERROR]+"ERROR"+LogEndColor, 1)
	logStat  = NewWrapKitLogger("INFO", 3)
	logDebug = NewWrapKitLogger(colors[DEBUG]+"DEBUG"+LogEndColor, 3)
)

// LogsType - interface for print logs record
type LogsType interface {
	PrintToLogs(*strings.Builder) string
}

type wrapKitLogger struct {
	*log.Logger
	callDepth int
	line      int
	fileName  string
	funcName  string
	typeLog   string
	toSentry  bool
	sentryDsn string
	sentryOrg string
	toOther   io.Writer
	lock      sync.RWMutex
}

const logFlags = log.Lshortfile | log.Ltime

var stackBeginWith = 1

func NewWrapKitLogger(pref string, depth int) *wrapKitLogger {
	return &wrapKitLogger{
		Logger:    log.New(os.Stdout, "[["+pref+"]]", logFlags),
		typeLog:   pref,
		callDepth: depth,
		toOther:   &MultiWriter{lock: sync.RWMutex{}},
	}
}

// SetDebug set debug level for log, return old value
func SetDebug(d bool) bool {
	old := *fDebug
	*fDebug = d

	return old
}

// SetStatus set status level for log, return old value
func SetStatus(s bool) bool {
	old := *fStatus
	*fStatus = s

	return old
}

type FgLogWriter int8

const (
	FgAll FgLogWriter = iota
	FgErr
	FgInfo
	FgDebug
)

func (logger *wrapKitLogger) addWriter(newWriters ...io.Writer) {
	logger.toOther.(*MultiWriter).Append(newWriters...)
}

func (logger *wrapKitLogger) deleteWriter(writersToDelete ...io.Writer) {
	logger.toOther.(*MultiWriter).Remove(writersToDelete...)
}

// SetWriters for logs
func SetWriters(newWriter io.Writer, logFlags ...FgLogWriter) {
	// todo: можно поменять местами аргументы и дать возможность добавлять неограниченное количество врайтеров

	for _, logFlag := range logFlags {
		switch logFlag {
		case FgAll:
			logErr.addWriter(newWriter)
			logStat.addWriter(newWriter)
			logDebug.addWriter(newWriter)
		case FgErr:
			logErr.addWriter(newWriter)
		case FgInfo:
			logStat.addWriter(newWriter)
		case FgDebug:
			logDebug.addWriter(newWriter)
		}
	}

}

// DeleteWriters deletes mentioned writer from writers for mentioned logFlag
func DeleteWriters(writerToDelete io.Writer, logFlags ...FgLogWriter) {

	for _, logFlag := range logFlags {
		switch logFlag {
		case FgAll:
			logErr.deleteWriter(writerToDelete)
			logStat.deleteWriter(writerToDelete)
			logDebug.deleteWriter(writerToDelete)
		case FgErr:
			logErr.deleteWriter(writerToDelete)
		case FgInfo:
			logStat.deleteWriter(writerToDelete)
		case FgDebug:
			logDebug.deleteWriter(writerToDelete)
		}
	}
}

// SetSentry set SetSentry output for error
func SetSentry(dsn string, org string) error {
	err := sentry.Init(sentry.ClientOptions{Dsn: dsn})
	if err != nil {
		return errors.Wrap(err, "sentry.Init")
	}

	logErr.toSentry = true
	logErr.sentryOrg = org
	if dsn > "" {
		logErr.sentryDsn = dsn
	}

	return nil
}

// SetLogFlags set logger flags & return old flags
func SetLogFlags(f int) int {

	flags := logErr.Flags()

	logErr.SetFlags(f)
	logDebug.SetFlags(f)
	logStat.SetFlags(f)

	return flags
}

// SetStackBeginWith set stackBeginWith level for log, return old value
func SetStackBeginWith(s int) int {
	old := stackBeginWith
	stackBeginWith = s

	return old
}

type logMess struct {
	Message string    `json:"message"`
	Now     time.Time `json:"@timestamp"`
	Level   string    `json:"level"`
}

func NewlogMess(mess string, logger *wrapKitLogger) *logMess {
	return &logMess{mess, time.Now(), logger.typeLog}
}

func (logger *wrapKitLogger) Printf(vars ...any) {

	checkPrint, checkType := vars[0].(errLogPrint)

	if checkType == true {
		vars = vars[1:]
	}

	b := &strings.Builder{}
	getArgsString(b, vars...)
	if checkType && bool(checkPrint) {
		fmt.Println(b.String())
	} else {
		_ = logger.Output(logger.callDepth, b.String())
	}

	if logger.toOther != nil {
		b := bytes.NewBuffer(nil)
		if checkType && bool(checkPrint) {
			b.WriteString(b.String())
		} else {
			fmt.Fprintf(b, "%s%s:%d %s",
				timeLogFormat(),
				logger.fileName,
				logger.line,
				b.String())
		}

		go func() {
			defer func() {
				if err := recover(); err != nil {
					_ = logger.Output(logger.callDepth, fmt.Sprintf("recover: %v,", err))
				}
			}()
			_, err := logger.toOther.Write(b.Bytes())
			if err != nil {
				_ = logger.Output(logger.callDepth, fmt.Sprintf("Write toOther: %v,", err))
			}
		}()
	}
}

func getArgsString(b *strings.Builder, args ...any) {

	// if first param is formatting string
	if format, c := getFormatString(args); c > 0 {
		fmt.Fprintf(b, format, args[1:]...)
	} else {
		argsToString(b, args...)
	}
}

func argsToString(b *strings.Builder, args ...any) {
	for i, arg := range args {
		if i > 0 {
			b.WriteRune(',')
		}
		argToString(b, arg)
	}
}

func argToString(b *strings.Builder, arg any) {
	switch val := arg.(type) {
	case nil:
		b.WriteString(" is nil")
	case string:
		b.WriteString(strings.TrimPrefix(val, "ERROR:"))
	case []string:
		b.WriteString(strings.Join(val, "\n"))
	case LogsType:
		val.PrintToLogs(b)
	case time.Time:
		b.WriteString(val.Format("Mon Jan 2 15:04:05 -0700 MST 2006"))
	case []any:
		switch len(val) {
		case 0:
			b.WriteString("")
		case 1:
			argToString(b, val[0])
		default:
			getArgsString(b, val...)
		}

	case error:
		b.WriteString(strings.TrimPrefix(val.Error(), "ERROR:"))
	default:
		fmt.Fprintf(b, "%#v", arg)
	}
}

func getFormatString(args []any) (string, int) {
	if len(args) < 2 {
		return "", 0
	}

	if format, ok := args[0].(string); ok && strings.Index(format, "%") > -1 {
		c := strings.Count(format, "%")
		if c < len(args)-1 {
			format += strings.Repeat(", %v", len(args)-c-1)
		}

		return format, c
	}

	return "", 0
}
