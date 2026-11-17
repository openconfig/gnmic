package app

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/loaders"
)

// chanLoader hands the loader loop whatever operations the test sends on ch.
type chanLoader struct {
	ch chan *loaders.TargetOperation
}

func (l *chanLoader) Init(context.Context, map[string]interface{}, *slog.Logger, ...loaders.Option) error {
	return nil
}

func (l *chanLoader) RunOnce(context.Context) (map[string]*types.TargetConfig, error) {
	return nil, nil
}

func (l *chanLoader) Start(context.Context) chan *loaders.TargetOperation { return l.ch }

func (l *chanLoader) RegisterMetrics(*prometheus.Registry)                {}
func (l *chanLoader) WithActions(map[string]map[string]interface{})       {}
func (l *chanLoader) WithTargetsDefaults(func(*types.TargetConfig) error) {}

// selectTargets recreates a proxy client from a.Config.Targets whenever the
// name is missing from a.Targets, so a loader delete has to remove the target
// from both maps or the proxy keeps serving it.
func TestProxyLoaderDeleteForgetsTarget(t *testing.T) {
	const name = "device-1"

	a := New()
	t.Cleanup(a.Cfn)

	ch := make(chan *loaders.TargetOperation)
	const loaderType = "proxydeleteloadertest"
	loaders.Register(loaderType, func() loaders.TargetLoader { return &chanLoader{ch: ch} })
	a.Config.Loader = map[string]interface{}{"type": loaderType}

	go a.startLoaderProxy(a.ctx)

	// ch is unbuffered and the loop handles one operation at a time, so once a
	// send returns, the operation sent before it has been applied.
	send := func(op *loaders.TargetOperation) {
		t.Helper()
		select {
		case ch <- op:
		case <-time.After(5 * time.Second):
			t.Fatal("the proxy loader loop did not receive the operation")
		}
	}
	send(&loaders.TargetOperation{Add: map[string]*types.TargetConfig{
		name: {Name: name, Address: "127.0.0.1:1", Insecure: new(bool), Timeout: time.Second},
	}})
	send(&loaders.TargetOperation{Del: []string{name}})
	send(&loaders.TargetOperation{})

	a.configLock.RLock()
	_, ok := a.Config.Targets[name]
	a.configLock.RUnlock()
	if ok {
		t.Errorf("a.Config.Targets still holds %q after the loader deleted it", name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := a.selectTargets(ctx, name)
	if status.Code(err) != codes.NotFound {
		t.Errorf("selectTargets(%q) err = %v, want code %v", name, err, codes.NotFound)
	}
}
