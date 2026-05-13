// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package apiserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/openconfig/gnmic/pkg/api/types"

	cluster_manager "github.com/openconfig/gnmic/pkg/collector/managers/cluster"
	inputs_manager "github.com/openconfig/gnmic/pkg/collector/managers/inputs"
	outputs_manager "github.com/openconfig/gnmic/pkg/collector/managers/outputs"
	targets_manager "github.com/openconfig/gnmic/pkg/collector/managers/targets"
	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/openconfig/gnmic/pkg/pipeline"
	"github.com/prometheus/client_golang/prometheus"
	zstore "github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	ctx := context.Background()
	cfg := gomap.NewMemStore(zstore.StoreOptions[any]{})
	if _, err := cfg.Set("global-flags", "global-flags", config.GlobalFlags{Encoding: "json"}); err != nil {
		t.Fatalf("seed global-flags: %v", err)
	}
	st := collstore.NewStore(cfg)
	pipeline := make(chan *pipeline.Msg, 8)
	reg := prometheus.NewRegistry()
	s := NewServer(
		st,
		targets_manager.NewTargetsManager(ctx, st, pipeline, reg),
		outputs_manager.NewOutputsManager(ctx, st, pipeline, reg),
		inputs_manager.NewInputsManager(ctx, st, pipeline),
		cluster_manager.NewClusterManager(st),
		reg,
	)
	s.logger = logging.DiscardLogger()
	return s
}

func TestSanitizeConfig(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]any
		want map[string]any
	}{
		{
			name: "empty",
			in:   map[string]any{},
			want: map[string]any{},
		},
		{
			name: "unwraps known nested map keys",
			in: map[string]any{
				"api-server": map[string]any{
					"api-server": map[string]any{"address": ":7890"},
				},
				"other": "keep",
			},
			want: map[string]any{
				"api-server": map[string]any{"address": ":7890"},
				"other":      "keep",
			},
		},
		{
			name: "non-map known keys unchanged",
			in: map[string]any{
				"global-flags": "scalar",
			},
			want: map[string]any{
				"global-flags": "scalar",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeConfig(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len got=%d want=%d\ngot=%#v", len(got), len(tt.want), got)
			}
			for k, wv := range tt.want {
				gv, ok := got[k]
				if !ok {
					t.Fatalf("missing key %q", k)
				}
				gb, _ := json.Marshal(gv)
				wb, _ := json.Marshal(wv)
				if string(gb) != string(wb) {
					t.Fatalf("key %q: got %s want %s", k, gb, wb)
				}
			}
		})
	}
}

func TestCreateListener_TCP(t *testing.T) {
	ln, err := createListener(&config.APIServer{Address: "127.0.0.1:0"})
	if err != nil {
		t.Fatalf("createListener: %v", err)
	}
	if err := ln.Close(); err != nil {
		t.Fatalf("close listener: %v", err)
	}
}

func TestServer_Routes_HealthzAndMetrics(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	t.Run("healthz", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/healthz")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), `"status":"healthy"`) {
			t.Fatalf("body: %s", body)
		}
	})

	t.Run("metrics", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/metrics")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "go_info") {
			t.Fatalf("expected prometheus go_info in body")
		}
	})
}

func TestServer_HandleConfigGet(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/api/v1/config")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestServer_HandleAdminShutdown(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	resp, err := http.Post(ts.URL+"/api/v1/admin/shutdown", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

func TestServer_Start_SkipsWhenNoAPIServerConfig(t *testing.T) {
	s := newTestServer(t)
	var wg sync.WaitGroup
	if err := s.Start(nil, &wg); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if s.srv != nil {
		t.Fatal("expected http.Server not started")
	}
}

func TestServer_Start_InvalidAPIServerConfigType(t *testing.T) {
	s := newTestServer(t)
	if _, err := s.store.Config.Set("api-server", "api-server", "not-a-server-config"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	var wg sync.WaitGroup
	err := s.Start(nil, &wg)
	if err == nil || !strings.Contains(err.Error(), "invalid api-server config") {
		t.Fatalf("Start err = %v", err)
	}
}

func TestServer_RequireClusteringReturns503WhenLockerNil(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/api/v1/cluster")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "clustering is not enabled") {
		t.Fatalf("body: %s", body)
	}
}

func TestServer_ConfigTargetsGet(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	t.Run("list empty", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/config/targets")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if strings.TrimSpace(string(body)) != "{}" {
			t.Fatalf("body %q", body)
		}
	})

	t.Run("get missing", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/config/targets/no-such-target")
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "target no-such-target not found") {
			t.Fatalf("body: %s", body)
		}
	})
}

