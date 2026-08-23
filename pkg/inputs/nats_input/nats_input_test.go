package nats_input

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/inputs"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/openconfig/gnmic/pkg/pipeline"
	"github.com/zestor-dev/zestor/store"
	"github.com/zestor-dev/zestor/store/gomap"
)

func memStore() store.Store[any] {
	return gomap.NewMemStore(store.StoreOptions[any]{})
}

func TestNatsInput_CloseUnblocksDial(t *testing.T) {
	in := inputs.Inputs["nats"]()
	cfg := map[string]any{
		"address":           "127.0.0.1:1",
		"subject":           "x",
		"format":            "event",
		"connect-time-wait": "50ms",
		"num-workers":       1,
	}
	if err := in.Start(context.Background(), "n1", cfg, inputs.WithConfigStore(memStore())); err != nil {
		t.Fatalf("Start: %v", err)
	}
	done := make(chan struct{})
	go func() {
		_ = in.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Close hung while Dial was retrying a closed port")
	}
}

func TestNatsInput_malformedEventDoesNotBlockPipeline(t *testing.T) {
	pipe := make(chan *pipeline.Msg, 2)
	n := &natsInput{
		confLock: new(sync.RWMutex),
		cfg:      new(atomic.Pointer[config]),
		dynCfg:   new(atomic.Pointer[dynConfig]),
		logger:   logging.DiscardLogger(),
		wg:       new(sync.WaitGroup),
		pipeline: pipe,
	}
	dc := &dynConfig{outputsMap: map[string]struct{}{"obs": {}}}
	n.dynCfg.Store(dc)

	ctx := context.Background()
	if err := n.ingestEventPayload(ctx, "w0", []byte("{not-json"), dc); err != nil {
		t.Fatalf("malformed payload: %v", err)
	}
	select {
	case msg := <-pipe:
		t.Fatalf("malformed payload was forwarded: %+v", msg)
	default:
	}

	if err := n.ingestEventPayload(ctx, "w0", []byte(`[{"name":"ok","timestamp":1,"values":{"v":1}}]`), dc); err != nil {
		t.Fatalf("valid payload: %v", err)
	}
	select {
	case msg := <-pipe:
		if len(msg.Events) != 1 || msg.Events[0].Name != "ok" {
			t.Fatalf("events: %+v", msg.Events)
		}
		if _, ok := msg.Outputs["obs"]; !ok {
			t.Fatalf("outputs: %+v", msg.Outputs)
		}
	case <-time.After(time.Second):
		t.Fatal("valid event after malformed payload never reached the pipeline")
	}
}
