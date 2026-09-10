// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestRecoverMiddleware(t *testing.T) {
	logBuf := new(bytes.Buffer)
	logger := slog.New(slog.NewTextHandler(logBuf, nil))

	r := mux.NewRouter()
	r.Use(RecoverMiddleware(func() *slog.Logger { return logger }))
	r.HandleFunc("/panic/{id}", func(w http.ResponseWriter, r *http.Request) {
		var m map[string]any
		_ = m["type"].(string) // nil map lookup returns nil, the assertion panics
	}).Methods(http.MethodPost)
	r.HandleFunc("/panic-after-write", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("partial"))
		panic("late")
	})
	r.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := w.(http.Flusher); !ok {
			http.Error(w, "flusher lost", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	t.Run("panic before write returns 500 json", func(t *testing.T) {
		resp, err := http.Post(ts.URL+"/panic/x", "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatalf("expected an HTTP response, got %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusInternalServerError {
			t.Fatalf("status %d, want 500", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Fatalf("content-type %q", ct)
		}
		var body struct {
			Errors []string `json:"errors"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if len(body.Errors) != 1 || body.Errors[0] != "internal server error" {
			t.Fatalf("body %+v", body)
		}
		logged := logBuf.String()
		for _, want := range []string{"panic in HTTP handler", "method=POST", "path=/panic/x", "route=/panic/{id}", "interface conversion"} {
			if !strings.Contains(logged, want) {
				t.Errorf("log missing %q:\n%s", want, logged)
			}
		}
	})

	t.Run("panic after write keeps the started response", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/panic-after-write")
		if err != nil {
			t.Fatalf("expected an HTTP response, got %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("status %d, want the handler's 202", resp.StatusCode)
		}
		b, _ := io.ReadAll(resp.Body)
		if string(b) != "partial" {
			t.Fatalf("body %q", b)
		}
	})

	t.Run("flusher is preserved", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/ok")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("status %d: %s", resp.StatusCode, b)
		}
	})
}

func TestRecoverMiddlewareNilLogger(t *testing.T) {
	r := mux.NewRouter()
	r.Use(RecoverMiddleware(nil))
	r.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) { panic("boom") })
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500", rec.Code)
	}
}
