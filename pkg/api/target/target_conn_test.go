package target

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	"google.golang.org/grpc"
)

// Readers such as a target's connection watcher or the metrics updater read
// the connection without the caller's lock while a restart replaces it.
func TestConnStateReadersDuringReconnect(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	go func() { _ = srv.Serve(l) }()
	t.Cleanup(srv.Stop)

	insecure := true
	tg := NewTarget(&types.TargetConfig{
		Name:     "t1",
		Address:  l.Addr().String(),
		Insecure: &insecure,
		Timeout:  time.Second,
	})

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = tg.ConnState()
			_ = tg.ConnectivityState()
			_ = tg.connClosing()
		}
	}()

	for range 20 {
		if err := tg.CreateGNMIClient(context.Background()); err != nil {
			t.Fatal(err)
		}
		_ = tg.Close()
	}
	close(stop)
	wg.Wait()
}
