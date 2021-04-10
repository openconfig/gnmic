package collector

import (
	"context"
	"fmt"

	"github.com/karimra/gnmic/inputs"
)

func WithInputs(inputsConfig map[string]map[string]interface{}) CollectorOption {
	return func(c *Collector) {
		c.inputsConfig = inputsConfig
	}
}

// AddInput adds an input called name, with config cfg to the collector instance
func (c *Collector) AddInput(name string, cfg map[string]interface{}) error {
	if c.Inputs == nil {
		c.Inputs = make(map[string]inputs.Input)
	}
	if c.inputsConfig == nil {
		c.inputsConfig = make(map[string]map[string]interface{})
	}
	if _, ok := c.Outputs[name]; ok {
		return fmt.Errorf("input '%q' already exists", name)
	}
	c.m.Lock()
	defer c.m.Unlock()
	c.inputsConfig[name] = cfg
	return nil
}

func (c *Collector) InitInput(ctx context.Context, name string, tcs map[string]interface{}) {
	c.m.Lock()
	defer c.m.Unlock()
	if _, ok := c.Inputs[name]; ok {
		return
	}
	if cfg, ok := c.inputsConfig[name]; ok {
		if inputType, ok := cfg["type"]; ok {
			c.logger.Printf("starting input type %q", inputType)
			if initializer, ok := inputs.Inputs[inputType.(string)]; ok {
				input := initializer()
				go func() {
					err := input.Start(ctx, name, cfg,
						inputs.WithLogger(c.logger),
						inputs.WithOutputs(c.Outputs),
						inputs.WithName(c.Config.Name),
						inputs.WithEventProcessors(c.EventProcessorsConfig, c.logger, tcs),
					)
					if err != nil {
						c.logger.Printf("failed to start input type %q: %v", inputType, err)
					}
				}()
				c.Inputs[name] = input
			}
		}
	}
}

func (c *Collector) InitInputs(ctx context.Context) {
	tcs := c.targetsConfigsToMap()
	for name := range c.inputsConfig {
		c.InitInput(ctx, name, tcs)
	}
}
