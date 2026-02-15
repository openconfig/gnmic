// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package store

import (
	zstore "github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
)

// Store wraps both the config store and the state store.
// The config store holds user-defined configuration (targets, subscriptions, outputs, inputs, etc.).
// The state store holds runtime state for each component (running, stopped, failed, etc.).
type Store struct {
	Config zstore.Store[any]
	State  zstore.Store[any]
}

// NewStore creates a new Store with the given config store and a fresh
// in-memory state store.
func NewStore(configStore zstore.Store[any]) *Store {
	return &Store{
		Config: configStore,
		State:  gomap.NewMemStore(zstore.StoreOptions[any]{}),
	}
}
