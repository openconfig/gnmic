// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"maps"

	"github.com/openconfig/gnmic/pkg/api/target"
)

func (a *App) targetsSnapshot() map[string]*target.Target {
	a.operLock.RLock()
	defer a.operLock.RUnlock()
	return maps.Clone(a.Targets)
}

func (a *App) targetByName(name string) (*target.Target, bool) {
	a.operLock.RLock()
	defer a.operLock.RUnlock()
	t, ok := a.Targets[name]
	return t, ok
}
