// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package script_action

import (
	"log/slog"

	"github.com/openconfig/gnmic/pkg/actions"
	"github.com/openconfig/gnmic/pkg/api/types"
)

func (s *scriptAction) WithTargets(map[string]*types.TargetConfig) {}

func (s *scriptAction) WithLogger(l *slog.Logger) {
	s.logger = actions.BindLogger(l, actionType, s.Name)
}
