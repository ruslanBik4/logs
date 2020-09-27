// Copyright 2018 Author: Ruslan Bikchentaev. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package logs

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
)

func TestWrapKitLogger_Printf(t *testing.T) {
	var flag errLogPrint = true

	l := NewWrapKitLogger("[[test]]", 0)

	l.Printf(flag, "test true flag", 0, "\n")

	flag = false

	l.Printf(flag, "test false flag", 0)

	l.Printf("test not flag", 0)
}

type fakeErr struct{}

func (f fakeErr) Error() string {
	return "fake error"
}

func TestErrorLog(t *testing.T) {

	var err fakeErr

	ErrorLog(err, "test err", 2)
	ErrorLog(err, "test err %s format %d", 2)
	ErrorLog(err, "test err format missing %d", 2, time.Now())

	err1 := errors.Wrap(err, "mess for error")
	ErrorLog(err1, "test err wrap", 1)
	ErrorLog(err1, "test err wrap format %d", 1)
	ErrorLog(err1, "test err wrap format missing %d", 1, 3)
}

type fakeWriter struct {
	wg *sync.WaitGroup
}

func newFakeWriter(wg *sync.WaitGroup) *fakeWriter {
	wg.Add(1)

	return &fakeWriter{wg: wg}
}

func (w fakeWriter) Write(b []byte) (int, error) {

	fmt.Println(boldcolors[WARNING] + "fake writer" + string(b) + LogEndColor)

	w.wg.Done()

	return len(b), nil
}

// Func to check error occurance line
func FuncStack(number int) error {
	err := InnerErrorFunc(number)
	if number < 5 && err != nil {
		err = errors.New("number < 5 and InnerErrorFunc() error occured")
		ErrorLog(err)
		return err
	}
	if number >= 5 && err != nil {
		return err
	}
	return err
}

// Func to raise error inside
func InnerErrorFunc(number int) error {
	err := InnerErrorFuncLower(number)
	if number < 5 && err != nil {
		ErrorLog(err)
		return err
	}

	if number < 5 {
		return nil
	}
	err = errors.New("-InnerErrorFunc()- error occured")
	ErrorLog(err)
	return err
}

func InnerErrorFuncLower(number int) error {
	if number < 3 {
		return nil
	}
	err := errors.New("-InnerErrorFuncLower()- error occured")
	ErrorLog(err)
	ErrorStack(err)
	return err
}

func TestLogErr(t *testing.T) {
	err := FuncStack(7)
	ErrorLog(errors.Wrap(err, "uhd3ekuiwe"))
	err = FuncStack(3)
	ErrorLog(errors.Wrap(err, "yw"))
}

func TestErrStack(t *testing.T) {
	err := FuncStack(7)
	ErrorStack(errors.Wrap(err, "uhd3ekuiwe"))
	err = FuncStack(3)
	ErrorStack(errors.Wrap(err, "yw"), "khef")
}

func TestLogsWithSentry(t *testing.T) {
	err := SetSentry("https://5gerstge5rgtry.io/18wstger4tge5rg13325", "ertgesrg")
	fmt.Println(err)
	ErrorLog(errors.New("Test SetSentry"))

	err = fakeErr{}
	ErrorLog(err)
}

func BenchmarkErrorLog(b *testing.B) {

	b.Run("single log", func(b *testing.B) {
		ErrorLog(errors.New("BenchmarkErrorLog testing"), 1)
		b.ReportAllocs()
	})

	wg := &sync.WaitGroup{}
	b.Run("fakewriter", func(b *testing.B) {

		SetWriters(newFakeWriter(wg), FgErr)
		ErrorLog(errors.New("BenchmarkErrorLog testing with "), 1)

		b.ReportAllocs()
	})
	wg.Wait()

}

func TestDebug(t *testing.T) {
	DebugLog("deiwhd", "sduh")
}
