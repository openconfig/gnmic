// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A POST without a `type` field used to panic on an unchecked type assertion,
// which net/http turned into a reset connection with no status code.
func TestServer_ConfigOutputsPost_MissingType(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{name: "missing type", body: `{"name":"o1"}`, want: "output type is required"},
		{name: "empty type", body: `{"name":"o1","type":""}`, want: "output type is required"},
		{name: "non-string type", body: `{"name":"o1","type":42}`, want: "output type is required"},
		{name: "unknown type", body: `{"name":"o1","type":"nope"}`, want: `unknown output type: "nope"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Post(ts.URL+"/api/v1/config/outputs", "application/json", strings.NewReader(tc.body))
			if err != nil {
				t.Fatalf("expected an HTTP response, got %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status %d, want 400", resp.StatusCode)
			}
			var apiErr APIErrors
			if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if len(apiErr.Errors) != 1 || apiErr.Errors[0] != tc.want {
				t.Fatalf("errors %v, want [%q]", apiErr.Errors, tc.want)
			}
		})
	}
}

// Any handler panic must surface as a 500 through the router's recovery middleware.
func TestServer_HandlerPanicReturns500(t *testing.T) {
	s := newTestServer(t)
	s.router.HandleFunc("/api/v1/test-panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/api/v1/test-panic")
	if err != nil {
		t.Fatalf("expected an HTTP response, got %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status %d, want 500", resp.StatusCode)
	}
	var apiErr APIErrors
	if err := json.NewDecoder(resp.Body).Decode(&apiErr); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(apiErr.Errors) != 1 || apiErr.Errors[0] != "internal server error" {
		t.Fatalf("errors %v", apiErr.Errors)
	}
}
