// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	zstore "github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
	"google.golang.org/protobuf/encoding/prototext"

	"github.com/openconfig/gnmic/pkg/api"
	"github.com/openconfig/gnmic/pkg/api/testutils"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/cache"
	"github.com/openconfig/gnmic/pkg/logging"
)

var createGetRequestTestSet = map[string]struct {
	in  *Config
	out *gnmi.GetRequest
	err error
}{
	"nil_input": {
		in:  nil,
		out: nil,
		err: ErrInvalidConfig,
	},
	"unknown_encoding_type": {
		in: &Config{
			GlobalFlags{
				Encoding: "dummy",
			},
			LocalFlags{},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: nil,
		err: api.ErrInvalidValue,
	},
	"invalid_prefix": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{
				GetPrefix: "/invalid/]prefix",
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: nil,
		err: api.ErrInvalidValue,
	},
	"invalid_path": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{
				GetPrefix: "/invalid/]path",
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: nil,
		err: api.ErrInvalidValue,
	},
	"unknown_data_type": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{
				GetPrefix: "/valid/path",
				GetType:   "dummy",
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: nil,
		err: api.ErrInvalidValue,
	},
	"basic_get_request": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{
				GetPath: []string{"/valid/path"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.GetRequest{
			Path: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{Name: "valid"},
						{Name: "path"},
					},
				},
			},
		},
		err: nil,
	},
	"get_request_with_type": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{
				GetPath: []string{"/valid/path"},
				GetType: "state",
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.GetRequest{
			Path: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{Name: "valid"},
						{Name: "path"},
					},
				},
			},
			Type: gnmi.GetRequest_STATE,
		},
		err: nil,
	},
	"get_request_with_encoding": {
		in: &Config{
			GlobalFlags{
				Encoding: "proto",
			},
			LocalFlags{
				GetPath: []string{"/valid/path"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.GetRequest{
			Path: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{Name: "valid"},
						{Name: "path"},
					},
				},
			},
			Encoding: gnmi.Encoding_PROTO,
		},
		err: nil,
	},
	"get_request_with_prefix": {
		in: &Config{
			GlobalFlags{
				Encoding: "proto",
			},
			LocalFlags{
				GetPrefix: "/valid/prefix",
				GetPath:   []string{"/valid/path"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.GetRequest{
			Prefix: &gnmi.Path{
				Elem: []*gnmi.PathElem{
					{Name: "valid"},
					{Name: "prefix"},
				},
			},
			Path: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{Name: "valid"},
						{Name: "path"},
					},
				},
			},
			Encoding: gnmi.Encoding_PROTO,
		},
		err: nil,
	},
	"get_request_with_2_paths": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{
				GetPath: []string{
					"/valid/path1",
					"/valid/path2",
				},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.GetRequest{
			Path: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{Name: "valid"},
						{Name: "path1"},
					},
				},
				{
					Elem: []*gnmi.PathElem{
						{Name: "valid"},
						{Name: "path2"},
					},
				},
			},
		},
		err: nil,
	},
}

var createSetRequestTestSet = map[string]struct {
	in  *Config
	out *gnmi.SetRequest
	err error
}{

	"set_update_request": {
		in: &Config{
			GlobalFlags{},
			LocalFlags{
				SetDelimiter: ":::",
				SetUpdate:    []string{"/valid/path:::json:::value"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.SetRequest{
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte("\"value\""),
						},
					},
				},
			},
		},
		err: nil,
	},
	"set_replace_request": {
		in: &Config{
			GlobalFlags{},
			LocalFlags{
				SetDelimiter: ":::",
				SetReplace:   []string{"/valid/path:::json:::value"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.SetRequest{
			Replace: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte("\"value\""),
						},
					},
				},
			},
		},
		err: nil,
	},
	"set_delete_request": {
		in: &Config{
			GlobalFlags{},
			LocalFlags{
				SetDelete: []string{"/valid/path"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.SetRequest{
			Delete: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{Name: "valid"},
						{Name: "path"},
					},
				},
			},
		},
		err: nil,
	},
	"set_multiple_update_request": {
		in: &Config{
			GlobalFlags{},
			LocalFlags{
				SetDelimiter: ":::",
				SetUpdate: []string{
					"/valid/path1:::json:::value1",
					"/valid/path2:::json_ietf:::value2",
				},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.SetRequest{
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path1"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte("\"value1\""),
						},
					},
				},
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path2"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonIetfVal{
							JsonIetfVal: []byte("\"value2\""),
						},
					},
				},
			},
		},
		err: nil,
	},
	"set_multiple_replace_request": {
		in: &Config{
			GlobalFlags{},
			LocalFlags{
				SetDelimiter: ":::",
				SetReplace: []string{
					"/valid/path1:::json:::value1",
					"/valid/path2:::json_ietf:::value2",
				},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.SetRequest{
			Replace: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path1"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte("\"value1\""),
						},
					},
				},
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path2"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonIetfVal{
							JsonIetfVal: []byte("\"value2\""),
						},
					},
				},
			},
		},
		err: nil,
	},
	"set_multiple_delete_request": {
		in: &Config{
			GlobalFlags{},
			LocalFlags{
				SetDelete: []string{
					"/valid/path1",
					"/valid/path2",
				},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.SetRequest{
			Delete: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{Name: "valid"},
						{Name: "path1"},
					},
				},
				{
					Elem: []*gnmi.PathElem{
						{Name: "valid"},
						{Name: "path2"},
					},
				},
			},
		},
		err: nil,
	},
	"set_combined_request": {
		in: &Config{
			GlobalFlags{},
			LocalFlags{
				SetDelimiter: ":::",
				SetUpdate:    []string{"/valid/path1:::json:::value1"},
				SetReplace:   []string{"/valid/path2:::json:::value2"},
				SetDelete:    []string{"/valid/path"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.SetRequest{
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path1"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte("\"value1\""),
						},
					},
				},
			},
			Replace: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path2"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte("\"value2\""),
						},
					},
				},
			},
			Delete: []*gnmi.Path{
				{
					Elem: []*gnmi.PathElem{
						{Name: "valid"},
						{Name: "path"},
					},
				},
			},
		},
		err: nil,
	},
	"set_update_path_request": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{
				SetUpdatePath:  []string{"/valid/path"},
				SetUpdateValue: []string{"value"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.SetRequest{
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte("\"value\""),
						},
					},
				},
			},
		},
		err: nil,
	},
	"set_replace_path_request": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{
				SetReplacePath:  []string{"/valid/path"},
				SetReplaceValue: []string{"value"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.SetRequest{
			Replace: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte("\"value\""),
						},
					},
				},
			},
		},
		err: nil,
	},
	"set_union_replace_path_request": {
		in: &Config{
			GlobalFlags{
				Encoding: "json",
			},
			LocalFlags{
				SetUnionReplacePath:  []string{"/valid/path"},
				SetUnionReplaceValue: []string{"value"},
			},
			nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		},
		out: &gnmi.SetRequest{
			UnionReplace: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Elem: []*gnmi.PathElem{
							{Name: "valid"},
							{Name: "path"},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{
							JsonVal: []byte("\"value\""),
						},
					},
				},
			},
		},
		err: nil,
	},
}

