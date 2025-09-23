// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"strings"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"

	"github.com/openconfig/gnmic/pkg/api"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/utils"
)

func (c *Config) CreateDiffSubscribeRequest(cmd *cobra.Command) (*gnmi.SubscribeRequest, error) {
	sc := &types.SubscriptionConfig{
		Name:     "diff-sub",
		Models:   c.DiffModel,
		Prefix:   c.DiffPrefix,
		Target:   c.DiffTarget,
		Paths:    c.DiffPath,
		Mode:     "ONCE",
		Encoding: &c.Encoding,
	}
	if flagIsSet(cmd, "qos") {
		sc.Qos = &c.DiffQos
	}
	return utils.CreateSubscribeRequest(sc, nil, c.Encoding)
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
