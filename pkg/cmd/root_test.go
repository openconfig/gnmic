// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/config"
)

// captureStderr redirects os.Stderr for the duration of f and returns
// everything written to it.
func captureStderr(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() failed: %v", err)
	}
	orig := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = orig }()

	f()

	w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed reading captured stderr: %v", err)
	}
	r.Close()
	return buf.String()
}

func TestWaitForShutdown(t *testing.T) {
	t.Run("not started", func(t *testing.T) {
		if waitForShutdown(make(chan struct{}), make(chan struct{})) {
			t.Fatal("waitForShutdown() = true before shutdown started")
		}
	})

	t.Run("waits for completion", func(t *testing.T) {
		started := make(chan struct{})
		done := make(chan struct{})
		close(started)
		returned := make(chan bool, 1)
		go func() {
			returned <- waitForShutdown(started, done)
		}()

		select {
		case <-returned:
			t.Fatal("waitForShutdown returned before shutdown completed")
		case <-time.After(20 * time.Millisecond):
		}
		close(done)
		select {
		case waited := <-returned:
			if !waited {
				t.Fatal("waitForShutdown() = false after shutdown started")
			}
		case <-time.After(time.Second):
			t.Fatal("waitForShutdown did not return after shutdown completed")
		}
	})
}

// TestInitConfigReportsExplicitMissingConfig is a regression test for
// https://github.com/openconfig/gnmic/issues/772: when the user explicitly
// points --config at a file that does not exist, initConfig() must report
// the load failure on stderr instead of silently continuing as if no config
// file had been requested.
//
// This exercises initConfig() itself (the code that changed), not
// Config.Load() in isolation.
func TestInitConfigReportsExplicitMissingConfig(t *testing.T) {
	origConfig := gApp.Config
	gApp.Config = config.New()
	t.Cleanup(func() {
		gApp.Config = origConfig
	})

	missing := filepath.Join(t.TempDir(), "tunnel_server_config.yaml")
	gApp.Config.CfgFile = missing

	out := captureStderr(t, initConfig)

	if !strings.Contains(out, "failed loading config file") {
		t.Errorf("initConfig() with a missing explicit --config produced no error message; got stderr: %q", out)
	}
	if !strings.Contains(out, missing) {
		t.Errorf("initConfig() error message does not reference the missing config path %q; got: %q", missing, out)
	}
}
