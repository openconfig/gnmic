// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"log/slog"
	"sync"

	"github.com/IBM/sarama"

	"github.com/openconfig/gnmic/pkg/logging"
)

var saramaLoggerOnce sync.Once

// SetSaramaLoggerOnce configures Sarama's process-wide logger once. Sarama
// exposes this as a global variable, so kafka inputs and outputs must
// coordinate through the same sync.Once to avoid racing assignments during
// startup.
func SetSaramaLoggerOnce(logger *slog.Logger, level slog.Level) {
	saramaLoggerOnce.Do(func() {
		sarama.Logger = logging.StdLogger(logger, level)
	})
}
