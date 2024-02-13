package main

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/formatters/event_plugin"
)

const (
	processorType = "event-add-hostname"
	loggingPrefix = "[" + processorType + "] "
	hostnameCmd   = "hostname"
)

type addHostnameProcessor struct {
	Debug           bool          `mapstructure:"debug,omitempty" yaml:"debug,omitempty" json:"debug,omitempty"`
	ReadInterval    time.Duration `mapstructure:"read-interval,omitempty" yaml:"read-interval,omitempty" json:"read-interval,omitempty"`
	HostnameTagName string        `mapstructure:"hostname-tag-name,omitempty" yaml:"hostname-tag-name,omitempty" json:"hostname-tag-name,omitempty"`

	m        *sync.RWMutex
	hostname string
	lastRead time.Time

	targetsConfigs        map[string]*types.TargetConfig
	actionsDefinitions    map[string]map[string]interface{}
	processorsDefinitions map[string]map[string]any
	logger                hclog.Logger
}

func (p *addHostnameProcessor) Init(cfg interface{}, opts ...formatters.Option) error {
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}

	p.setupLogger()
	if p.ReadInterval <= 0 {
		p.ReadInterval = time.Minute
	}
	if p.HostnameTagName == "" {
		p.HostnameTagName = "collector-hostname"
	}
	return p.readHostname()
}

func (p *addHostnameProcessor) Apply(event ...*formatters.EventMsg) []*formatters.EventMsg {
	p.m.Lock()
	defer p.m.Unlock()

	err := p.readHostname()
	if err != nil {
		p.logger.Error("failed to read hostname", "error", err)
	}
	for _, e := range event {
		if e.Tags == nil {
			e.Tags = make(map[string]string)
		}
		e.Tags[p.HostnameTagName] = p.hostname
	}
	return event
}

func (p *addHostnameProcessor) WithActions(act map[string]map[string]interface{}) {
	p.actionsDefinitions = act
}

func (p *addHostnameProcessor) WithTargets(tcs map[string]*types.TargetConfig) {
	p.targetsConfigs = tcs
}

func (p *addHostnameProcessor) WithProcessors(procs map[string]map[string]any) {
	p.processorsDefinitions = procs
}

func (p *addHostnameProcessor) WithLogger(l *log.Logger) {
}

func (p *addHostnameProcessor) setupLogger() {
	p.logger = hclog.New(&hclog.LoggerOptions{
		Output:     os.Stderr,
		TimeFormat: "2006/01/02 15:04:05.999999",
	})
	if p.Debug {
		p.logger.SetLevel(hclog.Debug)
	}
}

func (p *addHostnameProcessor) readHostname() error {
	now := time.Now()
	if p.lastRead.After(now.Add(-p.ReadInterval)) {
		return nil
	}
	//
	cmd := exec.Command(hostnameCmd)
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	p.hostname = string(bytes.TrimSpace(out))
	p.lastRead = now
	return nil
}

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Output:      os.Stderr,
		DisableTime: true,
	})

	logger.Info("starting plugin processor", "name", processorType)

	plug := &addHostnameProcessor{
		m: new(sync.RWMutex),
	}
	plugin.Serve(&plugin.ServeConfig{
		HandshakeConfig: plugin.HandshakeConfig{
			ProtocolVersion:  1,
			MagicCookieKey:   "GNMIC_PLUGIN",
			MagicCookieValue: "gnmic",
		},
		Plugins: map[string]plugin.Plugin{
			processorType: &event_plugin.EventProcessorPlugin{Impl: plug},
		},
		Logger: logger,
	})
}
