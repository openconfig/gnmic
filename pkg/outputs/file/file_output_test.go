// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package file

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
)

func newStore() store.Store[any] {
	return gomap.NewMemStore(store.StoreOptions[any]{})
}

func TestFile_SetDefaults(t *testing.T) {
	f := &File{}
	cases := []struct {
		name   string
		in     *config
		check  func(t *testing.T, c *config)
		expErr bool
	}{
		{
			name: "stdout default",
			in:   &config{},
			check: func(t *testing.T, c *config) {
				if c.FileType != fileType_STDOUT {
					t.Errorf("file type=%q", c.FileType)
				}
				if c.Format != defaultFormat {
					t.Errorf("format=%q", c.Format)
				}
				if c.Separator != defaultSeparator {
					t.Errorf("sep=%q", c.Separator)
				}
				if c.Indent != "  " || !c.Multiline {
					t.Errorf("stdout should imply multiline+indent")
				}
				if c.ConcurrencyLimit != 1 {
					t.Errorf("stdout concurrency=%d", c.ConcurrencyLimit)
				}
			},
		},
		{
			name: "regular file",
			in:   &config{FileName: "/tmp/x.log"},
			check: func(t *testing.T, c *config) {
				if c.ConcurrencyLimit != defaultWriteConcurrency {
					t.Errorf("default concurrency=%d", c.ConcurrencyLimit)
				}
			},
		},
		{
			name:   "proto format rejected",
			in:     &config{Format: "proto"},
			expErr: true,
		},
		{
			name: "multiline default indent",
			in:   &config{FileName: "/tmp/x", Multiline: true},
			check: func(t *testing.T, c *config) {
				if c.Indent != "  " {
					t.Errorf("indent=%q", c.Indent)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := f.setDefaults(tc.in)
			if (err != nil) != tc.expErr {
				t.Fatalf("err=%v expErr=%v", err, tc.expErr)
			}
			if !tc.expErr {
				tc.check(t, tc.in)
			}
		})
	}
}

func TestFile_Validate(t *testing.T) {
	f := &File{}
	if err := f.Validate(map[string]any{"format": "proto"}); err == nil {
		t.Errorf("expected proto error")
	}
	if err := f.Validate(map[string]any{"file-type": "stdout"}); err != nil {
		t.Errorf("valid: %v", err)
	}
	if err := f.Validate(map[string]any{"concurrency-limit": "bad"}); err == nil {
		t.Errorf("expected decode error")
	}
}

func TestFile_InitStdoutLifecycle(t *testing.T) {
	f := &File{}
	cfg := map[string]any{
		"file-type": "stdout",
		"format":    "json",
	}
	if err := f.Init(context.Background(), "out1", cfg, outputs.WithConfigStore(newStore())); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if !strings.Contains(f.String(), "stdout") {
		t.Errorf("String() = %s", f.String())
	}
	// Update with same target type but different separator
	cfg2 := map[string]any{
		"file-type": "stdout",
		"format":    "json",
		"separator": ";",
	}
	if err := f.Update(context.Background(), cfg2); err != nil {
		t.Fatalf("Update: %v", err)
	}
	// Update decode error
	if err := f.Update(context.Background(), map[string]any{"concurrency-limit": "x"}); err == nil {
		t.Errorf("expected decode error")
	}
	// Note: not calling Close() here; see TestFile_CloseStdoutIsNoop which
	// asserts closing a stdout output is a safe no-op.
}

// TestFile_CloseStdoutIsNoop is a regression test: Close() on a file output
// backed by os.Stdout/os.Stderr must not close the shared process fd. Otherwise
// deleting one stdout output would break the collector's logging and cause a
// "file already closed" error when a second stdout output is closed.
func TestFile_CloseStdoutIsNoop(t *testing.T) {
	cfg := map[string]any{"file-type": "stdout", "format": "json"}

	f1 := &File{}
	if err := f1.Init(context.Background(), "out1", cfg, outputs.WithConfigStore(newStore())); err != nil {
		t.Fatalf("Init out1: %v", err)
	}
	f2 := &File{}
	if err := f2.Init(context.Background(), "out2", cfg, outputs.WithConfigStore(newStore())); err != nil {
		t.Fatalf("Init out2: %v", err)
	}

	if err := f1.Close(); err != nil {
		t.Fatalf("Close out1: %v", err)
	}
	// Before the fix this second close returned "file already closed".
	if err := f2.Close(); err != nil {
		t.Fatalf("Close out2: %v", err)
	}
	// os.Stdout must still be open/usable.
	if _, err := os.Stdout.Stat(); err != nil {
		t.Fatalf("os.Stdout was closed by File.Close(): %v", err)
	}
}

func TestFile_InitRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.log")
	f := &File{}
	cfg := map[string]any{
		"filename":          path,
		"format":            "json",
		"concurrency-limit": 1,
	}
	if err := f.Init(context.Background(), "out1", cfg, outputs.WithConfigStore(newStore())); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer f.Close()

	// reopen via Update with a new filename
	path2 := filepath.Join(dir, "out2.log")
	cfg2 := map[string]any{
		"filename":          path2,
		"format":            "json",
		"concurrency-limit": 2,
	}
	if err := f.Update(context.Background(), cfg2); err != nil {
		t.Fatalf("Update reopen: %v", err)
	}
}

func TestFile_InitDecodeError(t *testing.T) {
	f := &File{}
	if err := f.Init(context.Background(), "out1", map[string]any{
		"concurrency-limit": "x",
	}, outputs.WithConfigStore(newStore())); err == nil {
		t.Errorf("expected decode error")
	}
}

func TestFile_Predicates(t *testing.T) {
	if !fileNeedsReopen(nil, &config{}) {
		t.Errorf("nil should need reopen")
	}
	if fileNeedsReopen(&config{FileName: "a"}, &config{FileName: "a"}) {
		t.Errorf("same name should not need reopen")
	}
	if !fileNeedsReopen(&config{FileName: "a"}, &config{FileName: "b"}) {
		t.Errorf("name change should need reopen")
	}
	if rotationChanged(nil, nil) {
		t.Errorf("both nil unchanged")
	}
	if !rotationChanged(nil, &rotationConfig{}) {
		t.Errorf("nil/non-nil changed")
	}
	if rotationChanged(&rotationConfig{MaxSize: 1}, &rotationConfig{MaxSize: 1}) {
		t.Errorf("same rotation unchanged")
	}
	if !rotationChanged(&rotationConfig{MaxSize: 1}, &rotationConfig{MaxSize: 2}) {
		t.Errorf("max size change should be changed")
	}
}
