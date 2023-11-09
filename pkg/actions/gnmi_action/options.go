// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package gnmi_action

import (
	"log"
	"os"

	"github.com/openconfig/gnmic/pkg/types"
	"github.com/openconfig/gnmic/pkg/utils"
)

func (g *gnmiAction) WithTargets(tcs map[string]*types.TargetConfig) {
	if tcs == nil {
		return
	}
	g.targetsConfigs = tcs
}

func (g *gnmiAction) WithLogger(logger *log.Logger) {
	if g.Debug && logger != nil {
		g.logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
	} else if g.Debug {
		g.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}
