package env

import (
	"os"
	"strconv"
	"strings"

	"github.com/openconfig/gnmic/pkg/config"
)

func ExpandClusterEnv(clusteringConfig *config.Clustering) {
	clusteringConfig.ClusterName = os.ExpandEnv(clusteringConfig.ClusterName)
	clusteringConfig.InstanceName = os.ExpandEnv(clusteringConfig.InstanceName)
	clusteringConfig.ServiceAddress = os.ExpandEnv(clusteringConfig.ServiceAddress)
	for i := range clusteringConfig.Tags {
		clusteringConfig.Tags[i] = os.ExpandEnv(clusteringConfig.Tags[i])
	}
	if clusteringConfig.TLS != nil {
		clusteringConfig.TLS.CaFile = os.ExpandEnv(clusteringConfig.TLS.CaFile)
		clusteringConfig.TLS.CertFile = os.ExpandEnv(clusteringConfig.TLS.CertFile)
		clusteringConfig.TLS.KeyFile = os.ExpandEnv(clusteringConfig.TLS.KeyFile)
	}
	if clusteringConfig.Locker != nil {
		expandLockerEnv(clusteringConfig.Locker)
	}
}

func ExpandAPIEnv(apiConfig *config.APIServer) {
	apiConfig.Address = os.ExpandEnv(apiConfig.Address)
	if apiConfig.TLS != nil {
		apiConfig.TLS.CaFile = os.ExpandEnv(apiConfig.TLS.CaFile)
		apiConfig.TLS.CertFile = os.ExpandEnv(apiConfig.TLS.CertFile)
		apiConfig.TLS.KeyFile = os.ExpandEnv(apiConfig.TLS.KeyFile)
		apiConfig.TLS.ClientAuth = os.ExpandEnv(apiConfig.TLS.ClientAuth)
	}
	apiConfig.EnableMetrics = os.ExpandEnv(strings.ToLower(strconv.FormatBool(apiConfig.EnableMetrics))) == "true"
	apiConfig.EnableProfiling = os.ExpandEnv(strings.ToLower(strconv.FormatBool(apiConfig.EnableProfiling))) == "true"
	apiConfig.Debug = os.ExpandEnv(strings.ToLower(strconv.FormatBool(apiConfig.Debug))) == "true"
	apiConfig.HealthzDisableLogging = os.ExpandEnv(strings.ToLower(strconv.FormatBool(apiConfig.HealthzDisableLogging))) == "true"
}

func expandLockerEnv(locker map[string]any) {
	expandMapEnv(locker)
}

func expandMapEnv(m map[string]any) {
	for f := range m {
		switch v := m[f].(type) {
		case string:
			m[f] = os.ExpandEnv(v)
		case map[string]any:
			expandMapEnv(v)
			m[f] = v
		case []any:
			for i, item := range v {
				switch item := item.(type) {
				case string:
					v[i] = os.ExpandEnv(item)
				case map[string]any:
					expandMapEnv(item)
				case []any:
					expandSliceEnv(item)
				}
			}
			m[f] = v
		}
	}
}

func expandSliceEnv(s []any) {
	for i, item := range s {
		switch item := item.(type) {
		case string:
			s[i] = os.ExpandEnv(item)
		case map[string]any:
			expandMapEnv(item)
		case []any:
			expandSliceEnv(item)
		}
	}
}
