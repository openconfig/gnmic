// © 2025 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/zestor-dev/zestor/store"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Flags carries the subset of configuration values needed to build the
// application logger. It is a value type so callers can construct it
// directly without sharing pointers.
type Flags struct {
	Log           bool
	Debug         bool
	LogFile       string
	LogMaxSize    int
	LogMaxBackups int
	LogCompress   bool
}

// FlagsProvider is implemented by types that expose logging Flags.
// Configuration types implement this interface so the logging package
// can construct a logger without depending on the config package.
type FlagsProvider interface {
	LoggingFlags() Flags
}

func GetLogger(level slog.Level, args ...any) *slog.Logger {
	handlerOptions := &slog.HandlerOptions{Level: level}
	return slog.New(slog.NewTextHandler(os.Stderr, handlerOptions)).With(args...)
}

func NewLogger(store store.Store[any], args ...any) *slog.Logger {
	cfg, ok, err := store.Get("global-flags", "global-flags")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error getting global flags: %v. Building a default logger.\n", err)
		return GetLogger(slog.LevelInfo, args...)
	}
	if !ok {
		fmt.Fprintf(os.Stderr, "global flags entry not found in store. Building a default logger.\n")
		return GetLogger(slog.LevelInfo, args...)
	}
	provider, ok := cfg.(FlagsProvider)
	if !ok {
		fmt.Fprintf(os.Stderr, "stored global flags type %T does not implement logging.FlagsProvider. Building a default logger.\n", cfg)
		return GetLogger(slog.LevelInfo, args...)
	}
	flags := provider.LoggingFlags()
	if !flags.Log {
		return slog.New(slog.DiscardHandler)
	}
	level := slog.LevelInfo
	if flags.Debug {
		level = slog.LevelDebug
	}

	handlerOptions := &slog.HandlerOptions{Level: level}
	if flags.LogFile != "" {
		if flags.LogMaxSize > 0 {
			lj := &lumberjack.Logger{
				Filename:   flags.LogFile,
				MaxSize:    clampMaxSize(flags.LogMaxSize),
				MaxBackups: clampMaxBackups(flags.LogMaxBackups),
				Compress:   flags.LogCompress,
			}
			return slog.New(slog.NewTextHandler(lj, handlerOptions)).With(args...)
		}
		f, err := os.OpenFile(flags.LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error opening log file: %v\n", err)
			return GetLogger(slog.LevelInfo, args...)
		}
		return slog.New(slog.NewTextHandler(f, handlerOptions)).With(args...)
	}
	return slog.New(slog.NewTextHandler(os.Stderr, handlerOptions)).With(args...)
}

// minLogMaxSize is the smallest log-file size (MiB) we accept before
// rotating. Anything smaller thrashes the rotator on busy systems.
const minLogMaxSize = 5

// clampMaxSize ensures the configured rotation size is at least
// minLogMaxSize MiB.
func clampMaxSize(v int) int {
	if v < minLogMaxSize {
		return minLogMaxSize
	}
	return v
}

// clampMaxBackups guards against negative inputs (lumberjack treats 0 as
// "keep all backups", which is a valid choice and left untouched).
func clampMaxBackups(v int) int {
	if v < 0 {
		return 0
	}
	return v
}
