// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
)

func TestRedact(t *testing.T) {
	if got := Redact(""); got != "" {
		t.Fatalf("Redact(\"\") = %q, want empty", got)
	}
	if got := Redact("secret"); got != RedactedPlaceholder {
		t.Fatalf("Redact(non-empty) = %q, want %q", got, RedactedPlaceholder)
	}
}

func TestRedactedJSON(t *testing.T) {
	in := map[string]any{
		"username": "admin",
		"password": "password1",
		"token":    "",
		"metadata": map[string]any{
			"client-secret": "nested",
			"token":         42,
		},
		"items": []map[string]any{
			{"api-key": "key"},
		},
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(RedactedJSON(in)), &got); err != nil {
		t.Fatalf("RedactedJSON returned invalid JSON: %v", err)
	}
	if got["password"] != RedactedPlaceholder {
		t.Fatalf("password = %v, want redacted", got["password"])
	}
	if got["token"] != "" {
		t.Fatalf("empty token = %v, want empty", got["token"])
	}
	metadata := got["metadata"].(map[string]any)
	if metadata["client-secret"] != RedactedPlaceholder {
		t.Fatalf("nested client-secret = %v, want redacted", metadata["client-secret"])
	}
	if metadata["token"] != float64(42) {
		t.Fatalf("numeric token = %v, want preserved number", metadata["token"])
	}
	items := got["items"].([]any)
	item := items[0].(map[string]any)
	if item["api-key"] != RedactedPlaceholder {
		t.Fatalf("array api-key = %v, want redacted", item["api-key"])
	}
}

func TestRedactedJSONCustomFields(t *testing.T) {
	got := RedactedJSON(map[string]string{"private": "value"}, "private")
	if !strings.Contains(got, `"private":"`+RedactedPlaceholder+`"`) {
		t.Fatalf("custom secret field was not redacted: %s", got)
	}
}

func TestComponent(t *testing.T) {
	Component(nil, "output", "file", "out1").Info("discarded")

	var buf bytes.Buffer
	l := slog.New(slog.NewTextHandler(&buf, nil))
	Component(l, "output", "file", "out1").Info("ready")
	got := buf.String()
	for _, want := range []string{`msg=ready`, `output=file`, `name=out1`} {
		if !strings.Contains(got, want) {
			t.Fatalf("component log %q missing %q", got, want)
		}
	}
}

func TestNewTest(t *testing.T) {
	sink := &testLogSink{}
	NewTest(sink).Debug("hello", "answer", 42)
	if got := sink.String(); !strings.Contains(got, "hello") || !strings.Contains(got, "answer=42") {
		t.Fatalf("NewTest sink = %q, want debug record", got)
	}
}

type testLogSink struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (s *testLogSink) Logf(format string, args ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf.WriteString(fmt.Sprintf(format, args...))
	s.buf.WriteByte('\n')
}

func (s *testLogSink) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

func TestClampRotationValues(t *testing.T) {
	if got := clampMaxSize(1); got != minLogMaxSize {
		t.Fatalf("clampMaxSize(1) = %d, want %d", got, minLogMaxSize)
	}
	if got := clampMaxSize(minLogMaxSize + 1); got != minLogMaxSize+1 {
		t.Fatalf("clampMaxSize(max+1) = %d", got)
	}
	if got := clampMaxBackups(-1); got != 0 {
		t.Fatalf("clampMaxBackups(-1) = %d, want 0", got)
	}
	if got := clampMaxBackups(3); got != 3 {
		t.Fatalf("clampMaxBackups(3) = %d, want 3", got)
	}
}

func TestNewHandlerJSON(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(NewHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}, LogFormatJSON))
	logger.Info("ready", "target", "10.0.0.1:57344")

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("JSON log is invalid: %v\nraw=%q", err, buf.String())
	}
	if got["msg"] != "ready" {
		t.Fatalf("msg = %v, want ready", got["msg"])
	}
	if got["level"] != "INFO" {
		t.Fatalf("level = %v, want INFO", got["level"])
	}
	if _, ok := got["time"]; !ok {
		t.Fatalf("time missing: %v", got)
	}
	if got["target"] != "10.0.0.1:57344" {
		t.Fatalf("target = %v", got["target"])
	}
}

func TestNormalizeLogFormat(t *testing.T) {
	if got := normalizeLogFormat(""); got != LogFormatText {
		t.Fatalf("empty = %q", got)
	}
	if got := normalizeLogFormat("JSON"); got != LogFormatJSON {
		t.Fatalf("JSON = %q", got)
	}
	if got := normalizeLogFormat("weird"); got != LogFormatText {
		t.Fatalf("unsupported = %q", got)
	}
}
