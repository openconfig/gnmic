// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

func TestEnvToMapSplitsOnlyFreeFormSections(t *testing.T) {
	sections := []string{
		"clustering",
		"outputs",
		"inputs",
		"processors",
		"loader",
		"actions",
	}

	for _, section := range sections {
		t.Setenv(
			"GNMIC_"+strings.ToUpper(section)+"_TEST_VALUE",
			section,
		)
	}

	// These are known struct-backed fields and must be handled by Viper,
	// not interpreted as nested maps by envToMap.
	t.Setenv("GNMIC_CLUSTER_NAME", "collector-cluster")
	t.Setenv("GNMIC_LOG_FILE", "/tmp/gnmic-env.log")

	got := envToMap()

	for _, section := range sections {
		level1, ok := got[section].(map[string]any)
		if !ok {
			t.Fatalf(
				"envToMap()[%q] = %T, want map[string]any",
				section,
				got[section],
			)
		}

		level2, ok := level1["test"].(map[string]any)
		if !ok {
			t.Fatalf(
				"envToMap()[%q][test] = %T, want map[string]any",
				section,
				level1["test"],
			)
		}

		if value, want := level2["value"], section; value != want {
			t.Errorf(
				"envToMap()[%q][test][value] = %v, want %q",
				section,
				value,
				want,
			)
		}
	}

	if _, ok := got["cluster"]; ok {
		t.Errorf(
			"envToMap() contains cluster = %v; known cluster-name field should be excluded",
			got["cluster"],
		)
	}

	if _, ok := got["log"]; ok {
		t.Errorf(
			"envToMap() contains log = %v; known log-file field should be excluded",
			got["log"],
		)
	}
}

func TestLoadEnvironmentOverridesConfig(t *testing.T) {
	t.Setenv("GNMIC_CLUSTER_NAME", "collector-cluster")
	t.Setenv("GNMIC_LOG_FILE", "/tmp/gnmic-env.log")
	t.Setenv(
		"GNMIC_CLUSTERING_LOCKER_ADDRESS",
		"consul.example:8500",
	)

	configFile := filepath.Join(t.TempDir(), "gnmic.yaml")
	if err := os.WriteFile(
		configFile,
		[]byte(`log-file: /tmp/gnmic-file.log
clustering:
  locker:
    type: consul
    address: file.example:8500
`),
		0o600,
	); err != nil {
		t.Fatalf("os.WriteFile() failed: %v", err)
	}

	cfg := New()
	cfg.CfgFile = configFile

	// Global flags are bound before Config.Load in the actual application.
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("cluster-name", "default-cluster", "")
	flags.String("log-file", "", "")

	for _, name := range []string{"cluster-name", "log-file"} {
		if err := cfg.FileConfig.BindPFlag(
			name,
			flags.Lookup(name),
		); err != nil {
			t.Fatalf("BindPFlag(%q) failed: %v", name, err)
		}
	}

	if err := cfg.Load(context.Background()); err != nil {
		t.Fatalf("Config.Load() failed: %v", err)
	}

	if got, want := cfg.ClusterName, "collector-cluster"; got != want {
		t.Errorf("ClusterName = %q, want %q", got, want)
	}

	if got, want := cfg.LogFile, "/tmp/gnmic-env.log"; got != want {
		t.Errorf("LogFile = %q, want %q", got, want)
	}

	if cfg.Clustering == nil {
		t.Fatal("Clustering is nil, want configuration populated")
	}

	if got, want := cfg.Clustering.Locker["type"], "consul"; got != want {
		t.Errorf(
			"Clustering.Locker[type] = %v, want %q",
			got,
			want,
		)
	}

	if got, want := cfg.Clustering.Locker["address"], "consul.example:8500"; got != want {
		t.Errorf(
			"Clustering.Locker[address] = %v, want %q",
			got,
			want,
		)
	}
}