var execPathTemplateTestSet = map[string]struct {
	tpl   string
	input interface{}
	out   string
}{
	"nil": {
		tpl:   "",
		input: nil,
		out:   "",
	},
	"simple": {
		tpl:   `"/path/"`,
		input: nil,
		out:   "/path/",
	},
	"with_an_expression": {
		tpl: `"/interfaces/" + .name`,
		input: map[string]interface{}{
			"name": "interface",
		},
		out: "/interfaces/interface",
	},
}

func TestCreateGetRequest(t *testing.T) {
	for name, data := range createGetRequestTestSet {
		t.Run(name, func(t *testing.T) {
			getReq, err := data.in.CreateGetRequest(&types.TargetConfig{})
			t.Logf("exp value: %+v", data.out)
			t.Logf("got value: %+v", getReq)
			t.Logf("exp error: %+v", data.err)
			t.Logf("got error: %+v", err)
			if err != nil {
				uerr := errors.Unwrap(err)
				if !errors.Is(uerr, data.err) {
					t.Fail()
				}
			}
			if !testutils.GetRequestsEqual(getReq, data.out) {
				t.Fail()
			}
		})
	}
}

func TestCreateSetRequest(t *testing.T) {
	for name, data := range createSetRequestTestSet {
		t.Run(name, func(t *testing.T) {
			setReq, err := data.in.CreateSetRequest("")
			t.Logf("exp value: %+v", data.out)
			t.Logf("exp error: %+v", data.err)
			t.Logf("got value: %+v", setReq)
			t.Logf("got error: %+v", err)
			if err != nil {
				if !strings.HasPrefix(err.Error(), data.err.Error()) {
					t.Fail()
				}
			}
			if !testutils.SetRequestsEqual(setReq[0], data.out) {
				t.Fail()
			}
		})
	}
}

func TestExecPathTemplate(t *testing.T) {
	c := New()
	c.Debug = true
	c.internalLog = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	for name, data := range execPathTemplateTestSet {
		t.Run(name, func(t *testing.T) {
			o, err := c.execPathTemplate(data.tpl, data.input)
			if err != nil {
				t.Logf("failed: %v", err)
				t.Fail()
			}
			t.Logf("exp value: %+v", data.out)
			t.Logf("got value: %+v", o)
			if data.out != o {
				t.Fail()
			}
		})
	}
}

func newConfigFromYAML(t *testing.T, body string) *Config {
	t.Helper()
	c := New()
	c.FileConfig.SetConfigType("yaml")
	if err := c.FileConfig.ReadConfig(bytes.NewBufferString(body)); err != nil {
		t.Fatalf("failed reading yaml config: %v", err)
	}
	return c
}

func TestLoggingFlags(t *testing.T) {
	g := GlobalFlags{
		Log:           true,
		Debug:         true,
		LogFile:       "gnmic.log",
		LogMaxSize:    10,
		LogMaxBackups: 3,
		LogCompress:   true,
	}
	if got, want := g.LoggingFlags(), (logging.Flags{
		Log:           true,
		Debug:         true,
		LogFile:       "gnmic.log",
		LogMaxSize:    10,
		LogMaxBackups: 3,
		LogCompress:   true,
	}); got != want {
		t.Fatalf("LoggingFlags() = %+v, want %+v", got, want)
	}
}

func TestConfigUtilityFunctions(t *testing.T) {
	if got := SanitizeArrayFlagValue([]string{"[a]", "[[b]]", "[]", "c"}); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("SanitizeArrayFlagValue() = %#v", got)
	}
	if got := ParseAddressField([]string{"[a,b]", "c", "[]"}); !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Fatalf("ParseAddressField() = %#v", got)
	}
	if got := trimQuotes(`"quoted"`); got != "quoted" {
		t.Fatalf("trimQuotes() = %q", got)
	}
	if got := trimQuotes(`not-quoted`); got != "not-quoted" {
		t.Fatalf("trimQuotes(non quoted) = %q", got)
	}

	jsonBytes, err := toJSONBytes([]byte("a: 1\nb:\n  c: two\n"))
	if err != nil {
		t.Fatalf("toJSONBytes failed: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(jsonBytes, &decoded); err != nil {
		t.Fatalf("toJSONBytes returned invalid json: %v", err)
	}
	if decoded["a"] != float64(1) || decoded["b"].(map[string]any)["c"] != "two" {
		t.Fatalf("unexpected decoded json: %#v", decoded)
	}
}