func TestServer_ConfigTargetsPost(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	tests := []struct {
		name       string
		body       string
		wantCode   int
		wantSubstr string
	}{
		{
			name:       "invalid json",
			body:       `{`,
			wantCode:   http.StatusBadRequest,
			wantSubstr: "",
		},
		{
			name:       "missing name",
			body:       `{"address":"127.0.0.1:1"}`,
			wantCode:   http.StatusBadRequest,
			wantSubstr: "target name is required",
		},
		{
			name:       "missing address",
			body:       `{"name":"onlyname"}`,
			wantCode:   http.StatusBadRequest,
			wantSubstr: "target address is required",
		},
		{
			name:       "unknown subscription ref",
			body:       `{"name":"t1","address":"127.0.0.1:1","subscriptions":["missing-sub"]}`,
			wantCode:   http.StatusNotFound,
			wantSubstr: "subscription missing-sub not found",
		},
		{
			name:       "unknown output ref",
			body:       `{"name":"t2","address":"127.0.0.1:1","outputs":["missing-out"]}`,
			wantCode:   http.StatusNotFound,
			wantSubstr: "output missing-out not found",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/config/targets", strings.NewReader(tt.body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.wantCode {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("status %d body %s", resp.StatusCode, body)
			}
			if tt.wantSubstr != "" {
				body, _ := io.ReadAll(resp.Body)
				if !strings.Contains(string(body), tt.wantSubstr) {
					t.Fatalf("body %q want substring %q", body, tt.wantSubstr)
				}
			}
		})
	}
}

func TestServer_ConfigTargetsPostMinimalRoundTrip(t *testing.T) {
	s := newTestServer(t)
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	body := `{"name":"rt1","address":"127.0.0.1:9"}`
	req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/config/targets", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST status %d", resp.StatusCode)
	}

	resp2, err := http.Get(ts.URL + "/api/v1/config/targets/rt1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("GET status %d", resp2.StatusCode)
	}
	var got types.TargetConfig
	if err := json.NewDecoder(resp2.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "rt1" || got.Address != "127.0.0.1:9" {
		t.Fatalf("got %+v", got)
	}
}

func TestServer_ConfigTargetsPatch(t *testing.T) {
	s := newTestServer(t)
	if _, err := s.store.Config.Set("targets", "pt1", &types.TargetConfig{Name: "pt1", Address: "127.0.0.1:1"}); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	t.Run("subscriptions missing target", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/config/targets/ghost/subscriptions", strings.NewReader(`[]`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "target ghost not found") {
			t.Fatalf("body: %s", body)
		}
	})

	t.Run("subscriptions bad json", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/config/targets/pt1/subscriptions", strings.NewReader(`{`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})

	t.Run("outputs bad json", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/config/targets/pt1/outputs", strings.NewReader(`{`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})

	t.Run("subscriptions unknown subscription", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/config/targets/pt1/subscriptions", strings.NewReader(`["nope"]`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "subscription nope not found") {
			t.Fatalf("body: %s", body)
		}
	})
}

func TestServer_ConfigTargetsDelete(t *testing.T) {
	s := newTestServer(t)
	if _, err := s.store.Config.Set("targets", "dt1", &types.TargetConfig{Name: "dt1", Address: "127.0.0.1:1"}); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	ts := httptest.NewServer(s.router)
	t.Cleanup(ts.Close)

	req, err := http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/config/targets/dt1", nil)
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
}
