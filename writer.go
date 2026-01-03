// Copyright 2018 Author: Ruslan Bikchentaev. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package logs

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/pkg/errors"
)

var (
	ignoreFunc = []string{
		"views.RenderHandlerError",
		"views.RenderInternalError",
		"RenderHandlerError",
		"RenderInternalError",
		"ErrorStack",
		"ErrorLogHandler",
		"v1.Catch",
		"runtime.gopanic",
		"runtime.panicindex",
		"runtime.call32",
		"runtime.panicdottypeE",
		"v1.WrapAPIHandler.func1",
		"fasthttp.(*workerPool).workerFunc",
		"apis.(*Apis).Handler",
		"apis.(*Apis).Handler.func1()",
		"apis.(*Apis).Handler-fm",
		"apis.(*Apis).renderError",
	}
	ignoreFiles = []string{
		"asm_amd64",
		"asm_arm64",
		"asm_amd64.s",
		"asm_arm64.s",
		"iface.go",
		"map_fast32.go",
		"panic.go",
		"proc.go",
		"server.go",
		"signal_unix.go",
		"testing.go",
		"workerpool.go",
		"writer.go",
	}
)

type errLogPrint bool

// Fatal - output formated (function and line calls) fatal information
func Fatal(err error, args ...any) {
	pc, _, _, _ := runtime.Caller(2)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	logErr.Printf(
		errLogPrint(true),
		"%s[[FATAL]]%s%s%s %v %v",
		boldcolors[CRITICAL],
		LogEndColor,
		timeLogFormat(),
		changeShortName(runtime.FuncForPC(pc).Name()),
		err,
		args,
	)
	ErrorStack(err, args...)
	os.Exit(1)
}

func changeShortName(file string) (short string) {
	short = file
	for i := len(file) - 1; i > 0; i-- {
		if file[i] == '/' {
			short = file[i+1:]
			break
		}
	}
	return short
}

// DebugLog output formatted (function and line calls) debug information
func DebugLog(args ...any) {
	if *fDebug {
		logDebug.lock.Lock()
		defer logDebug.lock.Unlock()

		pc, _, _, _ := runtime.Caller(logDebug.callDepth - 2)
		logDebug.funcName = changeShortName(runtime.FuncForPC(pc).Name())

		logDebug.Printf(args...)
	}
}

// StatusLog output formatted information for status
func StatusLog(args ...any) {
	if *fStatus {
		logStat.Printf(args...)
	}
}

type stackTracer interface {
	StackTrace() errors.StackTrace
}

func timeLogFormat() string {
	// todo get flags of current log !
	if logErr.Flags()&log.Ltime != 0 {
		hh, mm, ss := time.Now().Clock()
		return fmt.Sprintf("%.2d:%.2d:%.2d ", hh, mm, ss)
	}

	return ""
}

// ErrorLog - output formatted (function and line calls) error information
func ErrorLog(err error, args ...any) {
	logErr.lock.Lock()
	defer logErr.lock.Unlock()

	if err == nil {
		return
	}

	b := &strings.Builder{}
	format, c := getFormatString(args)
	if c > 0 {
		// add format for error
		if c < len(args) {
			argToString(b, []any{err, ",", format})
			args = args[1:]
		} else {
			args[0] = err
		}
	} else {
		argsToString(b, append([]any{err}, args...))
		args = args[:0]
	}

	if logErr.toSentry {
		defer sentry.Flush(2 * time.Second)
		args = append(args, logErr.sentryOrg, string(*(sentry.CaptureException(errors.Wrap(err, "sentry")))))
		if logErr.sentryDsn > "" {
			b.WriteString(" " + logErr.sentryDsn + "/%s/?query=%s")
		} else {
			b.WriteString(" https://sentry.io/organizations/%s/?query=%s")
		}
	}

	ErrFmt, ok := err.(stackTracer)
	if ok {
		errorPrint := errLogPrint(true)
		frames := ErrFmt.StackTrace()
		for _, frame := range frames {
			file := fmt.Sprintf("%s", frame)
			fncName := fmt.Sprintf("%n", frame)
			if !isIgnoreFile(file) && !isIgnoreFunc(fncName) {

				args = append([]any{
					errorPrint,
					logErr.Prefix() + "%s%s:%d: %s() " + b.String(),
					timeLogFormat(),
					file,
					frame,
					fncName,
				},
					args...)

				break
			}
		}
	} else {

		callDepth := 1
		isIgnore := true

		for pc, file, line, ok := runtime.Caller(callDepth); ok && isIgnore; pc, file, line, ok = runtime.Caller(callDepth) {
			logErr.fileName = changeShortName(file)
			logErr.funcName = changeShortName(runtime.FuncForPC(pc).Name())
			logErr.line = line
			// пропускаем рендер ошибок
			isIgnore = isIgnoreFile(logErr.fileName) || isIgnoreFunc(logErr.funcName)
			callDepth++
		}

		logErr.callDepth = callDepth + 1

		args = append([]any{
			logErr.funcName + "() " + b.String(),
		},
			args...)
	}

	logErr.Printf(args...)
}

