package apiserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
	cluster_manager "github.com/openconfig/gnmic/pkg/collector/managers/cluster"
	inputs_manager "github.com/openconfig/gnmic/pkg/collector/managers/inputs"
	outputs_manager "github.com/openconfig/gnmic/pkg/collector/managers/outputs"
	targets_manager "github.com/openconfig/gnmic/pkg/collector/managers/targets"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/config/store"
	"github.com/openconfig/gnmic/pkg/lockers"
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
}

func NewServer(
	configStore store.Store[any],
	targetManager *targets_manager.TargetsManager,
	outputsManager *outputs_manager.OutputsManager,
	inputsManager *inputs_manager.InputsManager,
	clusterManager *cluster_manager.ClusterManager,
	reg *prometheus.Registry,
) *Server {
	s := &Server{
		router:         mux.NewRouter(),
		configStore:    configStore,
		targetsManager: targetManager,
		outputsManager: outputsManager,
		inputsManager:  inputsManager,
		clusterManager: clusterManager,
		logger:         slog.With("component", "api-server"),
	}
	s.routes()
	return s
}

func (s *Server) Start(locker lockers.Locker, wg *sync.WaitGroup) error {
	s.locker = locker
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
		return nil
	}
	var apiCfg *config.APIServer
	var listener net.Listener
	switch apiCfgImpl := apiServer.(type) {
	case *config.APIServer:
		apiCfg = apiCfgImpl
		// create listener
		listener, err = net.Listen("tcp", apiCfg.Address)
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
		// TODO: rest of config and handlers
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
	err := s.srv.Shutdown(context.Background()) // change context ?
	if err != nil {
		s.logger.Error("failed to shutdown API server", "error", err)
	}
}
