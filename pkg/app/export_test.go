// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/outputs"
	"google.golang.org/protobuf/proto"
)

func TestExportReleasesOperationalLockDuringWrite(t *testing.T) {
	a := New()
	defer a.Cfn()

	started := make(chan struct{})
	release := make(chan struct{})
	a.Outputs["slow"] = &blockingOutput{started: started, release: release}

	done := make(chan struct{})
	go func() {
		a.export(context.Background(), &gnmi.SubscribeResponse{}, nil)
		close(done)
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("Write was not called")
	}

	if !a.operLock.TryLock() {
		t.Fatal("export holds operLock during Write")
	}
	a.operLock.Unlock()

	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("export did not return after Write completed")
	}
}

type blockingOutput struct {
	started chan struct{}
	release chan struct{}
}

func (o *blockingOutput) Init(context.Context, string, map[string]any, ...outputs.Option) error {
	return nil
}
func (o *blockingOutput) Validate(map[string]any) error                    { return nil }
func (o *blockingOutput) Update(context.Context, map[string]any) error     { return nil }
func (o *blockingOutput) UpdateProcessor(string, map[string]any) error     { return nil }
func (o *blockingOutput) WriteEvent(context.Context, *formatters.EventMsg) {}
func (o *blockingOutput) Close() error                                     { return nil }
func (o *blockingOutput) String() string                                   { return "blocking" }

func (o *blockingOutput) Write(context.Context, proto.Message, outputs.Meta) {
	select {
	case <-o.started:
	default:
		close(o.started)
	}
	<-o.release
}
