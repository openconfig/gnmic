The `event-plugin` processor initializes a processor that gNMIc loaded from the configured path under the `plugins:` section.

```yaml
plugins:
  # path to load plugin binaries from.
  path: /path/to/plugin/bin
  # glob to match binaries against.
  glob: "*"
  # sets a start timeout for plugins.
  start-timeout: 0s
```

The specific configuration of an `event-plugin` processor varies from one plugin to another. But they are configured just like any other processor i.e under the `processors:` section of the config file and linked to outputs by name reference.

The below configuration snippet initializes the `event-add-hostname` processor (a binary stored under `plugins.path`) and links to output `out1`.

```yaml
processors:
  proc1:
    event-add-hostname:
      debug: true
      # the tag name to add with the host hostname as a tag value.
      hostname-tag-name: "collector-host"
      # read-interval controls how often the plugin runs the hostname cmd to get the host hostanme
      # by default it's at most every 1 minute
      read-interval: 1m

outputs:
  out1:
    type: file
    format: event
    event-processors:
      - proc1
```

### Examples

See [here](https://github.com/openconfig/gnmic/tree/main/examples/plugins).

### Writing a plugin processor

Currently plugin processor can only be written in Golang. It relies on Hashicorp's [go-plugin](https://github.com/hashicorp/go-plugin) package for discovery and communication with gNMIc's main process.

To write your own processor you can use the below skeleton code as a starting point. Can be found [here](https://github.com/openconfig/gnmic/tree/main/examples/plugins/minimal) as well.

1. Choose a name for your processor
2. Add struct fields to decode the processor's config into.
3. Implement your processor's logic under the `Apply` method
4. Optionally, store the `targets`,`actions` and `processors` config maps given to the processor under your processor's struct (`myProcessor`) if they are relevant to your processor's logic.

```go
package main

import (
	"log"
	"os"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/formatters/event_plugin"
	"github.com/openconfig/gnmic/pkg/api/types"
)

const (
	// TODO: Choose a name for your processor
	processorType = "event-my-processor"
)

type myProcessor struct {
	// TODO: Add your config struct fields here
}

func (p *myProcessor) Init(cfg interface{}, opts ...formatters.Option) error {
	// decode the plugin config
	err := formatters.DecodeConfig(cfg, p)
	if err != nil {
		return err
	}
	// apply options
	for _, o := range opts {
		o(p)
	}
	// TODO: Other initialization steps...
	return nil
}

func (p *myProcessor) Apply(event ...*formatters.EventMsg) []*formatters.EventMsg {
	// TODO: The processor's logic is applied here
	return event
}

func (p *myProcessor) WithActions(act map[string]map[string]interface{}) {
}

func (p *myProcessor) WithTargets(tcs map[string]*types.TargetConfig) {
}

func (p *myProcessor) WithProcessors(procs map[string]map[string]any) {
}

func (p *myProcessor) WithLogger(l *log.Logger) {
}

func main() {
	logger := hclog.New(&hclog.LoggerOptions{
		Output:      os.Stderr,
		DisableTime: true,
	})

	logger.Info("starting plugin processor", "name", processorType)

	// TODO: Create and initialize your processor's struct
	plug := &myProcessor{}
	// start it
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
```

#### Return value

The data model of the return event is:

```go
type EventMsg struct {
	Name      string                 `json:"name,omitempty"`
	Timestamp int64                  `json:"timestamp,omitempty"`
	Tags      map[string]string      `json:"tags,omitempty"`
	Values    map[string]interface{} `json:"values,omitempty"`
	Deletes   []string               `json:"deletes,omitempty"`
}
```

However `Value` only support based type as value.
