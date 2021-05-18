package logs

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

var ErrBadWriter = errors.New("ErrBadWriter, it will be deleted from multiwriter")

type MultiwriterErr struct {
	ErrorsList []WriterErr
}

func (mwe MultiwriterErr) Error() string {
	return "MultiwriterErr: some writers have errors:\n" + mwe.String()
}

func (mwe MultiwriterErr) String() string {
	endl := ""
	retStr := ""
	for _, writerErr := range mwe.ErrorsList {
		retStr += fmt.Sprintf("%sError during write toOther: %v, writer: %v",
			endl, writerErr.Err, writerErr.Wr)
		endl = "\n"
	}

	return retStr
}

type WriterErr struct {
	Err error
	Wr  io.Writer
}

type MultiWriter struct {
	writers []io.Writer
	lock    sync.RWMutex
}

// NewMultiWriter creates a MultiWriter
func NewMultiWriter(writers ...io.Writer) io.Writer {
	allWriters := make([]io.Writer, 0, len(writers))
	for _, w := range writers {
		if mw, ok := w.(*MultiWriter); ok {
			allWriters = append(allWriters, mw.writers...)
		} else {
			allWriters = append(allWriters, w)
		}
	}

	return &MultiWriter{allWriters, sync.RWMutex{}}
}

func (t *MultiWriter) Write(p []byte) (int, error) {
	errList := make([]WriterErr, 0)
	t.lock.RLock()
	defer func() {
		t.lock.RUnlock()
		for _, item := range errList {
			if item.Err == ErrBadWriter {
				t.Remove(item.Wr)
				break
			}
		}
	}()

	for _, w := range t.writers {
		n, err := w.Write(p)

		if err == ErrBadWriter {
			errList = append(errList, WriterErr{err, w})
		} else if err != nil {
			errList = append(errList, WriterErr{err, w})
		}

		if n != len(p) {
			errList = append(errList, WriterErr{io.ErrShortWrite, w})
		}
	}

	if len(errList) > 0 {
		return len(p), MultiwriterErr{errList}
	}

	return len(p), nil
}

// Remove Removes all writers that are identical to the writer we need to remove
func (t *MultiWriter) Remove(writers ...io.Writer) {
	t.lock.Lock()
	defer t.lock.Unlock()

	for i := len(t.writers) - 1; i >= 0; i-- {
		for _, v := range writers {
			if t.writers[i] == v {
				t.writers = append(t.writers[:i], t.writers[i+1:]...)
				break
			}
		}
	}
}

// Append Appends each writer passed as single writer entity. If multiwriter is passed, appends it as single writer.
func (t *MultiWriter) Append(writers ...io.Writer) {
	t.lock.Lock()
	defer t.lock.Unlock()
	t.writers = append(t.writers, writers...)
}

// AppendWritersSeparately If multiwriter is passed, appends each writer of multiwriter separately
func (t *MultiWriter) AppendWritersSeparately(writers ...io.Writer) {
	t.lock.Lock()
	defer t.lock.Unlock()

	for _, w := range writers {
		if mw, ok := w.(*MultiWriter); ok {
			t.writers = append(t.writers, mw.writers...)
		} else {
			t.writers = append(t.writers, w)
		}
	}
}

var _ io.StringWriter = (*MultiWriter)(nil)

func (t *MultiWriter) WriteString(s string) (n int, err error) {
	p := []byte(s)
	errList := make([]WriterErr, len(t.writers))
	t.lock.RLock()
	defer t.lock.RUnlock()

	for _, w := range t.writers {
		if sw, ok := w.(io.StringWriter); ok {
			n, err = sw.WriteString(s)
		} else {
			n, err = w.Write(p)
		}

		if err != nil {
			errList = append(errList, WriterErr{err, w})
		}

		if n != len(s) {
			errList = append(errList, WriterErr{io.ErrShortWrite, w})
		}
	}

	return len(s), nil
}
