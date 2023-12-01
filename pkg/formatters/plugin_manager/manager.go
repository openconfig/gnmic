package plugin_manager

import (
	"log"
	"os/exec"
	"path/filepath"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/formatters/event_plugin"
)

var handshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "GNMIC_PLUGIN",
	MagicCookieValue: "gnmic",
}
var Plugins []*plugin.Client

type PluginManager struct {
	pluginMap map[string]plugin.Plugin
	Debug     bool
	logger    hclog.Logger
}

func init() {
	pm := NewPluginManager()
	pm.LoadPlugins()
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		pluginMap: map[string]plugin.Plugin{},
		logger: hclog.New(
			&hclog.LoggerOptions{Level: hclog.Trace, TimeFormat: "2006/01/02 15:04:05.999999"},
		),
	}
}

func (p *PluginManager) LoadPlugins() error {
	pluginPaths, _ := plugin.Discover("*", "./plugins")

	for _, pluginPath := range pluginPaths {
		fileName := filepath.Base(pluginPath)
		p.pluginMap[fileName] = &event_plugin.EventProcessorPlugin{}
	}

	for _, pluginPath := range pluginPaths {

		client := plugin.NewClient(&plugin.ClientConfig{
			HandshakeConfig: handshakeConfig,
			Plugins:         p.pluginMap,
			Cmd:             exec.Command(pluginPath),
			Logger:          p.logger,
		})

		Plugins = append(Plugins, client)
		fileName := filepath.Base(pluginPath)

		rpcClient, err := client.Client()
		if err != nil {
			log.Fatal(err)
		}

		raw, err := rpcClient.Dispense(fileName)
		if err != nil {
			log.Fatal(err)
		}
		eventPlugin := raw.(formatters.EventProcessor)

		formatters.Register(fileName, func() formatters.EventProcessor { return eventPlugin })
		formatters.EventProcessorTypes = append(formatters.EventProcessorTypes, fileName)
	}

	return nil
}

func Cleanup() {
	for _, client := range Plugins {
		client.Kill()
	}
}
