// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"context"
	"errors"
	"log/slog"
)

// LogErrUnlessCanceled logs msg with attrs plus "err", err.
// It logs at Error level when err is not nil and not context.Canceled,
// and at Info level when err is context.Canceled.
// It is a no-op when logger or err is nil.
func LogErrUnlessCanceled(logger *slog.Logger, err error, msg string, attrs ...any) {
	if logger == nil || err == nil {
		return
	}
	args := append(append([]any{}, attrs...), "err", err)
	if errors.Is(err, context.Canceled) {
		logger.Info(msg, args...)
		return
	}
	logger.Error(msg, args...)
}
