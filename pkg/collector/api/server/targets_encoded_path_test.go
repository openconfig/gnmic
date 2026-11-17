package apiserver

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/openconfig/gnmic/pkg/api/types"
)

func doRequest(t *testing.T, method, rawURL, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, rawURL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func getConfigTarget(t *testing.T, base, name string) *types.TargetConfig {
	t.Helper()
	code, body := doRequest(t, http.MethodGet, base+"/api/v1/config/targets/"+url.PathEscape(name), "")
	if code != http.StatusOK {
		t.Fatalf("GET config target %q: status %d: %s", name, code, body)
	}
	tc := new(types.TargetConfig)
	if err := json.Unmarshal([]byte(body), tc); err != nil {
		t.Fatalf("decode %q: %v: %s", name, err, body)
	}
	return tc
}

func TestServer_TargetNamesWithSlash(t *testing.T) {
	s := newTestServer(t)
	if _, err := s.store.Config.Set("subscriptions", "sub1", &types.SubscriptionConfig{Name: "sub1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.store.Config.Set("outputs", "out1", map[string]any{"type": "file"}); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)
	base := ts.URL + "/api/v1"

	for _, name := range []string{"router1", "rack/router", "rack%2Frouter", "a b"} {
		code, body := doRequest(t, http.MethodPost, base+"/config/targets",
			`{"name":`+mustJSON(t, name)+`,"address":"127.0.0.1:1"}`)
		if code != http.StatusOK {
			t.Fatalf("create %q: status %d: %s", name, code, body)
		}
	}

	t.Run("list includes slash name", func(t *testing.T) {
		code, body := doRequest(t, http.MethodGet, base+"/config/targets", "")
		if code != http.StatusOK {
			t.Fatalf("status %d: %s", code, body)
		}
		m := map[string]any{}
		if err := json.Unmarshal([]byte(body), &m); err != nil {
			t.Fatal(err)
		}
		if _, ok := m["rack/router"]; !ok {
			t.Fatalf("rack/router missing from list: %s", body)
		}
	})

	t.Run("get by escaped name", func(t *testing.T) {
		for _, name := range []string{"router1", "rack/router", "rack%2Frouter", "a b"} {
			if got := getConfigTarget(t, ts.URL, name).Name; got != name {
				t.Fatalf("got target %q, want %q", got, name)
			}
		}
	})

	t.Run("decodes path parameters once", func(t *testing.T) {
		code, body := doRequest(t, http.MethodGet, base+"/config/targets/rack%252Frouter", "")
		if code != http.StatusOK || !strings.Contains(body, `"name":"rack%2Frouter"`) {
			t.Fatalf("status %d: %s", code, body)
		}
	})

	t.Run("path cleaning redirect keeps the escaped name", func(t *testing.T) {
		client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		for _, path := range []string{"//api/v1/config/targets/rack%2Frouter", "/api/v1/./config/targets/rack%2Frouter"} {
			resp, err := client.Get(ts.URL + path)
			if err != nil {
				t.Fatal(err)
			}
			resp.Body.Close()
			if want := "/api/v1/config/targets/rack%2Frouter"; resp.StatusCode != http.StatusMovedPermanently || resp.Header.Get("Location") != want {
				t.Fatalf("%s: status %d, Location %q, want %q", path, resp.StatusCode, resp.Header.Get("Location"), want)
			}
		}
		// the collector router has never redirected trailing slashes.
		if code, _ := doRequest(t, http.MethodGet, base+"/config/targets/rack%2Frouter/", ""); code != http.StatusNotFound {
			t.Fatalf("trailing slash: status %d", code)
		}
	})

	t.Run("unescaped slash is not a target id", func(t *testing.T) {
		for _, method := range []string{http.MethodGet, http.MethodDelete} {
			code, body := doRequest(t, method, base+"/config/targets/rack/router", "")
			if code != http.StatusNotFound || strings.Contains(body, "errors") {
				t.Fatalf("%s: expected router 404, got %d: %s", method, code, body)
			}
		}
	})

	t.Run("patch subscriptions and outputs", func(t *testing.T) {
		for _, name := range []string{"router1", "rack/router"} {
			prefix := base + "/config/targets/" + url.PathEscape(name)
			if code, body := doRequest(t, http.MethodPatch, prefix+"/subscriptions", `["sub1"]`); code != http.StatusOK {
				t.Fatalf("%q subscriptions: status %d: %s", name, code, body)
			}
			if code, body := doRequest(t, http.MethodPatch, prefix+"/outputs", `["out1"]`); code != http.StatusOK {
				t.Fatalf("%q outputs: status %d: %s", name, code, body)
			}
			tc := getConfigTarget(t, ts.URL, name)
			if !slices.Equal(tc.Subscriptions, []string{"sub1"}) || !slices.Equal(tc.Outputs, []string{"out1"}) {
				t.Fatalf("%q: subscriptions=%v outputs=%v", name, tc.Subscriptions, tc.Outputs)
			}
		}
	})

	t.Run("runtime routes reach handler with decoded id", func(t *testing.T) {
		// the targets manager is not running, so the handlers report the
		// target as missing; a router-level 404 would not be JSON.
		for _, tc := range []struct {
			method, path, body string
		}{
			{http.MethodGet, "/targets/rack%2Frouter", ""},
			{http.MethodPost, "/targets/rack%2Frouter/state/disabled", ""},
			{http.MethodPost, "/config/targets/rack%2Frouter/state", `{"state":"disabled"}`},
		} {
			code, body := doRequest(t, tc.method, base+tc.path, tc.body)
			if code != http.StatusNotFound || !strings.Contains(body, "target not found") {
				t.Fatalf("%s %s: status %d: %s", tc.method, tc.path, code, body)
			}
		}
	})

	t.Run("assignments", func(t *testing.T) {
		code, body := doRequest(t, http.MethodPost, base+"/assignments",
			`{"assignments":[{"target":"rack/router","member":"m1"}]}`)
		if code != http.StatusOK {
			t.Fatalf("create: status %d: %s", code, body)
		}
		code, body = doRequest(t, http.MethodGet, base+"/assignments/rack%2Frouter", "")
		if code != http.StatusOK || !strings.Contains(body, `"target":"rack/router"`) {
			t.Fatalf("get: status %d: %s", code, body)
		}
		if code, body = doRequest(t, http.MethodDelete, base+"/assignments/rack%2Frouter", ""); code != http.StatusOK {
			t.Fatalf("delete: status %d: %s", code, body)
		}
		if code, _ = doRequest(t, http.MethodGet, base+"/assignments/rack%2Frouter", ""); code != http.StatusNotFound {
			t.Fatalf("get after delete: status %d", code)
		}
	})

	t.Run("delete by escaped name", func(t *testing.T) {
		for _, name := range []string{"router1", "rack/router"} {
			target := base + "/config/targets/" + url.PathEscape(name)
			if code, body := doRequest(t, http.MethodDelete, target, ""); code != http.StatusOK {
				t.Fatalf("delete %q: status %d: %s", name, code, body)
			}
			code, body := doRequest(t, http.MethodGet, target, "")
			if code != http.StatusNotFound || !strings.Contains(body, "target "+name+" not found") {
				t.Fatalf("get %q after delete: status %d: %s", name, code, body)
			}
		}
		if tc := getConfigTarget(t, ts.URL, "rack%2Frouter"); tc.Name != "rack%2Frouter" {
			t.Fatalf("unrelated target removed: %+v", tc)
		}
	})
}

func TestServer_NonTargetPathParamsStillDecoded(t *testing.T) {
	s := newTestServer(t)
	if _, err := s.store.Config.Set("subscriptions", "my sub", &types.SubscriptionConfig{Name: "my sub"}); err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	code, body := doRequest(t, http.MethodGet, ts.URL+"/api/v1/config/subscriptions/my%20sub", "")
	if code != http.StatusOK || !strings.Contains(body, `"name":"my sub"`) {
		t.Fatalf("status %d: %s", code, body)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
