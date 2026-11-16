package utils

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

func newCanonicalTestRouter(strictSlash bool) *mux.Router {
	r := mux.NewRouter().UseEncodedPath().SkipClean(true)
	r.NotFoundHandler = CanonicalPathRedirect(r, strictSlash)
	ok := func(http.ResponseWriter, *http.Request) {}
	api := r.PathPrefix("/api/v1").Subrouter()
	api.HandleFunc("/targets", ok).Methods(http.MethodGet)
	api.HandleFunc("/targets/{id}", ok).Methods(http.MethodGet)
	api.HandleFunc("/targets/{id}/subscriptions", ok).Methods(http.MethodPatch)
	r.HandleFunc("/debug/pprof/", ok)
	return r
}

func TestCanonicalPathRedirect(t *testing.T) {
	tests := []struct {
		name         string
		strictSlash  bool
		method       string
		path         string
		wantCode     int
		wantLocation string
	}{
		{name: "exact match", strictSlash: true, path: "/api/v1/targets/rack%2Frouter", wantCode: http.StatusOK},
		{name: "trailing slash keeps escape", strictSlash: true, path: "/api/v1/targets/rack%2Frouter/", wantCode: http.StatusMovedPermanently, wantLocation: "/api/v1/targets/rack%2Frouter"},
		{name: "trailing slash plain name", strictSlash: true, path: "/api/v1/targets/router1/", wantCode: http.StatusMovedPermanently, wantLocation: "/api/v1/targets/router1"},
		{name: "trailing slash on suffix route", strictSlash: true, method: http.MethodPatch, path: "/api/v1/targets/rack%2Frouter/subscriptions/", wantCode: http.StatusMovedPermanently, wantLocation: "/api/v1/targets/rack%2Frouter/subscriptions"},
		{name: "adds slash where the route has one", strictSlash: true, path: "/debug/pprof", wantCode: http.StatusMovedPermanently, wantLocation: "/debug/pprof/"},
		{name: "keeps query", strictSlash: true, path: "/api/v1/targets/?x=1", wantCode: http.StatusMovedPermanently, wantLocation: "/api/v1/targets?x=1"},
		{name: "double slash keeps escape", path: "//api/v1/targets/rack%2Frouter", wantCode: http.StatusMovedPermanently, wantLocation: "/api/v1/targets/rack%2Frouter"},
		{name: "dot segment keeps escape", path: "/api/v1/./targets/rack%2Frouter", wantCode: http.StatusMovedPermanently, wantLocation: "/api/v1/targets/rack%2Frouter"},
		{name: "unclean and trailing slash", strictSlash: true, path: "//api/v1/targets/rack%2Frouter/", wantCode: http.StatusMovedPermanently, wantLocation: "/api/v1/targets/rack%2Frouter"},
		{name: "no trailing slash redirect without strictSlash", path: "/api/v1/targets/rack%2Frouter/", wantCode: http.StatusNotFound},
		{name: "unescaped slash", strictSlash: true, path: "/api/v1/targets/rack/router", wantCode: http.StatusNotFound},
		{name: "unknown path", strictSlash: true, path: "/api/v1/nope/", wantCode: http.StatusNotFound},
		{name: "method mismatch on canonical path", strictSlash: true, method: http.MethodDelete, path: "/api/v1/targets/router1/", wantCode: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method := tt.method
			if method == "" {
				method = http.MethodGet
			}
			rec := httptest.NewRecorder()
			newCanonicalTestRouter(tt.strictSlash).ServeHTTP(rec, httptest.NewRequest(method, tt.path, nil))
			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d (Location %q)", rec.Code, tt.wantCode, rec.Header().Get("Location"))
			}
			if got := rec.Header().Get("Location"); got != tt.wantLocation {
				t.Fatalf("Location = %q, want %q", got, tt.wantLocation)
			}
		})
	}
}
