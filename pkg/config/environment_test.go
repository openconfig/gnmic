// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
)

func TestLoadEnvironmentOverridesConfig(t *testing.T) {
	t.Setenv("GNMIC_CLUSTER_NAME", "collector-cluster")
	t.Setenv("GNMIC_CLUSTERING_LOCKER_ADDRESS", "consul.example:8500")

	configFile := filepath.Join(t.TempDir(), "gnmic.yaml")
	if err := os.WriteFile(
		configFile,
		[]byte(`clustering:
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
	if err := cfg.FileConfig.BindPFlag(
		"cluster-name",
		flags.Lookup("cluster-name"),
	); err != nil {
		t.Fatalf("BindPFlag() failed: %v", err)
	}

	if err := cfg.Load(context.Background()); err != nil {
		t.Fatalf("Config.Load() failed: %v", err)
	}

	if got, want := cfg.ClusterName, "collector-cluster"; got != want {
		t.Errorf("ClusterName = %q, want %q", got, want)
	}

	if cfg.Clustering == nil {
		t.Fatal("Clustering is nil, want configuration populated from file and environment")
	}

	if got, want := cfg.Clustering.Locker["type"], "consul"; got != want {
		t.Errorf("Clustering.Locker[type] = %v, want %q", got, want)
	}

	if got, want := cfg.Clustering.Locker["address"], "consul.example:8500"; got != want {
		t.Errorf("Clustering.Locker[address] = %v, want %q", got, want)
	}
}
