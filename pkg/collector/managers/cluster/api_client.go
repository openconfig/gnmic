package cluster_manager

import (
	"net/http"
	"time"

	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/config"
)

const apiClientTimeout = 10 * time.Second

// newAPIClient builds the HTTP client used for leader to member API calls,
// honoring the clustering TLS configuration.
func newAPIClient(clusteringConfig *config.Clustering) (*http.Client, error) {
	if clusteringConfig == nil || clusteringConfig.TLS == nil {
		return &http.Client{Timeout: apiClientTimeout}, nil
	}
	tlsConfig, err := utils.NewTLSConfig(
		clusteringConfig.TLS.CaFile,
		clusteringConfig.TLS.CertFile,
		clusteringConfig.TLS.KeyFile, "",
		clusteringConfig.TLS.SkipVerify,
		false, false)
	if err != nil {
		return nil, err
	}
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = tlsConfig
	return &http.Client{
		Timeout:   apiClientTimeout,
		Transport: tr,
	}, nil
}
