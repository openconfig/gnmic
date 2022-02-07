package config

import (
	"fmt"
	"strings"

	"github.com/karimra/gnmic/api"
	"github.com/karimra/gnmic/types"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
)

func (c *Config) CreateDiffSubscribeRequest(cmd *cobra.Command) (*gnmi.SubscribeRequest, error) {
	sc := &types.SubscriptionConfig{
		Name:     "diff-sub",
		Models:   c.DiffModel,
		Prefix:   c.DiffPrefix,
		Target:   c.DiffTarget,
		Paths:    c.DiffPath,
		Mode:     "ONCE",
		Encoding: c.Encoding,
	}
	if flagIsSet(cmd, "qos") {
		sc.Qos = &c.DiffQos
	}
	return c.CreateSubscribeRequest(sc, "")
}

func (c *Config) CreateDiffGetRequest() (*gnmi.GetRequest, error) {
	if c == nil {
		return nil, fmt.Errorf("%w", ErrInvalidConfig)
	}
	gnmiOpts := make([]api.GNMIOption, 0, 4+len(c.LocalFlags.DiffPath))
	gnmiOpts = append(gnmiOpts,
		api.Encoding(c.Encoding),
		api.DataType(c.LocalFlags.DiffType),
		api.Prefix(c.LocalFlags.DiffPrefix),
		api.Target(c.LocalFlags.DiffTarget),
	)
	for _, p := range c.LocalFlags.DiffPath {
		gnmiOpts = append(gnmiOpts, api.Path(strings.TrimSpace(p)))
	}
	return api.NewGetRequest(gnmiOpts...)
}
