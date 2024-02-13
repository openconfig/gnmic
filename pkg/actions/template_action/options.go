// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package template_action

import (
	"log"
	"os"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
)

func (t *templateAction) WithTargets(map[string]*types.TargetConfig) {}

func (t *templateAction) WithLogger(logger *log.Logger) {
	if t.Debug && logger != nil {
		t.logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
	} else if t.Debug {
		t.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}
