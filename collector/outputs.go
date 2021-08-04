package collector

import (
	"context"
	"fmt"

	"github.com/karimra/gnmic/outputs"
	"github.com/karimra/gnmic/types"
)

// AddOutput initializes an output called name, with config cfg if it does not already exist
func (c *Collector) AddOutput(name string, cfg map[string]interface{}) error {
	if c.Outputs == nil {
		c.Outputs = make(map[string]outputs.Output)
	}
	if c.outputsConfig == nil {
		c.outputsConfig = make(map[string]map[string]interface{})
	}
	if _, ok := c.Outputs[name]; ok {
		return fmt.Errorf("output '%s' already exists", name)
	}
	c.m.Lock()
	defer c.m.Unlock()
	c.outputsConfig[name] = cfg
	return nil
}

func (c *Collector) InitOutput(ctx context.Context, name string, tcs map[string]*types.TargetConfig) {
	c.m.Lock()
	defer c.m.Unlock()
	if _, ok := c.Outputs[name]; ok {
		return
	}
	if cfg, ok := c.outputsConfig[name]; ok {
		if outType, ok := cfg["type"]; ok {
			c.logger.Printf("starting output type %s", outType)
			if initializer, ok := outputs.Outputs[outType.(string)]; ok {
				out := initializer()
				go func() {
					err := out.Init(ctx, name, cfg,
						outputs.WithLogger(c.logger),
						outputs.WithEventProcessors(c.EventProcessorsConfig, c.logger, c.targetsConfig),
						outputs.WithRegister(c.reg),
						outputs.WithName(c.Config.Name),
						outputs.WithClusterName(c.Config.ClusterName),
						outputs.WithTargetsConfig(tcs),
					)
					if err != nil {
						c.logger.Printf("failed to init output type %q: %v", outType, err)
					}
				}()
				c.Outputs[name] = out
			}
		}
	}
}

func (c *Collector) InitOutputs(ctx context.Context) {
	for name := range c.outputsConfig {
		c.InitOutput(ctx, name, c.targetsConfig)
	}
}

func (c *Collector) DeleteOutput(name string) error {
	if c.Outputs == nil {
		return nil
	}
	if _, ok := c.Outputs[name]; !ok {
		return fmt.Errorf("output '%s' does not exist", name)
	}
	c.m.Lock()
	defer c.m.Unlock()
	o := c.Outputs[name]
	o.Close()
	return nil
}
