package env_test

import (
	"os"
	"testing"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/collector/env"
	"github.com/openconfig/gnmic/pkg/config"
)

func TestExpandClusterEnv(t *testing.T) {
	// Set env vars for expansion tests; restore after.
	const testCluster = "test-cluster-name"
	const testInstance = "test-instance-01"
	const testAddr = "0.0.0.0:7890"
	const testTag = "region:us-east"
	const testCa = "/etc/ssl/ca.pem"
	const testCert = "/etc/ssl/cert.pem"
	const testKey = "/etc/ssl/key.pem"
	os.Setenv("GNMIC_CLUSTER", testCluster)
	os.Setenv("GNMIC_INSTANCE", testInstance)
	os.Setenv("GNMIC_ADDR", testAddr)
	os.Setenv("GNMIC_TAG", testTag)
	os.Setenv("GNMIC_CA", testCa)
	os.Setenv("GNMIC_CERT", testCert)
	os.Setenv("GNMIC_KEY", testKey)
	defer func() {
		os.Unsetenv("GNMIC_CLUSTER")
		os.Unsetenv("GNMIC_INSTANCE")
		os.Unsetenv("GNMIC_ADDR")
		os.Unsetenv("GNMIC_TAG")
		os.Unsetenv("GNMIC_CA")
		os.Unsetenv("GNMIC_CERT")
		os.Unsetenv("GNMIC_KEY")
	}()

	tests := []struct {
		name             string
		clusteringConfig *config.Clustering
		validate         func(t *testing.T, c *config.Clustering)
	}{
		{
			name:             "empty_config",
			clusteringConfig: &config.Clustering{},
			validate: func(t *testing.T, c *config.Clustering) {
				if c.ClusterName != "" || c.InstanceName != "" || c.ServiceAddress != "" {
					t.Errorf("empty config should remain empty")
				}
			},
		},
		{
			name: "literal_strings_unchanged",
			clusteringConfig: &config.Clustering{
				ClusterName:    "my-cluster",
				InstanceName:   "instance-1",
				ServiceAddress: ":7890",
			},
			validate: func(t *testing.T, c *config.Clustering) {
				if c.ClusterName != "my-cluster" || c.InstanceName != "instance-1" || c.ServiceAddress != ":7890" {
					t.Errorf("literal strings should be unchanged")
				}
			},
		},
		{
			name: "cluster_fields_expanded",
			clusteringConfig: &config.Clustering{
				ClusterName:    "$GNMIC_CLUSTER",
				InstanceName:   "$GNMIC_INSTANCE",
				ServiceAddress: "$GNMIC_ADDR",
			},
			validate: func(t *testing.T, c *config.Clustering) {
				if c.ClusterName != testCluster || c.InstanceName != testInstance || c.ServiceAddress != testAddr {
					t.Errorf("got cluster=%q instance=%q addr=%q", c.ClusterName, c.InstanceName, c.ServiceAddress)
				}
			},
		},
		{
			name: "tags_expanded",
			clusteringConfig: &config.Clustering{
				Tags: []string{"$GNMIC_TAG", "literal", "${GNMIC_TAG}"},
			},
			validate: func(t *testing.T, c *config.Clustering) {
				if len(c.Tags) != 3 {
					t.Fatalf("len(Tags)=%d", len(c.Tags))
				}
				if c.Tags[0] != testTag || c.Tags[1] != "literal" || c.Tags[2] != testTag {
					t.Errorf("tags: got %q", c.Tags)
				}
			},
		},
		{
			name: "tls_nil_no_panic",
			clusteringConfig: &config.Clustering{
				ClusterName: "c1",
				TLS:         nil,
			},
			validate: func(t *testing.T, c *config.Clustering) {
				if c.TLS != nil {
					t.Error("TLS should still be nil")
				}
			},
		},
		{
			name: "tls_paths_expanded",
			clusteringConfig: &config.Clustering{
				ClusterName: "c1",
				TLS: &types.TLSConfig{
					CaFile:   "$GNMIC_CA",
					CertFile: "$GNMIC_CERT",
					KeyFile:  "$GNMIC_KEY",
				},
			},
			validate: func(t *testing.T, c *config.Clustering) {
				if c.TLS == nil {
					t.Fatal("TLS should be set")
				}
				if c.TLS.CaFile != testCa || c.TLS.CertFile != testCert || c.TLS.KeyFile != testKey {
					t.Errorf("TLS paths: ca=%q cert=%q key=%q", c.TLS.CaFile, c.TLS.CertFile, c.TLS.KeyFile)
				}
			},
		},
		{
			name: "locker_nil_no_panic",
			clusteringConfig: &config.Clustering{
				ClusterName: "c1",
				Locker:      nil,
			},
			validate: func(t *testing.T, c *config.Clustering) {
				if c.Locker != nil {
					t.Error("Locker should still be nil")
				}
			},
		},
		{
			name: "locker_string_values_expanded",
			clusteringConfig: &config.Clustering{
				ClusterName: "c1",
				Locker: map[string]any{
					"type":    "consul",
					"address": "$GNMIC_ADDR",
					"key":     "literal",
				},
			},
			validate: func(t *testing.T, c *config.Clustering) {
				if c.Locker == nil {
					t.Fatal("Locker should be set")
				}
				if c.Locker["address"] != testAddr || c.Locker["key"] != "literal" {
					t.Errorf("locker: got %v", c.Locker)
				}
			},
		},
		{
			name: "locker_nested_map_expanded",
			clusteringConfig: &config.Clustering{
				ClusterName: "c1",
				Locker: map[string]any{
					"type": "consul",
					"opts": map[string]any{
						"host": "$GNMIC_CLUSTER",
					},
				},
			},
			validate: func(t *testing.T, c *config.Clustering) {
				opts, _ := c.Locker["opts"].(map[string]any)
				if opts == nil || opts["host"] != testCluster {
					t.Errorf("nested locker opts: got %v", c.Locker["opts"])
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env.ExpandClusterEnv(tt.clusteringConfig)
			if tt.validate != nil && tt.clusteringConfig != nil {
				tt.validate(t, tt.clusteringConfig)
			}
		})
	}
}

func TestExpandAPIEnv(t *testing.T) {
	const testAddr = "127.0.0.1:7890"
	const testCa = "/api/ca.pem"
	const testCert = "/api/cert.pem"
	const testKey = "/api/key.pem"
	const testClientAuth = "require"
	os.Setenv("API_ADDR", testAddr)
	os.Setenv("API_CA", testCa)
	os.Setenv("API_CERT", testCert)
	os.Setenv("API_KEY", testKey)
	os.Setenv("API_CLIENT_AUTH", testClientAuth)
	defer func() {
		os.Unsetenv("API_ADDR")
		os.Unsetenv("API_CA")
		os.Unsetenv("API_CERT")
		os.Unsetenv("API_KEY")
		os.Unsetenv("API_CLIENT_AUTH")
	}()

	tests := []struct {
		name      string
		apiConfig *config.APIServer
		validate  func(t *testing.T, a *config.APIServer)
	}{
		{
			name:      "empty_config",
			apiConfig: &config.APIServer{},
			validate: func(t *testing.T, a *config.APIServer) {
				if a.Address != "" {
					t.Errorf("Address should be empty, got %q", a.Address)
				}
			},
		},
		{
			name: "address_expanded",
			apiConfig: &config.APIServer{
				Address: "$API_ADDR",
			},
			validate: func(t *testing.T, a *config.APIServer) {
				if a.Address != testAddr {
					t.Errorf("Address: got %q", a.Address)
				}
			},
		},
		{
			name: "literal_address_unchanged",
			apiConfig: &config.APIServer{
				Address: ":7890",
			},
			validate: func(t *testing.T, a *config.APIServer) {
				if a.Address != ":7890" {
					t.Errorf("Address: got %q", a.Address)
				}
			},
		},
		{
			name: "tls_nil_no_panic",
			apiConfig: &config.APIServer{
				Address: ":7890",
				TLS:     nil,
			},
			validate: func(t *testing.T, a *config.APIServer) {
				if a.TLS != nil {
					t.Error("TLS should still be nil")
				}
			},
		},
		{
			name: "tls_paths_and_client_auth_expanded",
			apiConfig: &config.APIServer{
				Address: ":7890",
				TLS: &types.TLSConfig{
					CaFile:     "$API_CA",
					CertFile:   "$API_CERT",
					KeyFile:    "$API_KEY",
					ClientAuth: "$API_CLIENT_AUTH",
				},
			},
			validate: func(t *testing.T, a *config.APIServer) {
				if a.TLS == nil {
					t.Fatal("TLS should be set")
				}
				if a.TLS.CaFile != testCa || a.TLS.CertFile != testCert || a.TLS.KeyFile != testKey || a.TLS.ClientAuth != testClientAuth {
					t.Errorf("TLS: ca=%q cert=%q key=%q clientAuth=%q", a.TLS.CaFile, a.TLS.CertFile, a.TLS.KeyFile, a.TLS.ClientAuth)
				}
			},
		},
		{
			name: "bool_flags_unchanged_true",
			apiConfig: &config.APIServer{
				EnableMetrics:         true,
				EnableProfiling:       true,
				Debug:                 true,
				HealthzDisableLogging: true,
			},
			validate: func(t *testing.T, a *config.APIServer) {
				if !a.EnableMetrics || !a.EnableProfiling || !a.Debug || !a.HealthzDisableLogging {
					t.Errorf("bools true: metrics=%v profiling=%v debug=%v healthz=%v",
						a.EnableMetrics, a.EnableProfiling, a.Debug, a.HealthzDisableLogging)
				}
			},
		},
		{
			name: "bool_flags_unchanged_false",
			apiConfig: &config.APIServer{
				EnableMetrics:         false,
				EnableProfiling:       false,
				Debug:                 false,
				HealthzDisableLogging: false,
			},
			validate: func(t *testing.T, a *config.APIServer) {
				if a.EnableMetrics || a.EnableProfiling || a.Debug || a.HealthzDisableLogging {
					t.Errorf("bools false: metrics=%v profiling=%v debug=%v healthz=%v",
						a.EnableMetrics, a.EnableProfiling, a.Debug, a.HealthzDisableLogging)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env.ExpandAPIEnv(tt.apiConfig)
			if tt.validate != nil && tt.apiConfig != nil {
				tt.validate(t, tt.apiConfig)
			}
		})
	}
}
