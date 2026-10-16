// Package applog provides logging for cliamp.
//
// Two sinks are layered behind one API:
//
//   - A file sink, written through log/slog, for diagnostic logs the user
//     reads after the fact (~/.config/cliamp/cliamp.log).
//   - An in-memory ring buffer drained by the TUI footer for short-lived,
//     user-facing messages. The buffer exists because writing to stderr
//     would corrupt the TUI.
//
// Callers pick by intent, not sink:
//
//   - Debug/Info/Warn/Error    write only to the file.
//   - Status                   writes only to the footer (transient UI
//     feedback that wouldn't help post-mortem debugging).
//   - UserWarn/UserError       write to both.
//
// Init must be called once at startup. Calls before Init are silently
// dropped on the file side; the footer buffer always works.
package applog

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Entry is a single footer message with a timestamp.
type Entry struct {
	Text string
	At   time.Time
}

// Level is an alias for slog.Level so callers don't need a second import.
type Level = slog.Level

const (
	LevelDebug = slog.LevelDebug
	LevelInfo  = slog.LevelInfo
	LevelWarn  = slog.LevelWarn
	LevelError = slog.LevelError
)

const maxEntries = 4

// maxLogBytes is the size at which the log file rotates to path + ".1".
const maxLogBytes = 1 << 20

var (
	logger atomic.Pointer[slog.Logger]

	mu          sync.Mutex
	entries     []Entry
	currentFile io.Closer
)

func init() {
	logger.Store(slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// Init opens path for append, installs a slog text handler at the given
// level, and returns a close func. Calling Init twice closes the previous
// file before swapping handlers.
func Init(path string, level Level) (func() error, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create log dir: %w", err)
	}
	w, err := openRotating(path, maxLogBytes)
	if err != nil {
		return nil, err
	}

	h := slog.NewTextHandler(w, &slog.HandlerOptions{Level: level})
	logger.Store(slog.New(h))

	mu.Lock()
	prev := currentFile
	currentFile = w
	mu.Unlock()
	if prev != nil {
		_ = prev.Close()
	}

	return func() error {
		mu.Lock()
		defer mu.Unlock()
		if currentFile == nil {
			return nil
		}
		err := currentFile.Close()
		currentFile = nil
		logger.Store(slog.New(slog.NewTextHandler(io.Discard, nil)))
		return err
	}, nil
}

// ParseLevel maps a string to a Level. Empty input maps to LevelInfo.
// "warning" is accepted as an alias for "warn".
func ParseLevel(s string) (Level, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return LevelInfo, nil
	}
	if strings.EqualFold(s, "warning") {
		return LevelWarn, nil
	}
	var lvl Level
	if err := lvl.UnmarshalText([]byte(strings.ToUpper(s))); err != nil {
		return LevelInfo, fmt.Errorf("invalid log level %q (want debug|info|warn|error)", s)
	}
	return lvl, nil
}

// Debug, Info, Warn, Error log only to the file. Format follows fmt.Sprintf.
// The level guard avoids paying the Sprintf cost when the level is filtered out.
func Debug(format string, args ...any) { logf(slog.LevelDebug, format, args...) }
func Info(format string, args ...any)  { logf(slog.LevelInfo, format, args...) }
func Warn(format string, args ...any)  { logf(slog.LevelWarn, format, args...) }
func Error(format string, args ...any) { logf(slog.LevelError, format, args...) }

// Status pushes a message into the footer buffer without writing to the
// log file. Use for ephemeral, user-facing notifications that wouldn't help
// post-mortem debugging.
func Status(format string, args ...any) {
	pushFooter(fmt.Sprintf(format, args...))
}

// UserWarn logs at warn level and pushes the same message into the footer.
// The Sprintf cost is paid unconditionally because the footer needs the
// formatted string, so no Enabled gate here.
func UserWarn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logger.Load().Warn(msg)
	pushFooter(msg)
}

// UserError logs at error level and pushes the same message into the footer.
func UserError(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logger.Load().Error(msg)
	pushFooter(msg)
}

// Drain returns and clears all buffered footer entries.
func Drain() []Entry {
	mu.Lock()
	defer mu.Unlock()
	if len(entries) == 0 {
		return nil
	}
	out := entries
	entries = nil
	return out
}

func logf(level slog.Level, format string, args ...any) {
	lg := logger.Load()
	if !lg.Enabled(context.Background(), level) {
		return
	}
	lg.Log(context.Background(), level, fmt.Sprintf(format, args...))
}

func pushFooter(msg string) {
	msg = strings.TrimRight(msg, "\n")
	if msg == "" {
		return
	}
	mu.Lock()
	entries = append(entries, Entry{Text: msg, At: time.Now()})
	if len(entries) > maxEntries {
		entries = entries[len(entries)-maxEntries:]
	}
	mu.Unlock()
}

// rotatingWriter appends to path and, when a write would pass max bytes,
// renames the file to path+".1" (replacing an older one) and starts a fresh
// file, so the log never holds more than about 2*max bytes.
type rotatingWriter struct {
	mu   sync.Mutex
	f    *os.File
	path string
	max  int64
	size int64
}

func openRotating(path string, max int64) (*rotatingWriter, error) {
	w := &rotatingWriter{path: path, max: max}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotatingWriter) open() error {
	f, err := os.OpenFile(w.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return fmt.Errorf("stat log file: %w", err)
	}
	w.f, w.size = f, st.Size()
	return nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return 0, os.ErrClosed
	}
	if w.size >= w.max {
		_ = w.f.Close()
		if w.size > 2*w.max {
			// A log from before the limit: keeping it as .1 would keep the size problem.
			_ = os.Remove(w.path)
		} else {
			// A failed rename keeps the old file: it grows rather than loses lines.
			_ = os.Rename(w.path, w.path+".1")
		}
		if err := w.open(); err != nil {
			w.f = nil
			return 0, err
		}
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}
