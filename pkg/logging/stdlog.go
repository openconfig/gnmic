// © 2025 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"log"
	"log/slog"
)

// DiscardLogger returns a slog.Logger that discards all records.
func DiscardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// StdLogger returns a *log.Logger that writes through l's Handler using level as the record level.
func StdLogger(l *slog.Logger, level slog.Level) *log.Logger {
	if l == nil {
		return slog.NewLogLogger(slog.DiscardHandler, level)
	}
	return slog.NewLogLogger(l.Handler(), level)
}
