// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/grafana/pyroscope-go"
	"github.com/openconfig/gnmic/pkg/cache"
	apiserver "github.com/openconfig/gnmic/pkg/collector/api/server"
	cluster_manager "github.com/openconfig/gnmic/pkg/collector/managers/cluster"
	inputs_manager "github.com/openconfig/gnmic/pkg/collector/managers/inputs"
	outputs_manager "github.com/openconfig/gnmic/pkg/collector/managers/outputs"
	targets_manager "github.com/openconfig/gnmic/pkg/collector/managers/targets"
	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/lockers"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/openconfig/gnmic/pkg/pipeline"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"github.com/zestor-dev/zestor/store"
)

const (
	defaultPipelineBufferSize = 1_000_000
	initLockerRetryTimer      = 1 * time.Second
)

type Collector struct {
	ctx   context.Context
	store *collstore.Store

	apiServer *apiserver.Server
	cache     cache.Cache

	locker         lockers.Locker
	clusterManager *cluster_manager.ClusterManager
	targetsManager *targets_manager.TargetsManager
	outputsManager *outputs_manager.OutputsManager
	inputsManager  *inputs_manager.InputsManager
	pipeline       chan *pipeline.Msg
	wg             *sync.WaitGroup

	logger   *slog.Logger
	reg      *prometheus.Registry
	profiler *pyroscope.Profiler
}

func New(ctx context.Context, configStore store.Store[any]) *Collector {
	s := collstore.NewStore(configStore)
	pipeline := make(chan *pipeline.Msg, defaultPipelineBufferSize)
	reg := prometheus.NewRegistry()

	clusterManager := cluster_manager.NewClusterManager(s)
	targetsManager := targets_manager.NewTargetsManager(ctx, s, pipeline, reg)
	outputsManager := outputs_manager.NewOutputsManager(ctx, s, pipeline, reg)
	inputsManager := inputs_manager.NewInputsManager(ctx, s, pipeline)
	apiServer := apiserver.NewServer(
		s,
		targetsManager, outputsManager,
		inputsManager, clusterManager,
		reg,
	)
	c := &Collector{
		ctx:            ctx,
		store:          s,
		apiServer:      apiServer,
		clusterManager: clusterManager,
		targetsManager: targetsManager,
		outputsManager: outputsManager,
		inputsManager:  inputsManager,
		pipeline:       pipeline,
		wg:             new(sync.WaitGroup),
		reg:            reg,
	}
	return c
}

func (c *Collector) Start() error {
	c.logger = logging.NewLogger(c.store.Config, "component", "collector")
	var err error
	c.logger.Info("starting collector")
	// build locker
	for {
		err = c.getLocker()
		if err != nil {
			c.logger.Error("failed to get locker", "error", err)
			time.Sleep(initLockerRetryTimer)
			continue
		}
		break
	}
	if c.locker != nil {
		// start cluster manager
		err = c.clusterManager.Start(c.ctx, c.locker, c.wg)
		if err != nil {
			return err
		}
	}
	// create cache
	c.initCache()
	// start managers
	err = c.targetsManager.Start(c.locker, c.wg)
	if err != nil {
		return err
	}
	err = c.outputsManager.Start(c.cache, c.wg)
	if err != nil {
		return err
	}
	err = c.inputsManager.Start(c.wg)
	if err != nil {
		return err
	}
	// start API server
	err = c.apiServer.Start(c.locker, c.wg)
	if err != nil {
		return err
	}
	// wait for context done
	<-c.ctx.Done()
	// wait for all components to finish
	c.wg.Wait()
	return nil
}

func (c *Collector) Stop() {
	c.logger.Info("stopping collector")
	c.apiServer.Stop()
	c.clusterManager.Stop()
	c.targetsManager.Stop()
	c.outputsManager.Stop()
	c.inputsManager.Stop()
}

func (c *Collector) getLocker() error {
	clusteringMap, ok, err := c.store.Config.Get("clustering", "clustering")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	clustering, ok := clusteringMap.(*config.Clustering)
	if !ok {
		return errors.New("malformed clustering config")
	}
	if clustering == nil {
		return nil
	}
	if lockerType, ok := clustering.Locker["type"]; ok {
		c.logger.Info("starting locker", "type", lockerType)
		if initializer, ok := lockers.Lockers[lockerType.(string)]; ok {
			lock := initializer()
			err := lock.Init(c.ctx, clustering.Locker, lockers.WithLogger(log.New(os.Stdout, "", log.LstdFlags)))
			if err != nil {
				return err
			}
			c.locker = lock
			return nil
		}
		return fmt.Errorf("unknown locker type %q", lockerType)
	}
	return errors.New("missing locker type field")
}

func (c *Collector) CollectorPreRunE(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unknown command %q", args[0])
	}
	pyroscopeServerAddress := cmd.Flag("pyroscope-server-address").Value.String()
	pyroscopeApplicationName := cmd.Flag("pyroscope-application-name").Value.String()
	if pyroscopeServerAddress == "" {
		return nil
	}
	var err error
	c.profiler, err = pyroscope.Start(
		pyroscope.Config{
			ApplicationName: pyroscopeApplicationName,
			ServerAddress:   pyroscopeServerAddress,
			ProfileTypes: []pyroscope.ProfileType{
				pyroscope.ProfileInuseObjects,
				pyroscope.ProfileAllocObjects,
				pyroscope.ProfileInuseSpace,
				pyroscope.ProfileAllocSpace,
				pyroscope.ProfileGoroutines,
				pyroscope.ProfileMutexCount,
				pyroscope.ProfileMutexDuration,
				pyroscope.ProfileBlockCount,
				pyroscope.ProfileBlockDuration,
			},
		})
	if err != nil {
		return err
	}

	return nil
}

func (c *Collector) CollectorRunE(cmd *cobra.Command, _ []string) error {
	if c.profiler != nil {
		defer c.profiler.Stop()
	}
	return c.Start()
}

// InitSubscribeFlags used to init or reset subscribeCmd flags for gnmic-prompt mode
func (c *Collector) InitCollectorFlags(cmd *cobra.Command) {
	cmd.ResetFlags()
	cmd.Flags().String("pyroscope-server-address", "", "Pyroscope server address")
	cmd.Flags().String("pyroscope-application-name", "gnmic-collector", "Pyroscope application name")
}

func (c *Collector) initCache() error {
	cfg, ok, err := c.store.Config.Get("gnmi-server", "gnmi-server")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if cfg == nil {
		return nil
	}
	switch cfg := cfg.(type) {
	case *config.GNMIServer:
		if cfg == nil {
			return nil
		}
		if cfg.Cache == nil {
			return nil
		}
		c.cache, err = cache.New(cfg.Cache, cache.WithLogger(log.New(os.Stdout, "", log.LstdFlags)))
		if err != nil {
			return err
		}
	}
	return nil
}
