package streams

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDefaultIOStreams(t *testing.T) {
	s := DefaultIOStreams()

	// Check identity for In(). We won't write to Out/ErrOut to avoid polluting test output.
	if s.In() != os.Stdin {
		t.Fatalf("DefaultIOStreams.In() should be os.Stdin")
	}
	if s.Out() == nil || s.ErrOut() == nil {
		t.Fatalf("DefaultIOStreams Out/ErrOut must be non-nil")
	}
	// Spot-check that type is our concrete BasicIOStreams
	if _, ok := any(s).(BasicIOStreams); !ok {
		t.Fatalf("DefaultIOStreams() must return BasicIOStreams")
	}
}

func TestWriters(t *testing.T) {
	var outBuf, errBuf bytes.Buffer
	s := Writers(&outBuf, &errBuf)

	n, err := s.Out().Write([]byte("hello out\n"))
	if err != nil || n != len("hello out\n") {
		t.Fatalf("Out() write failed: n=%d err=%v", n, err)
	}
	n, err = s.ErrOut().Write([]byte("hello err\n"))
	if err != nil || n != len("hello err\n") {
		t.Fatalf("ErrOut() write failed: n=%d err=%v", n, err)
	}

	if got := outBuf.String(); got != "hello out\n" {
		t.Fatalf("Out buffer = %q, want %q", got, "hello out\n")
	}
	if got := errBuf.String(); got != "hello err\n" {
		t.Fatalf("Err buffer = %q, want %q", got, "hello err\n")
	}

	// Type check
	if _, ok := any(s).(BasicIOStreams); !ok {
		t.Fatalf("Writers() must return BasicIOStreams")
	}
}

func TestDiscard(t *testing.T) {
	s := Discard()

	// Writes should be accepted with full length, but nothing is captured.
	for _, w := range []io.Writer{s.Out(), s.ErrOut()} {
		n, err := w.Write([]byte("dropped\n"))
		if err != nil || n != len("dropped\n") {
			t.Fatalf("discard write failed: n=%d err=%v", n, err)
		}
	}

	// Type check
	if _, ok := any(s).(BasicIOStreams); !ok {
		t.Fatalf("Discard() must return BasicIOStreams")
	}
}

func TestBuffersStreams(t *testing.T) {
	bs := Buffers()

	// Writes accumulate in buffers.
	if _, err := bs.Out().Write([]byte("info 1\n")); err != nil {
		t.Fatalf("write to Out: %v", err)
	}
	if _, err := bs.ErrOut().Write([]byte("err 1\n")); err != nil {
		t.Fatalf("write to ErrOut: %v", err)
	}

	out, errS := bs.Strings()
	if out != "info 1\n" || errS != "err 1\n" {
		t.Fatalf("Strings() = %q / %q, want %q / %q", out, errS, "info 1\n", "err 1\n")
	}

	// Reset clears both.
	bs.Reset()
	out, errS = bs.Strings()
	if out != "" || errS != "" {
		t.Fatalf("after Reset, got %q / %q, want empty / empty", out, errS)
	}

	// In() should be os.Stdin by default.
	if bs.In() != os.Stdin {
		t.Fatalf("BuffersStreams.In() should be os.Stdin")
	}
}

func TestThreadSafeBuffersStreams(t *testing.T) {
	ts := ThreadSafeBuffers()

	var wg sync.WaitGroup
	wg.Add(200)

	// Concurrent writers on Out and ErrOut.
	for i := 0; i < 100; i++ {
		go func(i int) {
			defer wg.Done()
			_, _ = ts.Out().Write([]byte("O"))
		}(i)
		go func(i int) {
			defer wg.Done()
			_, _ = ts.ErrOut().Write([]byte("E"))
		}(i)
	}
	wg.Wait()

	out, errS := ts.Strings()
	if len(out) != 100 || strings.Count(out, "O") != 100 {
		t.Fatalf("Out length/count mismatch, got len=%d, content=%q", len(out), out)
	}
	if len(errS) != 100 || strings.Count(errS, "E") != 100 {
		t.Fatalf("ErrOut length/count mismatch, got len=%d, content=%q", len(errS), errS)
	}

	// Reset clears both.
	ts.Reset()
	out, errS = ts.Strings()
	if out != "" || errS != "" {
		t.Fatalf("after Reset, got %q / %q, want empty / empty", out, errS)
	}

	// In() should be os.Stdin by default.
	if ts.In() != os.Stdin {
		t.Fatalf("ThreadSafeBuffersStreams.In() should be os.Stdin")
	}
}

func TestSlogAdapter(t *testing.T) {
	var buf bytes.Buffer

	// Minimal slog handler writing into our buffer.
	// Use a stable time source to avoid flakiness in assertions.
	th := slog.NewTextHandler(&buf, &slog.HandlerOptions{ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
		// Drop time to make output deterministic
		if a.Key == slog.TimeKey {
			return slog.Attr{}
		}
		return a
	}})
	logger := slog.New(th)

	s := Slog(logger, slog.LevelInfo, slog.LevelError)

	// Writes to Out() should log at info level; ErrOut() at error level.
	if _, err := s.Out().Write([]byte("hello info\n")); err != nil {
		t.Fatalf("write to Out(): %v", err)
	}
	if _, err := s.ErrOut().Write([]byte("boom err\n")); err != nil {
		t.Fatalf("write to ErrOut(): %v", err)
	}

	// Allow handler to flush (handlers are synchronous, but just in case).
	time.Sleep(10 * time.Millisecond)

	got := buf.String()
	// We expect a text record with level and msg fields for both writes. The text
	// handler quotes msg values that contain spaces, so accept quoted form.
	if !strings.Contains(got, "level=INFO") || !strings.Contains(got, "msg=\"hello info\"") {
		t.Fatalf("missing info log in slog output: %q", got)
	}
	if !strings.Contains(got, "level=ERROR") || !strings.Contains(got, "msg=\"boom err\"") {
		t.Fatalf("missing error log in slog output: %q", got)
	}

	// Type check
	if _, ok := any(s).(BasicIOStreams); !ok {
		t.Fatalf("Slog() must return BasicIOStreams")
	}
}
