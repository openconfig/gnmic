// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package script_action

import (
	"log"
	"os"

	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
)

func (s *scriptAction) WithTargets(map[string]*types.TargetConfig) {}

func (s *scriptAction) WithLogger(logger *log.Logger) {
	if s.Debug && logger != nil {
		s.logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
	} else if s.Debug {
		s.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}
