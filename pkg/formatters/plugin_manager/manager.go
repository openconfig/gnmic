package plugin_manager

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"

	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/formatters/event_plugin"
)

var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "GNMIC_PLUGIN",
	MagicCookieValue: "gnmic",
}

type PluginManager struct {
	config    *config.PluginsConfig
	logOutput io.Writer

	m             *sync.Mutex
	pluginClients []*plugin.Client

	logger hclog.Logger
}

func New(pc *config.PluginsConfig, logOutput io.Writer) *PluginManager {
	pm := &PluginManager{
		config:        pc,
		logOutput:     logOutput,
		m:             new(sync.Mutex),
		pluginClients: make([]*plugin.Client, 0),
	}
	pm.logger = hclog.New(
		&hclog.LoggerOptions{
			Name:       "plugin-manager",
			Level:      hclog.Info,
			Output:     logOutput,
			TimeFormat: "2006/01/02 15:04:05.999999",
		},
	)
	if pc.Debug {
		pm.logger.SetLevel(hclog.Debug)
	}
	return pm
}

func (p *PluginManager) Load() error {
	if p.config == nil {
		return nil
	}
	// discover plugins in the supplied path
	pluginPaths, err := plugin.Discover(p.config.Glob, p.config.Path)
	if err != nil {
		return err
	}

	// initialize plugins clients and register plugin processors
	for _, pluginPath := range pluginPaths {
		name := filepath.Base(pluginPath)
		formatters.EventProcessorTypes = append(formatters.EventProcessorTypes, name)
		formatters.Register(name, p.initProcessorFn(name, pluginPath))
	}

	return nil
}

func (p *PluginManager) Cleanup() {
	p.m.Lock()
	defer p.m.Unlock()
	for _, client := range p.pluginClients {
		client.Kill()
	}
}

func (p *PluginManager) initProcessorFn(name, pluginPath string) func() formatters.EventProcessor {
	return func() formatters.EventProcessor {
		client := plugin.NewClient(&plugin.ClientConfig{
			HandshakeConfig: handshakeConfig,
			Plugins:         map[string]plugin.Plugin{name: &event_plugin.EventProcessorPlugin{}},
			Cmd:             exec.Command(pluginPath),
			StartTimeout:    p.config.StartTimeout,
			SyncStdout:      p.logOutput,
			SyncStderr:      p.logOutput,
			Logger:          p.logger,
		})
		p.m.Lock()
		p.pluginClients = append(p.pluginClients, client)
		p.m.Unlock()
		rpcClient, err := client.Client()
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to initialize plugin processor %s: %v\n", name, err)
			os.Exit(1)
		}
		raw, err := rpcClient.Dispense(name)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to dispense plugin processor %s: %v\n", name, err)
			os.Exit(1)
		}
		eventPlugin, ok := raw.(formatters.EventProcessor)
		if !ok {
			err := fmt.Errorf("plugin %s dispensed an unexpected interface: %T", name, raw)
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
		}
		return eventPlugin
	}
}
