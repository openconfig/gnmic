// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package store

import "time"

// State constants shared across all component types.
const (
	IntendedStateEnabled  = "enabled"
	IntendedStateDisabled = "disabled"

	StateRunning  = "running"
	StateStopped  = "stopped"
	StateStarting = "starting"
	StateFailed   = "failed"
	StatePaused   = "paused"
	StateStopping = "stopping"
)

// Kind names used in the state store.
const (
	KindTargets             = "targets"
	KindOutputs             = "outputs"
	KindInputs              = "inputs"
	KindSubscriptions       = "subscriptions"
	KindProcessors          = "processors"
	KindAssignments         = "assignments"
	KindTunnelTargetMatches = "tunnel-target-matches"
)

// ComponentState is the base state shared by all managed components.
type ComponentState struct {
	// Name          string    `json:"name"`
	IntendedState string    `json:"intended-state"`          // enabled|disabled
	State         string    `json:"state"`                   // running|stopped|starting|failed|paused|stopping
	FailedReason  string    `json:"failed-reason,omitempty"` // last error message
	LastUpdated   time.Time `json:"last-updated"`            // timestamp of last state transition
}

// TargetState extends ComponentState with target-specific fields.
type TargetState struct {
	ComponentState
	ConnectionState string            `json:"connection-state,omitempty"` // gRPC connectivity: READY|CONNECTING|TRANSIENT_FAILURE|...
	Subscriptions   map[string]string `json:"subscriptions,omitempty"`    // subscription_name -> running|stopped
}

// OutputState extends ComponentState with output-specific fields.
type OutputState struct {
	ComponentState
}

// InputState extends ComponentState with input-specific fields.
type InputState struct {
	ComponentState
}

// SubscriptionState tracks a subscription's aggregate state across targets.
type SubscriptionState struct {
	ComponentState
	Targets map[string]string `json:"targets,omitempty"` // target_name -> running|stopped|starting|failed|paused|stopping
}
