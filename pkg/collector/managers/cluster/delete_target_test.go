package cluster_manager

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/openconfig/gnmic/pkg/collector/managers/testutil"
	"github.com/openconfig/gnmic/pkg/logging"
)

func TestDeleteTargetFromMembersEscapesTargetName(t *testing.T) {
	var mu sync.Mutex
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		paths = append(paths, r.Method+" "+r.URL.EscapedPath())
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	cm := NewClusterManager(testutil.NewTestStore(t))
	cm.logger = logging.DiscardLogger()
	cm.apiClient = srv.Client()
	members := map[string]*Member{"m1": {ID: "m1", Address: strings.TrimPrefix(srv.URL, "http://")}}

	for _, name := range []string{"router1", "rack/router"} {
		if err := cm.deleteTargetFromMembers(t.Context(), name, members); err != nil {
			t.Fatalf("%q: %v", name, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{
		"DELETE /api/v1/config/targets/router1",
		"DELETE /api/v1/config/targets/rack%2Frouter",
	}
	if strings.Join(paths, "\n") != strings.Join(want, "\n") {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}
