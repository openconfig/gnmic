package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/openconfig/gnmic/pkg/api/target"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/lockers"
)

func newEncodedPathTestApp(t *testing.T) (*App, *httptest.Server) {
	t.Helper()
	a := New()
	t.Cleanup(a.Cfn)
	a.Config.APIServer = &config.APIServer{}
	a.Config.Targets = make(map[string]*types.TargetConfig)
	a.Config.Subscriptions = make(map[string]*types.SubscriptionConfig)
	a.routes()
	ts := httptest.NewServer(a.router)
	t.Cleanup(ts.Close)
	return a, ts
}

func doAPIRequest(t *testing.T, method, rawURL, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, rawURL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestAPITargetNamesWithSlash(t *testing.T) {
	a, ts := newEncodedPathTestApp(t)
	base := ts.URL + "/api/v1"
	names := []string{"router1", "rack/router"}

	for _, name := range names {
		b, _ := json.Marshal(&types.TargetConfig{Name: name, Address: "127.0.0.1:1"})
		if code, body := doAPIRequest(t, http.MethodPost, base+"/config/targets", string(b)); code != http.StatusOK {
			t.Fatalf("create %q: status %d: %s", name, code, body)
		}
	}

	t.Run("list includes slash name", func(t *testing.T) {
		code, body := doAPIRequest(t, http.MethodGet, base+"/config/targets", "")
		if code != http.StatusOK || !strings.Contains(body, `"rack/router"`) {
			t.Fatalf("status %d: %s", code, body)
		}
	})

	t.Run("get config by escaped name", func(t *testing.T) {
		for _, name := range names {
			code, body := doAPIRequest(t, http.MethodGet, base+"/config/targets/"+url.PathEscape(name), "")
			tc := new(types.TargetConfig)
			if code != http.StatusOK || json.Unmarshal([]byte(body), tc) != nil || tc.Name != name {
				t.Fatalf("%q: status %d: %s", name, code, body)
			}
		}
	})

	t.Run("unescaped slash is not a target id", func(t *testing.T) {
		code, body := doAPIRequest(t, http.MethodGet, base+"/config/targets/rack/router", "")
		if code != http.StatusNotFound || strings.Contains(body, "errors") {
			t.Fatalf("expected router 404, got %d: %s", code, body)
		}
	})

	t.Run("patch subscriptions finds target", func(t *testing.T) {
		// an unknown subscription is rejected only after the target lookup
		// succeeded, which keeps the target from being restarted.
		for _, name := range names {
			code, body := doAPIRequest(t, http.MethodPatch, base+"/config/targets/"+url.PathEscape(name)+"/subscriptions",
				`{"subscriptions":["missing"]}`)
			if code != http.StatusBadRequest || !strings.Contains(body, `subscription \"missing\" does not exist`) {
				t.Fatalf("%q: status %d: %s", name, code, body)
			}
		}
	})

	t.Run("runtime target get and delete", func(t *testing.T) {
		a.operLock.Lock()
		for _, name := range names {
			a.Targets[name] = target.NewTarget(&types.TargetConfig{Name: name})
		}
		a.operLock.Unlock()
		for _, name := range names {
			endpoint := base + "/targets/" + url.PathEscape(name)
			code, body := doAPIRequest(t, http.MethodGet, endpoint, "")
			if code != http.StatusOK || !strings.Contains(body, `"name":"`+name+`"`) {
				t.Fatalf("get %q: status %d: %s", name, code, body)
			}
			if code, body = doAPIRequest(t, http.MethodDelete, endpoint, ""); code != http.StatusOK {
				t.Fatalf("delete %q: status %d: %s", name, code, body)
			}
			if _, ok := a.targetByName(name); ok || a.targetConfigExists(name) {
				t.Fatalf("%q still present after delete", name)
			}
		}
	})

	t.Run("start missing target reports decoded name", func(t *testing.T) {
		code, body := doAPIRequest(t, http.MethodPost, base+"/targets/missing%2Fname", "")
		if code != http.StatusNotFound || !strings.Contains(body, `target \"missing/name\" not found`) {
			t.Fatalf("status %d: %s", code, body)
		}
	})

	t.Run("delete config by escaped name", func(t *testing.T) {
		a.AddTargetConfig(&types.TargetConfig{Name: "rack/router", Address: "127.0.0.1:1"})
		if code, body := doAPIRequest(t, http.MethodDelete, base+"/config/targets/rack%2Frouter", ""); code != http.StatusOK {
			t.Fatalf("status %d: %s", code, body)
		}
		if a.targetConfigExists("rack/router") {
			t.Fatal("rack/router still configured")
		}
	})
}

func TestClusteringCallersEscapeTargetNames(t *testing.T) {
	const name = "rack/router"
	member, ts := newEncodedPathTestApp(t)

	leader := New()
	t.Cleanup(leader.Cfn)
	leader.clusteringClient = ts.Client()
	const serviceID = "member-api"
	leader.apiServices[serviceID] = &lockers.Service{ID: serviceID, Address: strings.TrimPrefix(ts.URL, "http://")}

	t.Run("deleteTarget", func(t *testing.T) {
		member.AddTargetConfig(&types.TargetConfig{Name: name, Address: "127.0.0.1:1"})
		if err := leader.deleteTarget(t.Context(), name); err != nil {
			t.Fatal(err)
		}
		if member.targetConfigExists(name) {
			t.Fatal("member still has the target config")
		}
	})

	t.Run("unassignTarget", func(t *testing.T) {
		member.AddTargetConfig(&types.TargetConfig{Name: name, Address: "127.0.0.1:1"})
		member.operLock.Lock()
		member.Targets[name] = target.NewTarget(&types.TargetConfig{Name: name})
		member.operLock.Unlock()
		if err := leader.unassignTarget(t.Context(), name, serviceID); err != nil {
			t.Fatal(err)
		}
		if _, ok := member.targetByName(name); ok {
			t.Fatal("member still runs the target")
		}
	})

	t.Run("assignTarget", func(t *testing.T) {
		var mu sync.Mutex
		var paths []string
		rec := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			paths = append(paths, r.Method+" "+r.URL.EscapedPath())
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(rec.Close)
		svc := &lockers.Service{ID: "rec-api", Address: strings.TrimPrefix(rec.URL, "http://")}
		if err := leader.assignTarget(t.Context(), &types.TargetConfig{Name: name}, svc); err != nil {
			t.Fatal(err)
		}
		mu.Lock()
		defer mu.Unlock()
		want := []string{"POST /api/v1/config/targets", "POST /api/v1/targets/rack%2Frouter"}
		if !slices.Equal(paths, want) {
			t.Fatalf("paths = %v, want %v", paths, want)
		}
	})
}
