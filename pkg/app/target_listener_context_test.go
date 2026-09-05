// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"testing"
	"time"
)

func TestTargetListenerContextStopsWithTarget(t *testing.T) {
	stopped := make(chan struct{})
	ctx, cancel := targetListenerContext(context.Background(), stopped)
	defer cancel()
	close(stopped)

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("target listener context was not canceled")
	}
}

func TestTargetListenerContextStopsWithParent(t *testing.T) {
	parent, stopParent := context.WithCancel(context.Background())
	ctx, cancel := targetListenerContext(parent, make(chan struct{}))
	defer cancel()
	stopParent()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("target listener context ignored parent cancellation")
	}
}
