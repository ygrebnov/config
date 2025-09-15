// Package streams provides IOStreams adapters for the config Provider. It offers
// ready‑to‑use implementations that can write to stdout/stderr, discard output,
// capture output in memory buffers (with optional thread safety), or forward
// messages to structured loggers like slog.
package streams

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"sync"
)

// IOStreams defines the minimal contract for user‑facing streams used by the
// config Provider. Types that implement these three methods can be passed to
// config.WithStreams(...) even if they are defined in a different package.
//
// Note: Interfaces in Go are satisfied implicitly. All concrete types in this
// package (BasicIOStreams, BuffersStreams, ThreadSafeBuffersStreams) satisfy this
// contract and can be used directly with the config package.
type IOStreams interface {
	In() io.Reader
	Out() io.Writer
	ErrOut() io.Writer
}

// BasicIOStreams is a simple, zero‑dependency implementation of IOStreams.
// It forwards writes to the supplied io.Writer targets. Use the helpers
// DefaultIOStreams, Writers, Discard, and Slog to construct values quickly.
type BasicIOStreams struct {
	in     io.Reader
	out    io.Writer
	errOut io.Writer
}

func (s BasicIOStreams) In() io.Reader     { return s.in }
func (s BasicIOStreams) Out() io.Writer    { return s.out }
func (s BasicIOStreams) ErrOut() io.Writer { return s.errOut }

// DefaultIOStreams returns a BasicIOStreams backed by os.Stdin, os.Stdout and os.Stderr.
func DefaultIOStreams() BasicIOStreams {
	return BasicIOStreams{
		in:     os.Stdin,
		out:    os.Stdout,
		errOut: os.Stderr,
	}
}

// ---------- Basic writers ----------

// writers is an internal helper (unexported) used by Slog.
type writers struct {
	in     io.Reader
	out    io.Writer
	errOut io.Writer
}

func (w writers) In() io.Reader     { return w.in }
func (w writers) Out() io.Writer    { return w.out }
func (w writers) ErrOut() io.Writer { return w.errOut }

// Writers returns a BasicIOStreams that writes Out to `out` and ErrOut to `err`.
// In is set to os.Stdin.
func Writers(out, err io.Writer) BasicIOStreams {
	return BasicIOStreams{in: os.Stdin, out: out, errOut: err}
}

// Discard returns a BasicIOStreams that drops all output (useful for "--silent").
func Discard() BasicIOStreams {
	return Writers(io.Discard, io.Discard)
}

// ---------- Buffers (capture then flush) ----------

// BuffersStreams captures output into bytes.Buffers. Use this when you want to
// accumulate messages and flush or inspect them after Provider.Get() completes.
// It is NOT safe for concurrent writers; see ThreadSafeBuffersStreams for a
// synchronized variant.
type BuffersStreams struct {
	InR    io.Reader
	OutBuf *bytes.Buffer
	ErrBuf *bytes.Buffer
}

// Buffers creates a new BuffersStreams with fresh buffers for Out and ErrOut.
func Buffers() *BuffersStreams {
	return &BuffersStreams{
		InR:    os.Stdin,
		OutBuf: &bytes.Buffer{},
		ErrBuf: &bytes.Buffer{},
	}
}
func (b *BuffersStreams) In() io.Reader     { return b.InR }
func (b *BuffersStreams) Out() io.Writer    { return b.OutBuf }
func (b *BuffersStreams) ErrOut() io.Writer { return b.ErrBuf }

// Strings returns the current contents of the Out and ErrOut buffers as strings.
func (b *BuffersStreams) Strings() (out, err string) {
	return b.OutBuf.String(), b.ErrBuf.String()
}

// Reset clears both Out and ErrOut buffers.
func (b *BuffersStreams) Reset() {
	b.OutBuf.Reset()
	b.ErrBuf.Reset()
}

// tsBuf is a minimal mutex‑protected buffer.
type tsBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (t *tsBuf) Write(p []byte) (int, error) { t.mu.Lock(); defer t.mu.Unlock(); return t.b.Write(p) }
func (t *tsBuf) String() string              { t.mu.Lock(); defer t.mu.Unlock(); return t.b.String() }
func (t *tsBuf) Reset()                      { t.mu.Lock(); defer t.mu.Unlock(); t.b.Reset() }

// ThreadSafeBuffersStreams captures output into mutex‑protected buffers and is
// safe for concurrent writers.
type ThreadSafeBuffersStreams struct {
	InR    io.Reader
	OutBuf *tsBuf
	ErrBuf *tsBuf
}

// ThreadSafeBuffers creates a new thread‑safe buffers stream set.
func ThreadSafeBuffers() *ThreadSafeBuffersStreams {
	return &ThreadSafeBuffersStreams{
		InR:    os.Stdin,
		OutBuf: &tsBuf{},
		ErrBuf: &tsBuf{},
	}
}
func (b *ThreadSafeBuffersStreams) In() io.Reader     { return b.InR }
func (b *ThreadSafeBuffersStreams) Out() io.Writer    { return b.OutBuf }
func (b *ThreadSafeBuffersStreams) ErrOut() io.Writer { return b.ErrBuf }

// Strings returns the current contents of the Out and ErrOut buffers as strings.
func (b *ThreadSafeBuffersStreams) Strings() (string, string) {
	return b.OutBuf.String(), b.ErrBuf.String()
}

// Reset clears both Out and ErrOut buffers.
func (b *ThreadSafeBuffersStreams) Reset() { b.OutBuf.Reset(); b.ErrBuf.Reset() }

// ---------- slog adapter ----------

// slogWriter adapts slog.Logger to io.Writer and trims trailing newlines.
type slogWriter struct {
	l     *slog.Logger
	level slog.Level
}

func (w slogWriter) Write(p []byte) (int, error) {
	// trim trailing newline so each Write is one log record
	n := len(p)
	if n > 0 && p[n-1] == '\n' {
		p = p[:n-1]
	}
	w.l.Log(nil, w.level, string(p))
	return n, nil
}

// Slog returns a BasicIOStreams that writes Provider messages to a slog.Logger.
// Info‑level messages are written to `info`, and error/warning messages to `err`.
func Slog(l *slog.Logger, info, err slog.Level) BasicIOStreams {
	return BasicIOStreams{
		in:     os.Stdin,
		out:    slogWriter{l: l, level: info},
		errOut: slogWriter{l: l, level: err},
	}
}