func TestExpandOSPathHelpers(t *testing.T) {
	tmp := t.TempDir()
	file := filepath.Join(tmp, "input.json")
	if err := os.WriteFile(file, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := ExpandOSPaths([]string{file})
	if err != nil {
		t.Fatalf("ExpandOSPaths failed: %v", err)
	}
	if got[0] != file {
		t.Fatalf("ExpandOSPaths() = %q, want %q", got[0], file)
	}
	if got, err := expandOSPath("https://example.com/config.yaml"); err != nil || got != "https://example.com/config.yaml" {
		t.Fatalf("expandOSPath(url) = %q, %v", got, err)
	}
	if got, err := expandOSPath("-"); err != nil || got != "-" {
		t.Fatalf("expandOSPath(stdin) = %q, %v", got, err)
	}
}

func TestValidateSetInput(t *testing.T) {
	t.Run("missing input", func(t *testing.T) {
		if err := New().ValidateSetInput(); err == nil || !strings.Contains(err.Error(), "no paths") {
			t.Fatalf("ValidateSetInput() = %v, want no paths error", err)
		}
	})
	t.Run("conflicting update file and value", func(t *testing.T) {
		c := New()
		c.SetUpdatePath = []string{"/a"}
		c.SetUpdateFile = []string{"-"}
		c.SetUpdateValue = []string{"1"}
		if err := c.ValidateSetInput(); err == nil || !strings.Contains(err.Error(), "update from file and value") {
			t.Fatalf("ValidateSetInput() = %v, want conflict error", err)
		}
	})
	t.Run("matching update path and value", func(t *testing.T) {
		c := New()
		c.SetUpdatePath = []string{"[/a]"}
		c.SetUpdateValue = []string{"[1]"}
		if err := c.ValidateSetInput(); err != nil {
			t.Fatalf("ValidateSetInput() failed: %v", err)
		}
		if got := c.SetUpdatePath; !reflect.DeepEqual(got, []string{"/a"}) {
			t.Fatalf("SetUpdatePath after sanitize = %#v", got)
		}
	})
}

func TestEnvironmentHelpers(t *testing.T) {
	t.Setenv("GNMIC_API_SERVER_ADDRESS", ":9999")
	t.Setenv("GNMIC_OUTPUTS_FILE_TYPE", "file")
	t.Setenv("OTHER_VAR", "ignored")

	got := envToMap()
	apiServer := got["api"].(map[string]any)["server"].(map[string]any)
	if apiServer["address"] != ":9999" {
		t.Fatalf("envToMap api-server address = %#v", apiServer)
	}
	outputs := got["outputs"].(map[string]any)["file"].(map[string]any)
	if outputs["type"] != "file" {
		t.Fatalf("envToMap output type = %#v", outputs)
	}

	m := map[string]any{}
	mergeMap(m, []string{"a", "b", "c"}, "v")
	if m["a"].(map[string]any)["b"].(map[string]any)["c"] != "v" {
		t.Fatalf("mergeMap nested = %#v", m)
	}

	t.Setenv("EXPAND_ME", "expanded")
	exp := map[string]any{
		"plain": "${EXPAND_ME}",
		"skip":  "${EXPAND_ME}",
		"nested": map[string]any{
			"plain": "${EXPAND_ME}",
		},
		"slice": []any{"${EXPAND_ME}", []any{"${EXPAND_ME}"}},
	}
	expandMapEnv(exp, expandExcept("skip"))
	if exp["plain"] != "expanded" || exp["skip"] != "${EXPAND_ME}" {
		t.Fatalf("expandMapEnv top-level = %#v", exp)
	}
	if exp["nested"].(map[string]any)["plain"] != "expanded" {
		t.Fatalf("expandMapEnv nested = %#v", exp)
	}
	if got := exp["slice"].([]any)[1].([]any)[0]; got != "expanded" {
		t.Fatalf("expandSliceEnv nested = %#v", exp["slice"])
	}
}

func TestGetAPIServer(t *testing.T) {
	t.Setenv("API_ADDR", ":9999")
	c := newConfigFromYAML(t, `
api-server:
  address: ${API_ADDR}
  timeout: 3s
  enable-metrics: "true"
  enable-profiling: "true"
  debug: "true"
  healthz-disable-logging: "true"
`)
	if err := c.GetAPIServer(); err != nil {
		t.Fatalf("GetAPIServer() failed: %v", err)
	}
	if c.APIServer.Address != ":9999" ||
		c.APIServer.Timeout != 3*time.Second ||
		!c.APIServer.EnableMetrics ||
		!c.APIServer.EnableProfiling ||
		!c.APIServer.Debug ||
		!c.APIServer.HealthzDisableLogging {
		t.Fatalf("unexpected API server config: %+v", c.APIServer)
	}

	c = New()
	c.APIServer = &APIServer{}
	c.setAPIServerDefaults()
	if c.APIServer.Address != defaultAPIServerAddress || c.APIServer.Timeout != defaultAPIServerTimeout {
		t.Fatalf("API defaults not set: %+v", c.APIServer)
	}

	c = New()
	c.API = ":1234"
	c.FileConfig.Set("api", ":1234")
	if err := c.GetAPIServer(); err != nil {
		t.Fatalf("GetAPIServer(api flag) failed: %v", err)
	}
	if c.APIServer.Address != ":1234" {
		t.Fatalf("APIServer address from api flag = %q", c.APIServer.Address)
	}
}

func TestGetGNMIServer(t *testing.T) {
	c := newConfigFromYAML(t, `
gnmi-server:
  address: :57401
  max-subscriptions: "9"
  max-unary-rpc: "7"
  enable-metrics: "true"
  debug: "true"
  timeout: 4s
  service-registration:
    name: gnmic
    check-interval: 2s
  cache:
    type: redis
    address: localhost:6379
    timeout: 2s
`)
	if err := c.GetGNMIServer(); err != nil {
		t.Fatalf("GetGNMIServer() failed: %v", err)
	}
	if c.GnmiServer.Address != ":57401" ||
		c.GnmiServer.MaxSubscriptions != 9 ||
		c.GnmiServer.MaxUnaryRPC != 7 ||
		!c.GnmiServer.EnableMetrics ||
		!c.GnmiServer.Debug ||
		c.GnmiServer.Timeout != 4*time.Second {
		t.Fatalf("unexpected gnmi server config: %+v", c.GnmiServer)
	}
	if sr := c.GnmiServer.ServiceRegistration; sr.Address != defaultServiceRegistrationAddress ||
		sr.CheckInterval != defaultRegistrationCheckInterval ||
		sr.MaxFail != defaultMaxServiceFail ||
		sr.DeregisterAfter != (defaultRegistrationCheckInterval*time.Duration(defaultMaxServiceFail)).String() {
		t.Fatalf("unexpected service registration defaults: %+v", sr)
	}
	if c.GnmiServer.Cache.Type != cache.CacheType("redis") || c.GnmiServer.Cache.Address != "localhost:6379" {
		t.Fatalf("unexpected cache config: %+v", c.GnmiServer.Cache)
	}

	c = New()
	c.GnmiServer = &GNMIServer{}
	c.setGnmiServerDefaults()
	if c.GnmiServer.Address != defaultAddress ||
		c.GnmiServer.MaxSubscriptions != defaultMaxSubscriptions ||
		c.GnmiServer.MaxUnaryRPC != defaultMaxUnaryRPC ||
		c.GnmiServer.MinSampleInterval != minimumSampleInterval ||
		c.GnmiServer.DefaultSampleInterval != defaultSampleInterval ||
		c.GnmiServer.MinHeartbeatInterval != minimumHeartbeatInterval {
		t.Fatalf("gnmi server defaults not set: %+v", c.GnmiServer)
	}

	kp := (&grpcKeepaliveConfig{Time: 5 * time.Second, Timeout: time.Second}).Convert()
	if kp.Time != 5*time.Second || kp.Timeout != time.Second {
		t.Fatalf("keepalive conversion = %+v", kp)
	}
	if (*grpcKeepaliveConfig)(nil).Convert() != nil {
		t.Fatalf("nil keepalive config should convert to nil")
	}
}

func TestGetTunnelServer(t *testing.T) {
	c := newConfigFromYAML(t, `
tunnel-server:
  enable-metrics: "true"
  debug: "true"
  target-wait-time: 5s
  targets:
    - type: gnmi
      id: dev-.*
      config:
        address: 127.0.0.1:57400
`)
	if err := c.GetTunnelServer(); err != nil {
		t.Fatalf("GetTunnelServer() failed: %v", err)
	}
	if c.TunnelServer.Address != defaultAddress ||
		c.TunnelServer.TargetWaitTime != 5*time.Second ||
		!c.TunnelServer.EnableMetrics ||
		!c.TunnelServer.Debug ||
		len(c.TunnelServer.Targets) != 1 ||
		c.TunnelServer.Targets[0].Config.Address != "127.0.0.1:57400" {
		t.Fatalf("unexpected tunnel server config: %+v", c.TunnelServer)
	}

	c = newConfigFromYAML(t, `tunnel-server: {targets: bad}`)
	if err := c.GetTunnelServer(); err == nil || !strings.Contains(err.Error(), "unexpected target") {
		t.Fatalf("GetTunnelServer() = %v, want target type error", err)
	}
}

func TestGetClusteringAndLocker(t *testing.T) {
	t.Setenv("LOCK_ADDR", "127.0.0.1:6379")
	c := newConfigFromYAML(t, `
clustering:
  cluster-name: cluster-a
  instance-name: inst-a
  service-address: svc
  tags: ["one", "${LOCK_ADDR}"]
  targets-watch-timer: 1s
  target-assignment-timeout: 1s
  services-watch-timer: 1s
  leader-wait-timer: 1s
  locker:
    type: redis
    address: ${LOCK_ADDR}
`)
	if err := c.GetClustering(); err != nil {
		t.Fatalf("GetClustering() failed: %v", err)
	}
	if c.ClusterName != "cluster-a" || c.InstanceName != "inst-a" {
		t.Fatalf("global cluster fields not synced: cluster=%q instance=%q", c.ClusterName, c.InstanceName)
	}
	if c.Clustering.TargetsWatchTimer != minTargetWatchTimer ||
		c.Clustering.TargetAssignmentTimeout != defaultTargetAssignmentTimeout ||
		c.Clustering.ServicesWatchTimer != defaultServicesWatchTimer ||
		c.Clustering.LeaderWaitTimer != defaultLeaderWaitTimer {
		t.Fatalf("clustering timers not defaulted: %+v", c.Clustering)
	}
	if c.Clustering.Locker["address"] != "127.0.0.1:6379" {
		t.Fatalf("locker env not expanded: %+v", c.Clustering.Locker)
	}

	c = newConfigFromYAML(t, `clustering: {locker: {type: unknown}}`)
	if err := c.GetClustering(); err == nil || !strings.Contains(err.Error(), "unknown locker type") {
		t.Fatalf("GetClustering() = %v, want unknown locker error", err)
	}
}

func TestGetLoaderAndPlugins(t *testing.T) {
	c := New()
	c.TargetsFile = "targets.yaml"
	if err := c.GetLoader(); err != nil {
		t.Fatalf("GetLoader(targets-file) failed: %v", err)
	}
	if c.Loader["type"] != "file" || c.Loader["path"] != "targets.yaml" {
		t.Fatalf("loader from targets-file = %#v", c.Loader)
	}

	t.Setenv("LOADER_ADDR", "http://example")
	c = newConfigFromYAML(t, `
loader:
  type: http
  url: ${LOADER_ADDR}
  password: literal-password
`)
	if err := c.GetLoader(); err != nil {
		t.Fatalf("GetLoader() failed: %v", err)
	}
	if c.Loader["url"] != "http://example" || c.Loader["password"] != "literal-password" {
		t.Fatalf("loader env handling = %#v", c.Loader)
	}

	c = newConfigFromYAML(t, `loader: {type: unknown}`)
	if err := c.GetLoader(); err == nil || !strings.Contains(err.Error(), "unknown loader type") {
		t.Fatalf("GetLoader() = %v, want unknown type error", err)
	}

	c = newConfigFromYAML(t, `
plugins:
  path: /plugins
  start-timeout: 10s
  debug: true
`)
	pc, err := c.GetPluginsConfig()
	if err != nil {
		t.Fatalf("GetPluginsConfig() failed: %v", err)
	}
	if pc.Path != "/plugins" || pc.Glob != "*" || pc.StartTimeout != 10*time.Second || !pc.Debug {
		t.Fatalf("unexpected plugin config: %+v", pc)
	}
}

func TestGetActionsAndInputs(t *testing.T) {
	t.Setenv("SCRIPT_CMD", "echo ok")
	c := newConfigFromYAML(t, `
actions:
  a1:
    type: script
    command: ${SCRIPT_CMD}
  a2:
    type: template
    template: ${SCRIPT_CMD}
inputs:
  in1:
    type: nats
    address: ${SCRIPT_CMD}
format: event
`)
	c.Actions = make(map[string]map[string]any)
	c.Inputs = make(map[string]map[string]any)
	acts, err := c.GetActions()
	if err != nil {
		t.Fatalf("GetActions() failed: %v", err)
	}
	if acts["a1"]["name"] != "a1" || acts["a1"]["command"] != "echo ok" {
		t.Fatalf("unexpected action config: %#v", acts)
	}
	if acts["a2"]["template"] != "${SCRIPT_CMD}" {
		t.Fatalf("template action should not expand template body: %#v", acts["a2"])
	}
	ins, err := c.GetInputs()
	if err != nil {
		t.Fatalf("GetInputs() failed: %v", err)
	}
	if ins["in1"]["format"] != "event" || ins["in1"]["address"] != "echo ok" {
		t.Fatalf("unexpected input config: %#v", ins)
	}

	c = newConfigFromYAML(t, `actions: {bad: {type: nope}}`)
	c.Actions = make(map[string]map[string]any)
	if _, err := c.GetActions(); err == nil || !strings.Contains(err.Error(), "unknown action type") {
		t.Fatalf("GetActions() = %v, want unknown action error", err)
	}

	c = newConfigFromYAML(t, `inputs: {bad: {type: nope}}`)
	c.Inputs = make(map[string]map[string]any)
	if _, err := c.GetInputs(); err == nil || !strings.Contains(err.Error(), "unknown input type") {
		t.Fatalf("GetInputs() = %v, want unknown input error", err)
	}
}

func TestOutputsHelpers(t *testing.T) {
	c := newConfigFromYAML(t, `
outputs:
  z:
    - type: file
    - type: nats
  a:
    - type: kafka
`)
	suggestions := c.GetOutputsSuggestions()
	if len(suggestions) != 2 ||
		suggestions[0].Name != "a" ||
		!reflect.DeepEqual(suggestions[0].Types, []string{"kafka"}) ||
		suggestions[1].Name != "z" ||
		!reflect.DeepEqual(suggestions[1].Types, []string{"file", "nats"}) {
		t.Fatalf("GetOutputsSuggestions() = %#v", suggestions)
	}
	configs := c.GetOutputsConfigs()
	if len(configs) != 2 {
		t.Fatalf("GetOutputsConfigs() len = %d", len(configs))
	}
}

func TestToStore(t *testing.T) {
	c := New()
	c.Encoding = "json"
	c.Targets = map[string]*types.TargetConfig{
		"t1": {Name: "t1", Address: "127.0.0.1:57400"},
	}
	c.Subscriptions = map[string]*types.SubscriptionConfig{
		"s1": {Name: "s1", Paths: []string{"/interfaces"}},
	}
	c.Processors = map[string]map[string]any{"p1": {"type": "event-drop"}}
	c.Outputs = map[string]map[string]any{"o1": {"type": "file"}}
	c.Inputs = map[string]map[string]any{"i1": {"type": "nats"}}
	c.Actions = map[string]map[string]any{"a1": {"type": "script"}}
	c.Clustering = &Clustering{ClusterName: "cluster"}
	c.GnmiServer = &GNMIServer{Address: ":57400"}
	c.APIServer = &APIServer{Address: ":7890"}
	c.Loader = map[string]any{"type": "file"}
	c.TunnelServer = &TunnelServer{Address: ":57401"}

	st := gomap.NewMemStore(zstore.StoreOptions[any]{})
	if err := c.ToStore(st); err != nil {
		t.Fatalf("ToStore() failed: %v", err)
	}
	if got, ok, err := st.Get("targets", "t1"); err != nil || !ok || got.(*types.TargetConfig).Address != "127.0.0.1:57400" {
		t.Fatalf("stored target = %#v, ok=%v, err=%v", got, ok, err)
	}
	if got, ok, err := st.Get("global-flags", "global-flags"); err != nil || !ok || got.(GlobalFlags).Encoding != "json" {
		t.Fatalf("stored globals = %#v, ok=%v, err=%v", got, ok, err)
	}
}

func TestFlagsFromFile(t *testing.T) {
	c := newConfigFromYAML(t, `
debug: true
log: true
address: [a, b]
get-path: [/interfaces]
`)
	cmd := &cobra.Command{Use: "get"}
	cmd.PersistentFlags().Bool("debug", false, "")
	cmd.PersistentFlags().Bool("log", false, "")
	cmd.PersistentFlags().StringSlice("address", nil, "")
	cmd.Flags().StringSlice("path", nil, "")

	c.SetPersistentFlagsFromFile(cmd)
	c.SetLocalFlagsFromFile(cmd)
	c.setFlagValue(cmd, "debug", true)
	c.setFlagValue(cmd, "address", []any{"a", "b"})

	debug, _ := cmd.Flags().GetBool("debug")
	address, _ := cmd.Flags().GetStringSlice("address")
	path, _ := cmd.Flags().GetStringSlice("path")
	if !debug || !reflect.DeepEqual(address, []string{"a", "b"}) || !reflect.DeepEqual(path, []string{"/interfaces"}) {
		t.Fatalf("flags not set from file: debug=%v address=%v path=%v", debug, address, path)
	}
	if !flagIsSet(cmd, "path") || flagIsSet(nil, "path") {
		t.Fatalf("flagIsSet returned unexpected result")
	}
}

func TestReadFileAndSetRequestTemplate(t *testing.T) {
	dir := t.TempDir()
	requestFile := filepath.Join(dir, "request.yaml")
	varsFile := filepath.Join(dir, "request.vars.yaml")
	if err := os.WriteFile(requestFile, []byte(`
updates:
  - path: /system/config/hostname
    value: '{{ index .Vars "hostname" }}'
deletes:
  - /interfaces/interface[name={{ .TargetName }}]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(varsFile, []byte(`hostname: leaf1`), 0o600); err != nil {
		t.Fatal(err)
	}

	raw, err := readFile(varsFile)
	if err != nil {
		t.Fatalf("readFile(yaml) failed: %v", err)
	}
	if !json.Valid(raw) {
		t.Fatalf("readFile(yaml) returned non-json: %s", raw)
	}

	c := New()
	c.Encoding = "json"
	c.SetRequestFile = []string{requestFile}
	if err := c.ReadSetRequestTemplate(); err != nil {
		t.Fatalf("ReadSetRequestTemplate() failed: %v", err)
	}
	reqs, err := c.CreateSetRequestFromFile("eth0")
	if err != nil {
		t.Fatalf("CreateSetRequestFromFile() failed: %v", err)
	}
	if len(reqs) != 1 ||
		len(reqs[0].Update) != 1 ||
		len(reqs[0].Delete) != 1 ||
		reqs[0].Delete[0].Elem[1].Key["name"] != "eth0" {
		t.Fatalf("unexpected set request: %s", prototext.Format(reqs[0]))
	}

	c = New()
	c.SetRequestFile = []string{requestFile}
	c.SetRequestVars = ""
	if err := c.readTemplateVarsFile(); err != nil {
		t.Fatalf("readTemplateVarsFile(missing implicit vars) failed: %v", err)
	}
	if c.SetRequestVars != "" {
		t.Fatalf("SetRequestVars = %q, want empty when implicit vars file is missing", c.SetRequestVars)
	}
}

func TestCreateSetRequestFromProtoFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "request.textproto")
	req := &gnmi.SetRequest{
		Delete: []*gnmi.Path{{Elem: []*gnmi.PathElem{{Name: "interfaces"}}}},
	}
	if err := os.WriteFile(file, []byte(prototext.Format(req)), 0o600); err != nil {
		t.Fatal(err)
	}
	c := New()
	c.SetRequestProtoFile = []string{file}
	reqs, err := c.CreateSetRequestFromProtoFile()
	if err != nil {
		t.Fatalf("CreateSetRequestFromProtoFile() failed: %v", err)
	}
	if len(reqs) != 1 || reqs[0].Delete[0].Elem[0].Name != "interfaces" {
		t.Fatalf("unexpected proto set request: %+v", reqs)
	}
}

func TestDiffAndGASRequests(t *testing.T) {
	c := New()
	c.Encoding = "json"
	c.DiffPath = []string{"/interfaces"}
	c.DiffPrefix = "/"
	c.DiffType = "state"
	c.DiffTarget = "target1"
	getReq, err := c.CreateDiffGetRequest()
	if err != nil {
		t.Fatalf("CreateDiffGetRequest() failed: %v", err)
	}
	if len(getReq.Path) != 1 || getReq.Path[0].Elem[0].Name != "interfaces" {
		t.Fatalf("unexpected diff get request: %s", prototext.Format(getReq))
	}

	cmd := &cobra.Command{Use: "diff"}
	cmd.Flags().Uint32("qos", 0, "")
	if err := cmd.Flags().Set("qos", "42"); err != nil {
		t.Fatal(err)
	}
	c.DiffQos = 42
	subReq, err := c.CreateDiffSubscribeRequest(cmd)
	if err != nil {
		t.Fatalf("CreateDiffSubscribeRequest() failed: %v", err)
	}
	if subReq.GetSubscribe().Qos.GetMarking() != 42 {
		t.Fatalf("unexpected diff qos: %s", prototext.Format(subReq))
	}

	c.GetSetGet = "/state"
	c.GetSetType = "state"
	gasGet, err := c.CreateGASGetRequest()
	if err != nil {
		t.Fatalf("CreateGASGetRequest() failed: %v", err)
	}
	if gasGet.Type != gnmi.GetRequest_STATE || gasGet.Path[0].Elem[0].Name != "state" {
		t.Fatalf("unexpected GAS get request: %s", prototext.Format(gasGet))
	}

	c.GetSetUpdate = `.path`
	c.GetSetValue = `.value`
	gasSet, err := c.CreateGASSetRequest(map[string]any{"path": "/system/config/hostname", "value": "leaf1"})
	if err != nil {
		t.Fatalf("CreateGASSetRequest() failed: %v", err)
	}
	if len(gasSet.Update) != 1 || gasSet.Update[0].Path.Elem[0].Name != "system" {
		t.Fatalf("unexpected GAS set request: %s", prototext.Format(gasSet))
	}
}

func TestExecValueTemplateAndRequestExtensions(t *testing.T) {
	c := New()
	val, err := c.execValueTemplate(`.value`, map[string]any{"value": "leaf1"})
	if err != nil {
		t.Fatalf("execValueTemplate() failed: %v", err)
	}
	if val != "leaf1" {
		t.Fatalf("execValueTemplate() = %q, want leaf1", val)
	}
	if _, err := c.execValueTemplate(`[1,2]`, map[string]any{}); err == nil || !strings.Contains(err.Error(), "unexpected jq result") {
		t.Fatalf("execValueTemplate(array) = %v, want unexpected result error", err)
	}

	opts, err := c.parseAdditionalRequestExtensions()
	if err != nil {
		t.Fatalf("parseAdditionalRequestExtensions(empty) failed: %v", err)
	}
	if len(opts) != 0 {
		t.Fatalf("parseAdditionalRequestExtensions(empty) len=%d", len(opts))
	}
	exts, err := createAdditionalRequestExtensions(`{}`, nil, nil, nil)
	if err != nil {
		t.Fatalf("createAdditionalRequestExtensions(no files) failed: %v", err)
	}
	if len(exts) != 0 {
		t.Fatalf("createAdditionalRequestExtensions(no files) len=%d", len(exts))
	}
}

func TestExpandOSPathFlagValuesAndMergeEnv(t *testing.T) {
	file := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(file, []byte("ca"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := New()
	c.FileConfig.Set("tls-ca", file)
	if err := c.expandOSPathFlagValues(); err != nil {
		t.Fatalf("expandOSPathFlagValues() failed: %v", err)
	}
	if got := c.FileConfig.GetString("tls-ca"); got != file {
		t.Fatalf("tls-ca = %q, want %q", got, file)
	}

	t.Setenv("GNMIC_FORMAT", "event")
	c = New()
	c.mergeEnvVars()
	if got := c.FileConfig.GetString("format"); got != "event" {
		t.Fatalf("mergeEnvVars format=%q", got)
	}
}

func TestSetGlobalsFromEnv(t *testing.T) {
	t.Setenv("GLOBAL_PASSWORD", "expanded-pass")
	c := New()
	c.FileConfig.Set("format", "event")
	c.FileConfig.Set("password", "$GLOBAL_PASSWORD")
	c.FileConfig.Set("token", "literal-token")

	cmd := &cobra.Command{Use: "root"}
	cmd.PersistentFlags().String("format", "", "")
	cmd.PersistentFlags().String("password", "", "")
	cmd.PersistentFlags().String("token", "", "")

	c.SetGlobalsFromEnv(cmd)
}

func TestLoadReadsExplicitConfigAndExpandsPaths(t *testing.T) {
	dir := t.TempDir()
	ca := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(ca, []byte("ca"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfgFile := filepath.Join(dir, "gnmic.yaml")
	if err := os.WriteFile(cfgFile, []byte(`
format: event
tls-ca: ca.pem
outputs:
  out1:
    type: file
`), 0o600); err != nil {
		t.Fatal(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })

	c := New()
	c.CfgFile = cfgFile
	if err := c.Load(t.Context()); err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if c.Format != "event" {
		t.Fatalf("Format = %q, want event", c.Format)
	}
	if got := c.FileConfig.GetString("tls-ca"); got != ca {
		t.Fatalf("tls-ca = %q, want %q", got, ca)
	}
	if c.Outputs["out1"]["type"] != "file" {
		t.Fatalf("Outputs not unmarshaled: %#v", c.Outputs)
	}
}

func TestSubscriptionConfigFromFlagsAndFileList(t *testing.T) {
	c := New()
	c.Subscriptions = make(map[string]*types.SubscriptionConfig)
	c.SubscribePrefix = "/"
	c.SubscribeTarget = "target1"
	c.SubscribePath = []string{"/interfaces"}
	c.SubscribeMode = "stream"
	c.SubscribeStreamMode = "sample"
	c.SubscribeQos = 10
	c.SubscribeSampleInterval = time.Second
	c.SubscribeHeartbeatInterval = 2 * time.Second
	c.SubscribeHistoryStart = "2024-01-01T00:00:00Z"
	c.SubscribeHistoryEnd = "2024-01-01T00:01:00Z"

	cmd := &cobra.Command{Use: "subscribe"}
	cmd.Flags().Uint32("qos", 0, "")
	cmd.Flags().Duration("sample-interval", 0, "")
	cmd.Flags().Duration("heartbeat-interval", 0, "")
	cmd.Flags().String("history-start", "", "")
	cmd.Flags().String("history-end", "", "")
	for name, value := range map[string]string{
		"qos":                "10",
		"sample-interval":    "1s",
		"heartbeat-interval": "2s",
		"history-start":      c.SubscribeHistoryStart,
		"history-end":        c.SubscribeHistoryEnd,
	} {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatal(err)
		}
	}

	subs, err := c.subscriptionConfigFromFlags(cmd)
	if err != nil {
		t.Fatalf("subscriptionConfigFromFlags() failed: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("subscriptions len = %d", len(subs))
	}
	for _, sub := range subs {
		if sub.Target != "target1" ||
			sub.Qos == nil || *sub.Qos != 10 ||
			sub.SampleInterval == nil || *sub.SampleInterval != time.Second ||
			sub.HeartbeatInterval == nil || *sub.HeartbeatInterval != 2*time.Second ||
			sub.History == nil || sub.History.Start.IsZero() || sub.History.End.IsZero() {
			t.Fatalf("unexpected subscription from flags: %+v", sub)
		}
	}

	c = newConfigFromYAML(t, `
subscriptions:
  b:
    paths: [/b]
  a:
    paths: [/a]
`)
	list := c.GetSubscriptionsFromFile()
	if len(list) != 2 || list[0].Name != "a" || list[1].Name != "b" {
		t.Fatalf("GetSubscriptionsFromFile() = %+v", list)
	}
}

func TestTargetsHelpers(t *testing.T) {
	c := New()
	c.Targets = map[string]*types.TargetConfig{
		"b": {Name: "b", Address: "b:57400"},
		"a": {Name: "a", Address: "a:57400"},
	}
	list := c.TargetsList()
	if len(list) != 2 || list[0].Name != "a" || list[1].Name != "b" {
		t.Fatalf("TargetsList() = %+v", list)
	}

	t.Setenv("TARGET_ADDR", "127.0.0.1:57400")
	username := "${TARGET_USER}"
	password := "$TARGET_PASSWORD"
	token := "${TARGET_TOKEN}"
	t.Setenv("TARGET_USER", "user1")
	t.Setenv("TARGET_PASSWORD", "pass1")
	t.Setenv("TARGET_TOKEN", "token1")
	tc := &types.TargetConfig{
		Name:          "${TARGET_ADDR}",
		Address:       "${TARGET_ADDR}",
		Username:      &username,
		Password:      &password,
		Token:         &token,
		Subscriptions: []string{"${TARGET_ADDR}"},
		Outputs:       []string{"${TARGET_ADDR}"},
		Tags:          []string{"${TARGET_ADDR}"},
	}
	expandTargetEnv(tc)
	if tc.Address != "127.0.0.1:57400" || *tc.Username != "user1" || *tc.Password != "pass1" || *tc.Token != "token1" {
		t.Fatalf("expandTargetEnv() = %+v", tc)
	}

	certFile := filepath.Join(t.TempDir(), "cert.pem")
	if err := os.WriteFile(certFile, []byte("cert"), 0o600); err != nil {
		t.Fatal(err)
	}
	insecure := false
	tc = &types.TargetConfig{
		Insecure: &insecure,
		TLSCA:    &certFile,
		TLSCert:  &certFile,
		TLSKey:   &certFile,
	}
	if err := expandCertPaths(tc); err != nil {
		t.Fatalf("expandCertPaths() failed: %v", err)
	}

	st := gomap.NewMemStore(zstore.StoreOptions[any]{})
	if _, err := st.Set("global-flags", "global-flags", GlobalFlags{
		Username: "global-user",
		Password: "global-pass",
		Port:     "57400",
		Encoding: "json",
		Insecure: true,
	}); err != nil {
		t.Fatal(err)
	}
	tc = &types.TargetConfig{Name: "t1", Address: "127.0.0.1"}
	if err := SetTargetConfigDefaults(st, tc); err != nil {
		t.Fatalf("SetTargetConfigDefaults() failed: %v", err)
	}
	if tc.Address != "127.0.0.1:57400" || tc.Username == nil || *tc.Username != "global-user" {
		t.Fatalf("target defaults not applied: %+v", tc)
	}
	t.Setenv("TARGET_HOST", "localhost")
	tc = &types.TargetConfig{Name: "t2", Address: "${TARGET_HOST}"}
	if err := SetTargetConfigDefaultsExpandEnv(st, tc); err != nil {
		t.Fatalf("SetTargetConfigDefaultsExpandEnv() failed: %v", err)
	}
	if !strings.HasPrefix(tc.Address, "localhost:") {
		t.Fatalf("target env/defaults not applied: %+v", tc)
	}
}

func TestGetDiffTargets(t *testing.T) {
	c := newConfigFromYAML(t, `
targets:
  ref:
    address: 127.0.0.1:57400
  cmp:
    address: 127.0.0.2:57400
`)
	c.Encoding = "json"
	c.DiffRef = "ref"
	c.DiffCompare = []string{"cmp", "dynamic.example.com"}
	ref, compares, err := c.GetDiffTargets()
	if err != nil {
		t.Fatalf("GetDiffTargets() failed: %v", err)
	}
	if ref.Name != "ref" || compares["cmp"].Address != "127.0.0.2:57400" || compares["dynamic.example.com"].Name != "dynamic.example.com" {
		t.Fatalf("unexpected diff targets: ref=%+v compares=%+v", ref, compares)
	}
}
