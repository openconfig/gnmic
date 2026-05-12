// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package logging

import "log/slog"

// Component returns a child logger annotated with the component kind and
// instance name. If parent is nil, a discard logger is returned so callers
// can use the result unconditionally.
//
// The kind argument is used both as the attribute key and the attribute
// value, e.g. Component(l, "output", "file", "f1") yields
// output=file name=f1. Common kinds: "output", "input", "processor",
// "cache", "locker", "loader", "action".
func Component(parent *slog.Logger, kind, value, name string) *slog.Logger {
	if parent == nil {
		return DiscardLogger()
	}
	if name == "" {
		return parent.With(kind, value)
	}
	return parent.With(kind, value, "name", name)
}
