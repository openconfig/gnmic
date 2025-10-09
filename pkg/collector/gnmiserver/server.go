package gnmiserver

import (
	"context"
	"log/slog"

	inputs_manager "github.com/openconfig/gnmic/pkg/collector/managers/inputs"
	outputs_manager "github.com/openconfig/gnmic/pkg/collector/managers/outputs"
	targets_manager "github.com/openconfig/gnmic/pkg/collector/managers/targets"
	"github.com/openconfig/gnmic/pkg/config/store"
	"github.com/prometheus/client_golang/prometheus"
)

type Server struct {
	configStore store.Store[any]

	targetsManager *targets_manager.TargetsManager
	outputsManager *outputs_manager.OutputsManager
	inputsManager  *inputs_manager.InputsManager

	logger *slog.Logger
	reg    *prometheus.Registry
}

func NewServer(configStore store.Store[any],
	targetsManager *targets_manager.TargetsManager,
	outputsManager *outputs_manager.OutputsManager,
	inputsManager *inputs_manager.InputsManager,
	reg *prometheus.Registry,
) *Server {
	return &Server{
		configStore:    configStore,
		targetsManager: targetsManager,
		outputsManager: outputsManager,
		inputsManager:  inputsManager,
		logger:         slog.With("component", "gnmi-server"),
		reg:            reg,
	}
}

func (s *Server) Start(ctx context.Context) error {
	return nil
}

func (s *Server) Stop() error {
	return nil
}
