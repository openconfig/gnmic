// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package outputs

import (
	"log"

	"github.com/openconfig/gnmic/pkg/config/store"
	"github.com/prometheus/client_golang/prometheus"
)

type OutputOptions struct {
	Name        string
	ClusterName string
	Logger      *log.Logger
	Registry    *prometheus.Registry
	Store       store.Store[any]
}

type Option func(*OutputOptions) error

func WithLogger(logger *log.Logger) Option {
	return func(o *OutputOptions) error {
		o.Logger = logger
		return nil
	}
}

func WithRegistry(reg *prometheus.Registry) Option {
	return func(o *OutputOptions) error {
		o.Registry = reg
		return nil
	}
}

func WithName(name string) Option {
	return func(o *OutputOptions) error {
		o.Name = name
		return nil
	}
}

func WithClusterName(name string) Option {
	return func(o *OutputOptions) error {
		o.ClusterName = name
		return nil
	}
}

func WithConfigStore(st store.Store[any]) Option {
	return func(o *OutputOptions) error {
		o.Store = st
		return nil
	}
}
