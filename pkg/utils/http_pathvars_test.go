package utils

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestDecodePathVarsMiddleware(t *testing.T) {
	r := mux.NewRouter().UseEncodedPath()
	r.Use(DecodePathVarsMiddleware)
	var got map[string]string
	record := func(w http.ResponseWriter, r *http.Request) { got = mux.Vars(r) }
	r.HandleFunc("/targets", record)
	r.HandleFunc("/targets/{id}", record)
	r.HandleFunc("/targets/{id}/state/{state}", record)

	tests := []struct {
		path string
		code int
		want map[string]string
	}{
		{path: "/targets", code: http.StatusOK, want: map[string]string{}},
		{path: "/targets/router1", code: http.StatusOK, want: map[string]string{"id": "router1"}},
		{path: "/targets/rack%2Frouter", code: http.StatusOK, want: map[string]string{"id": "rack/router"}},
		{path: "/targets/rack%252Frouter", code: http.StatusOK, want: map[string]string{"id": "rack%2Frouter"}},
		{path: "/targets/a%20b", code: http.StatusOK, want: map[string]string{"id": "a b"}},
		{path: "/targets/rack%2Frouter/state/enabled", code: http.StatusOK, want: map[string]string{"id": "rack/router", "state": "enabled"}},
		{path: "/targets/rack/router", code: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got = nil
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if rec.Code != tt.code {
				t.Fatalf("status = %d, want %d", rec.Code, tt.code)
			}
			if tt.want == nil {
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("vars = %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Fatalf("vars = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestDecodePathVarsMiddlewareInvalidEscape(t *testing.T) {
	called := false
	h := DecodePathVarsMiddleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	req := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/targets/x", nil), map[string]string{"id": "bad%zz"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if called {
		t.Fatal("handler called with an invalid escape")
	}
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"errors"`) {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
}