const prefErrStack = "[[ERR_STACK]]"

// ErrorStack - output formatted (function and line calls) error runtime stack information
func ErrorStack(err error, args ...any) {

	b := &strings.Builder{}

	if format, c := getFormatString(args); c > 0 {
		// add format for error
		if c < len(args) {
			b.WriteString(prefErrStack)
			argToString(b, err)
			b.WriteRune(',')
			b.WriteString(format)
			args = args[1:]
		} else {
			args[0] = err
		}
	} else {
		argsToString(b, err, args)
		args = args[:0]
	}

	b.WriteString("\n")

	ErrFmt, ok := err.(stackTracer)
	if ok {
		frames := ErrFmt.StackTrace()
		for _, frame := range frames[:len(frames)-2] {
			fileName := fmt.Sprintf("%s", frame)
			fncName := fmt.Sprintf("%n", frame)
			if !isIgnoreFile(fileName) && !isIgnoreFunc(fncName) {
				fmt.Fprintf(b, "%s:%d %s %s()\n", fileName, frame, prefErrStack, fncName)
			}
		}
	} else {
		WriteStack(b, stackBeginWith)
	}

	logErr.lock.Lock()
	logErr.Printf(errLogPrint(true), b.String())
	logErr.lock.Unlock()
}

func WriteStack(b *strings.Builder, i int) {
	for pc, file, line, ok := runtime.Caller(i); ok; pc, file, line, ok = runtime.Caller(i) {
		i++
		fileName := changeShortName(file)
		fncName := changeShortName(runtime.FuncForPC(pc).Name())
		// skip errors rendering
		if !isIgnoreFile(fileName) && !isIgnoreFunc(fncName) {
			fmt.Fprintf(b, "%s:%d %s %s()\n", fileName, line, prefErrStack, fncName)
		}
	}
}

func isIgnoreFile(runFile string) bool {
	for _, name := range ignoreFiles {
		if (runFile == name) || (strings.HasPrefix(runFile, name)) {
			return true
		}
	}
	return false
}

func isIgnoreFunc(funcName string) bool {
	for _, name := range ignoreFunc {
		if (funcName == name) || (strings.HasSuffix(funcName, "."+name)) {
			return true
		}
	}

	return false
}

// ErrorLogHandler - output formatted(function and line calls) error information
func ErrorLogHandler(err error, args ...any) {
	ErrorStack(err, args...)
}

func CustomLog(level Level, prefix, fileName string, line int, msg string, logFlags ...FgLogWriter) {
	args := []any{
		errLogPrint(true),
		"%s[[%s]]%s%s%s:%d: %s",
		// LogPutColor,
		boldcolors[level],
		prefix,
		LogEndColor,
		timeLogFormat(),
		fileName,
		line,
		msg,
	}

	for _, logFlag := range logFlags {
		switch logFlag {
		case FgAll:
			logErr.Printf(args...)
			logStat.Printf(args...)
			logDebug.Printf(args...)
		case FgErr:
			logErr.Printf(args...)
		case FgInfo:
			logStat.Printf(args...)
		case FgDebug:
			logDebug.Printf(args...)
		}

	}
}
