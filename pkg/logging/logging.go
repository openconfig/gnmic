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
	"reflect"

	"github.com/openconfig/gnmic/pkg/config"
	"github.com/zestor-dev/zestor/store"
	"gopkg.in/natefinch/lumberjack.v2"
)

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
		fmt.Fprintf(os.Stderr, "globalFlags is of an unexpected type: %T. Building a default logger.\n", reflect.TypeOf(cfg))
		return GetLogger(slog.LevelInfo, args...)
	}
	flags := config.GlobalFlags{}
	switch i := cfg.(type) {
	case config.GlobalFlags:
		flags = i
	default:
		fmt.Fprintf(os.Stderr, "globalFlags is of an unexpected type: %T. Building a default logger.\n", reflect.TypeOf(cfg))
		return GetLogger(slog.LevelInfo, args...)
	}
	if !flags.Log {
		return slog.New(slog.DiscardHandler)
	}
	var level slog.Level
	if flags.Debug {
		level = slog.LevelDebug
	} else {
		level = slog.LevelInfo
	}

	handlerOptions := &slog.HandlerOptions{Level: level}
	if flags.LogFile != "" {
		if flags.LogMaxSize > 0 {
			lj := &lumberjack.Logger{
				Filename:   flags.LogFile,
				MaxSize:    flags.LogMaxSize,
				MaxBackups: flags.LogMaxBackups,
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
