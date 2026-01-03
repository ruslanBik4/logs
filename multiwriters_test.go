package logs

import (
	"fmt"
	"io"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestErrorLogOthers(t *testing.T) {
	runtime.GOMAXPROCS(3)
	t.Run("testErrtoOthers", func(t *testing.T) {
		wg := &sync.WaitGroup{}
		wg.Add(2)

		fwriter := fakeWriter{wg}
		mw := logErr.toOther.(*MultiWriter)

		SetWriters(fwriter, FgErr)
		SetWriters(fwriter, FgErr)

		var err fakeErr

		ErrorLog(err, "test err", 1)

		wg.Wait()

		wg.Add(1)
		DeleteWriters(fwriter, FgErr)
		if len(mw.writers) == 0 {
			wg.Done()
		}

		wg.Wait()
	})

}

func RemoveTest(mw *MultiWriter, w io.Writer, wg *sync.WaitGroup) {
	old_len := len(mw.writers)
	mw.Remove(w)
	if old_len == len(mw.writers)+1 {
		wg.Done()
	}
}

func TestRemoveOneMultiwriter(t *testing.T) {
	runtime.GOMAXPROCS(2)
	wg := &sync.WaitGroup{}
	wg.Add(1)
	b := testWriter2{wg, "Second"}
	m := NewMultiWriter(b)

	mw := m.(*MultiWriter)

	RemoveTest(mw, b, wg)

	data := []byte("Hello ")
	_, e := m.Write(data)
	if e != nil {
		panic(e)
	}
	time.Sleep(10 * time.Millisecond)
	wg.Wait()
}

func TestLogsMultiwriter(t *testing.T) {
	runtime.GOMAXPROCS(3)
	wg := &sync.WaitGroup{}

	a := testWriter{wg, "first"}
	b := testWriter2{wg, "Second"}
	m := NewMultiWriter(a, a)
	m = NewMultiWriter(m, a)

	mw := m.(*MultiWriter)

	mw.Append(a, b)
	mw.Remove(b)
	mw.Append(b)

	wg.Add(len(mw.writers))
	data := []byte("Hello ")
	_, e := m.Write(data)
	if e != nil {
		t.Fatal(e)
	}
	wg.Wait()
}

func TestCustomLog(t *testing.T) {
	CustomLog(NOTICE, "TEST", "test.go", 0, "test custom log", FgAll)
}

func TestErrorsMultiwriter(t *testing.T) {
	a := testErrorWriter{}
	b := testBadWriter{}
	m := NewMultiWriter(a, a)
	m = NewMultiWriter(m, a)
	m = NewMultiWriter(m, b)

	for i := 0; i < 2; i++ {
		lenWriters := len(m.(*MultiWriter).writers)
		_, e := m.Write([]byte("Hello "))
		if errMultiwriter, ok := e.(MultiWriterErr); ok {
			assert.Equal(t, lenWriters, len(errMultiwriter.ErrorsList))
			for _, writerErr := range errMultiwriter.ErrorsList {
				fmt.Printf("error: %v, writer:%v\n", writerErr.err, writerErr.w)
			}
		} else if e != nil {
			fmt.Println("error: ", e)
		}
	}

	assert.Equal(t, 3, len(m.(*MultiWriter).writers))
}

type testErrorWriter struct{}

func (tew testErrorWriter) Write(b []byte) (int, error) {
	fmt.Println("testErrorWriter: ", string(b))
	return len(b), errors.New("TestErrorWrite Error occurred")
}

type testBadWriter struct{}

func (tbw testBadWriter) Write(b []byte) (int, error) {
	fmt.Println("testBadWriter: ", string(b))
	return len(b), ErrBadWriter
}

type testWriter struct {
	wg   *sync.WaitGroup
	name string
}

func (tw testWriter) Write(b []byte) (int, error) {
	fmt.Println(boldcolors[WARNING] + tw.name + "|| testWriter writer: " + string(b) + LogEndColor)
	tw.wg.Done()
	return len(b), nil
}

type testWriter2 struct {
	wg   *sync.WaitGroup
	name string
}

func (tw2 testWriter2) Write(b []byte) (int, error) {
	fmt.Println(boldcolors[DEBUG] + tw2.name + "|| testWriter2 writer: " + string(b) + LogEndColor)
	tw2.wg.Done()
	return len(b), nil
}
