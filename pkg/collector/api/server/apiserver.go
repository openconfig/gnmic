package apiserver

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	"github.com/openconfig/gnmic/pkg/api/utils"
	cluster_manager "github.com/openconfig/gnmic/pkg/collector/managers/cluster"
	inputs_manager "github.com/openconfig/gnmic/pkg/collector/managers/inputs"
	outputs_manager "github.com/openconfig/gnmic/pkg/collector/managers/outputs"
	targets_manager "github.com/openconfig/gnmic/pkg/collector/managers/targets"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/lockers"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/openconfig/gnmic/pkg/store"
	"github.com/prometheus/client_golang/prometheus"
)

type Server struct {
	router      *mux.Router
	configStore store.Store[any]

	locker         lockers.Locker
	targetsManager *targets_manager.TargetsManager
	outputsManager *outputs_manager.OutputsManager
	inputsManager  *inputs_manager.InputsManager
	clusterManager *cluster_manager.ClusterManager
	srv            *http.Server
	logger         *slog.Logger
	reg            *prometheus.Registry
}

func NewServer(
	store store.Store[any],
	targetManager *targets_manager.TargetsManager,
	outputsManager *outputs_manager.OutputsManager,
	inputsManager *inputs_manager.InputsManager,
	clusterManager *cluster_manager.ClusterManager,
	reg *prometheus.Registry,
) *Server {
	s := &Server{
		router:         mux.NewRouter(),
		configStore:    store,
		targetsManager: targetManager,
		outputsManager: outputsManager,
		inputsManager:  inputsManager,
		clusterManager: clusterManager,
		reg:            reg,
	}
	s.routes()
	s.registerMetrics()
	return s
}

func (s *Server) Start(locker lockers.Locker, wg *sync.WaitGroup) error {
	s.locker = locker
	s.logger = logging.NewLogger(s.configStore, "component", "api-server")
	s.logger.Info("starting API server")

	apiServer, ok, err := s.configStore.Get("api-server", "api-server")
	if err != nil {
		s.logger.Error("failed to get api-server config", "error", err)
		return err
	}
	if !ok {
		return nil
	}
	if apiServer == nil {
		s.logger.Info("api-server config not found, skipping API server")
		return nil
	}
	var apiCfg *config.APIServer
	var listener net.Listener
	switch apiCfgImpl := apiServer.(type) {
	case *config.APIServer:
		if apiCfgImpl == nil {
			s.logger.Info("api-server config is nil, skipping API server")
			return nil
		}
		apiCfg = apiCfgImpl
		// create listener
		listener, err = createListener(apiCfgImpl)
		if err != nil {
			s.logger.Error("failed to create listener", "error", err)
			return err
		}
	default:
		s.logger.Error("invalid api-server config", "config", apiServer)
		return fmt.Errorf("invalid api-server config: %v", apiServer)
	}
	s.srv = &http.Server{
		Addr:    apiCfg.Address,
		Handler: s.router,
		// ReadTimeout:  apiCfg.Timeout / 2,
		// WriteTimeout: apiCfg.Timeout / 2,
		// IdleTimeout:  apiCfg.Timeout / 2,
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := s.srv.Serve(listener)
		if err != nil { // TODO: ignore shutdown errors
			s.logger.Error("failed to serve API server", "error", err)
		}
	}()
	return nil
}

func (s *Server) Stop() {
	s.logger.Info("stopping API server")
	err := s.srv.Shutdown(context.Background()) // TODO: change context ?
	if err != nil {
		s.logger.Error("failed to shutdown API server", "error", err)
	}
}

type APIErrors struct {
	Errors []string `json:"errors,omitempty"`
}

func createListener(apiCfg *config.APIServer) (net.Listener, error) {
	if apiCfg.TLS != nil {
		tlsCfg, err := utils.NewTLSConfig(
			apiCfg.TLS.CaFile,
			apiCfg.TLS.CertFile,
			apiCfg.TLS.KeyFile,
			apiCfg.TLS.ClientAuth,
			apiCfg.TLS.SkipVerify,
			false, // genSelfSigned
		)
		if err != nil {
			return nil, err
		}
		return tls.Listen("tcp", apiCfg.Address, tlsCfg)
	}
	return net.Listen("tcp", apiCfg.Address)
}
