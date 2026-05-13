// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"io"
	"log/slog"
	"sync"
)

// TestLogf is the subset of testing.TB used by NewTest. Defined as an
// interface so this package does not depend on the testing package and so
// callers can plug in their own sinks.
type TestLogf interface {
	Logf(format string, args ...any)
}

// NewTest returns a slog.Logger that forwards every record to t.Logf at
// LevelDebug or above. Useful for tests that want logger output captured
// alongside other test output.
func NewTest(t TestLogf) *slog.Logger {
	w := &testWriter{t: t}
	h := slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(h)
}

type testWriter struct {
	mu sync.Mutex
	t  TestLogf
}

func (w *testWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	// trim trailing newline so t.Logf doesn't double-space
	n := len(p)
	if n > 0 && p[n-1] == '\n' {
		w.t.Logf("%s", p[:n-1])
	} else {
		w.t.Logf("%s", p)
	}
	return n, nil
}

// compile-time assertion that testWriter implements io.Writer.
var _ io.Writer = (*testWriter)(nil)
