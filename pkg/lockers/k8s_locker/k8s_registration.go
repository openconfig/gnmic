// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package k8s_locker

import (
	"context"

	"github.com/openconfig/gnmic/pkg/lockers"
)

func (k *k8sLocker) Register(ctx context.Context, s *lockers.ServiceRegistration) error {
	return nil
}

func (k *k8sLocker) Deregister(s string) error {
	return nil
}
