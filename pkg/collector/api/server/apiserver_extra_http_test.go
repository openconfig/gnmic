// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/openconfig/gnmic/pkg/config"
	_ "github.com/openconfig/gnmic/pkg/inputs/all"
	_ "github.com/openconfig/gnmic/pkg/outputs/all"
)

func TestServer_ConfigApply(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	t.Run("empty gzip body", func(t *testing.T) {
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		if _, err := gw.Write([]byte("{}")); err != nil {
			t.Fatal(err)
		}
		if err := gw.Close(); err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/config/apply", &buf)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("status %d: %s", resp.StatusCode, body)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		resp, err := http.Post(ts.URL+"/api/v1/config/apply", "application/json", strings.NewReader(`{`))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})

	t.Run("validate targets without subscriptions", func(t *testing.T) {
		body := `{"targets":{"t1":{"name":"t1","address":"127.0.0.1:1"}}}`
		resp, err := http.Post(ts.URL+"/api/v1/config/apply", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %d", resp.StatusCode)
		}
		b, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(b), "validate error") {
			t.Fatalf("body: %s", b)
		}
	})
}

func TestServer_ConfigSubscriptionsHTTP(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	t.Run("list empty", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/config/subscriptions")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("get missing", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/config/subscriptions/nope")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("post invalid json", func(t *testing.T) {
		resp, err := http.Post(ts.URL+"/api/v1/config/subscriptions", "application/json", strings.NewReader(`{`))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("post minimal", func(t *testing.T) {
		body := `{"name":"sapi","paths":["/"],"mode":"STREAM","stream-mode":"TARGET_DEFINED"}`
		resp, err := http.Post(ts.URL+"/api/v1/config/subscriptions", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("status %d: %s", resp.StatusCode, b)
		}
	})
	t.Run("delete missing", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/config/subscriptions/ghost-sub", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
}

func TestServer_ConfigOutputsHTTP(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	t.Run("list empty", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/config/outputs")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("get missing", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/config/outputs/missing-out")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("post unknown type", func(t *testing.T) {
		body := `{"name":"o1","type":"not-a-real-output","file-type":"stdout"}`
		resp, err := http.Post(ts.URL+"/api/v1/config/outputs", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("post file stdout", func(t *testing.T) {
		body := `{"name":"fout1","type":"file","file-type":"stdout","format":"json"}`
		resp, err := http.Post(ts.URL+"/api/v1/config/outputs", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("status %d: %s", resp.StatusCode, b)
		}
	})
	t.Run("processors patch not implemented", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/config/outputs/fout1/processors", strings.NewReader(`[]`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotImplemented {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("delete", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/config/outputs/fout1", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
}

func TestServer_ConfigInputsHTTP(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	t.Run("get missing", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/config/inputs/nope")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("post unknown type", func(t *testing.T) {
		body := `{"name":"i1","type":"not-an-input"}`
		resp, err := http.Post(ts.URL+"/api/v1/config/inputs", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("processors patch not implemented", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/config/inputs/x/processors", strings.NewReader(`[]`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotImplemented {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
}

func TestServer_ConfigProcessorsHTTP(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	t.Run("list empty", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/config/processors")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("get missing", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/config/processors/nope")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("post minimal", func(t *testing.T) {
		body := `{"name":"p1","type":"event-drop","config":{}}`
		resp, err := http.Post(ts.URL+"/api/v1/config/processors", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("status %d: %s", resp.StatusCode, b)
		}
	})
	t.Run("get single", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/config/processors/p1")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("delete", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/config/processors/p1", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
}

func TestServer_AssignmentsHTTP(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	t.Run("post invalid json", func(t *testing.T) {
		resp, err := http.Post(ts.URL+"/api/v1/assignments", "application/json", strings.NewReader(`{`))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("post validate empty", func(t *testing.T) {
		resp, err := http.Post(ts.URL+"/api/v1/assignments", "application/json", strings.NewReader(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("post ok", func(t *testing.T) {
		body := `{"assignments":[{"target":"at1","member":"m1"}]}`
		resp, err := http.Post(ts.URL+"/api/v1/assignments", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("status %d: %s", resp.StatusCode, b)
		}
	})
	t.Run("get list", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/assignments")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("delete missing", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/assignments/ghost", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
}

func TestServer_RuntimeTargetsHTTP(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	t.Run("list", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/targets")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if strings.TrimSpace(string(body)) != "[]" && !strings.HasPrefix(strings.TrimSpace(string(body)), "[") {
			t.Fatalf("body %q", body)
		}
	})
	t.Run("get missing", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/targets/nope")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("state post missing target", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/targets/nope/state/running", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
}

func TestServer_ConfigTunnelTargetMatchesHTTP(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	t.Run("list empty", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/config/tunnel-target-matches")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("get missing", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/config/tunnel-target-matches/nope")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("post invalid json", func(t *testing.T) {
		resp, err := http.Post(ts.URL+"/api/v1/config/tunnel-target-matches", "application/json", strings.NewReader(`{`))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
	t.Run("post minimal", func(t *testing.T) {
		body := `{"id":"ttm1","type":"regex"}`
		resp, err := http.Post(ts.URL+"/api/v1/config/tunnel-target-matches", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("status %d: %s", resp.StatusCode, b)
		}
	})
	t.Run("delete", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/config/tunnel-target-matches/ttm1", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
}

func TestServer_StartStop_TCPListener(t *testing.T) {
	s := newTestServer(t)
	if _, err := s.store.Config.Set("api-server", "api-server", &config.APIServer{Address: "127.0.0.1:0"}); err != nil {
		t.Fatalf("Set api-server: %v", err)
	}
	var wg sync.WaitGroup
	if err := s.Start(nil, &wg); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if s.srv == nil {
		t.Fatal("expected http.Server")
	}
	s.Stop()
}
