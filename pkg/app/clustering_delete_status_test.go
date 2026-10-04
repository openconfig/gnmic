package app

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/openconfig/gnmic/pkg/lockers"
)

// A member that answers DELETE with a non-2xx status has not deleted the target.
// deleteTarget used to log the status and return nil, so the loader believed the
// deletion had been applied and never sent it again.
func TestClusterDeleteTargetRejectsNon2xx(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  int
		wantErr bool
	}{
		{name: "503 service unavailable", status: http.StatusServiceUnavailable, wantErr: true},
		{name: "500 internal error", status: http.StatusInternalServerError, wantErr: true},
		{name: "401 unauthorized", status: http.StatusUnauthorized, wantErr: true},
		// A member that no longer knows the target has nothing left to delete,
		// so the desired state is already true and this is not a failure.
		{name: "404 already deleted", status: http.StatusNotFound, wantErr: false},
		{name: "204 deleted", status: http.StatusNoContent, wantErr: false},
		{name: "200 ok", status: http.StatusOK, wantErr: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var mu sync.Mutex
			gotMethod := ""
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				gotMethod = r.Method
				mu.Unlock()
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			addr := srv.Listener.Addr().String()
			a := &App{
				configLock:       new(sync.RWMutex),
				apiServices:      map[string]*lockers.Service{"member": {ID: "member", Address: addr}},
				clusteringClient: &http.Client{},
				Logger:           slog.New(slog.DiscardHandler),
			}

			err := a.deleteTarget(t.Context(), "t1")
			mu.Lock()
			method := gotMethod
			mu.Unlock()
			if method != http.MethodDelete {
				t.Fatalf("expected a DELETE to the member, got %s", method)
			}
			if tc.wantErr && err == nil {
				t.Fatalf("deleteTarget returned nil for status %d: a member that "+
					"did not delete the target must be reported as an error so the "+
					"loader retries it", tc.status)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("deleteTarget returned an unexpected error for status %d: %v",
					tc.status, err)
			}
		})
	}
}
