package targets_manager

import (
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/api/target"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// A watcher belongs to one start of a target. Once its connection is closed
// it must exit even if its context is still live, instead of looping on to
// whatever connection a restart of the target has created since.
func TestWatchConnState_returnsWhenItsConnShutsDown(t *testing.T) {
	tm := newTargetsTestManager(t)
	addr := freeAddr(t)
	serveGNMI(t, addr)

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	conn.Connect()
	// The target has meanwhile been restarted with a connection of its own.
	mt := &ManagedTarget{Name: "t1", T: target.NewTarget(retryTestConfig("t1", addr))}
	if err := mt.T.CreateGNMIClient(t.Context()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mt.T.Close() })

	done := make(chan struct{})
	go func() {
		tm.watchConnState(t.Context(), mt, conn)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)
	_ = conn.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher still running after its connection shut down")
	}
}

func TestWatchConnState_nilConn(t *testing.T) {
	tm := newTargetsTestManager(t)
	mt := &ManagedTarget{Name: "t1", T: target.NewTarget(retryTestConfig("t1", "127.0.0.1:1"))}

	done := make(chan struct{})
	go func() {
		tm.watchConnState(t.Context(), mt, nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watcher without a connection did not return")
	}
}
