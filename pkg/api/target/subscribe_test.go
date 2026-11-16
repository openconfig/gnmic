package target

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// TestSubscriptionsConcurrentAccess is a regression test for
// https://github.com/openconfig/gnmic/issues/835: concurrent writes to
// Target.Subscriptions (from the collector's targets manager) raced with
// locked reads in SubscribeClientStates. Run with -race.
func TestSubscriptionsConcurrentAccess(t *testing.T) {
	tg := NewTarget(&types.TargetConfig{Name: "t1"})

	var wg sync.WaitGroup
	const iterations = 1000

	wg.Add(3)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			name := fmt.Sprintf("sub%d", i%10)
			tg.SetSubscriptionConfig(&types.SubscriptionConfig{Name: name})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			tg.DeleteSubscriptionConfig(fmt.Sprintf("sub%d", i%10))
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = tg.SubscribeClientStates()
		}
	}()
	wg.Wait()
}

const (
	testRetryTimer = 20 * time.Millisecond
	testWait       = 5 * time.Second
)

type fakeGNMIServer struct {
	gnmi.UnimplementedGNMIServer
	calls atomic.Int32
	// handler serves the n-th Subscribe RPC (starting at 1) after the
	// initial SubscribeRequest is received.
	handler func(n int32, stream gnmi.GNMI_SubscribeServer) error
}

func (s *fakeGNMIServer) Subscribe(stream gnmi.GNMI_SubscribeServer) error {
	n := s.calls.Add(1)
	if _, err := stream.Recv(); err != nil {
		return err
	}
	return s.handler(n, stream)
}

// newBufconnTarget returns a Target connected to srv over an in-memory gRPC
// connection, along with that connection.
func newBufconnTarget(t *testing.T, srv *fakeGNMIServer) (*Target, *grpc.ClientConn) {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	gs := grpc.NewServer()
	gnmi.RegisterGNMIServer(gs, srv)
	go gs.Serve(lis)
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("failed to create client conn: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	tg := NewTarget(&types.TargetConfig{Name: "t1", RetryTimer: testRetryTimer})
	tg.conn = conn
	tg.Client = gnmi.NewGNMIClient(conn)
	return tg, conn
}

func subscribeRequest(mode gnmi.SubscriptionList_Mode) *gnmi.SubscribeRequest {
	return &gnmi.SubscribeRequest{
		Request: &gnmi.SubscribeRequest_Subscribe{
			Subscribe: &gnmi.SubscriptionList{Mode: mode},
		},
	}
}

func sendUpdate(stream gnmi.GNMI_SubscribeServer, n int32) error {
	return stream.Send(&gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_Update{
			Update: &gnmi.Notification{Timestamp: int64(n)},
		},
	})
}

func sendSync(stream gnmi.GNMI_SubscribeServer) error {
	return stream.Send(&gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_SyncResponse{SyncResponse: true},
	})
}

// updateAndHold sends one update and keeps the stream open until the client
// goes away.
func updateAndHold(n int32, stream gnmi.GNMI_SubscribeServer) error {
	if err := sendUpdate(stream, n); err != nil {
		return err
	}
	<-stream.Context().Done()
	return nil
}

func recvResponse(t *testing.T, rspCh chan *SubscribeResponse, errCh chan *TargetError) *SubscribeResponse {
	t.Helper()
	select {
	case rsp, ok := <-rspCh:
		if !ok {
			t.Fatal("response channel closed before a response was received")
		}
		return rsp
	case terr, ok := <-errCh:
		if !ok {
			t.Fatal("error channel closed before a response was received")
		}
		t.Fatalf("unexpected subscription error: %v", terr.Err)
	case <-time.After(testWait):
		t.Fatal("timed out waiting for a response")
	}
	return nil
}

// waitClosed waits for both channels to close and fails on any error
// reported meanwhile.
func waitClosed(t *testing.T, rspCh chan *SubscribeResponse, errCh chan *TargetError) {
	t.Helper()
	timeout := time.After(testWait)
	for rspCh != nil || errCh != nil {
		select {
		case _, ok := <-rspCh:
			if !ok {
				rspCh = nil
			}
		case terr, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			t.Errorf("unexpected subscription error: %v", terr.Err)
		case <-timeout:
			t.Fatal("timed out waiting for subscription channels to close")
		}
	}
}

func TestSubscribeChanRetriesRemoteCanceled(t *testing.T) {
	tests := []struct {
		name string
		mode gnmi.SubscriptionList_Mode
		// serve answers every attempt after the cancelled first one
		serve func(n int32, stream gnmi.GNMI_SubscribeServer) error
	}{
		{
			name:  "stream",
			mode:  gnmi.SubscriptionList_STREAM,
			serve: updateAndHold,
		},
		{
			name: "once",
			mode: gnmi.SubscriptionList_ONCE,
			serve: func(n int32, stream gnmi.GNMI_SubscribeServer) error {
				if err := sendUpdate(stream, n); err != nil {
					return err
				}
				return sendSync(stream)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &fakeGNMIServer{
				handler: func(n int32, stream gnmi.GNMI_SubscribeServer) error {
					if n == 1 {
						return status.Error(codes.Canceled, "cancelled by server")
					}
					return tt.serve(n, stream)
				},
			}
			tg, _ := newBufconnTarget(t, srv)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			start := time.Now()
			rspCh, errCh := tg.SubscribeChan(ctx, subscribeRequest(tt.mode), "sub1")

			select {
			case terr, ok := <-errCh:
				if !ok {
					t.Fatal("subscription stopped without reporting the remote cancellation")
				}
				if code := status.Code(terr.Err); code != codes.Canceled {
					t.Fatalf("expected a CANCELED error, got %v: %v", code, terr.Err)
				}
				if terr.SubscriptionName != "sub1" {
					t.Fatalf("unexpected subscription name %q", terr.SubscriptionName)
				}
			case _, ok := <-rspCh:
				t.Fatalf("expected an error before any response (channel open=%v)", ok)
			case <-time.After(testWait):
				t.Fatal("timed out waiting for the remote cancellation error")
			}

			rsp := recvResponse(t, rspCh, errCh)
			if ts := rsp.Response.GetUpdate().GetTimestamp(); ts != 2 {
				t.Fatalf("expected the update from the second attempt, got timestamp %d", ts)
			}
			if elapsed := time.Since(start); elapsed < testRetryTimer {
				t.Fatalf("retried after %s, before the configured retry timer %s", elapsed, testRetryTimer)
			}
			if got := srv.calls.Load(); got != 2 {
				t.Fatalf("expected 2 Subscribe RPCs, got %d", got)
			}

			cancel()
			waitClosed(t, rspCh, errCh)
		})
	}
}

func TestSubscribeChanIntentionalCancelStops(t *testing.T) {
	tests := []struct {
		name string
		stop func(cancel context.CancelFunc, tg *Target, conn *grpc.ClientConn)
	}{
		{
			name: "parent context",
			stop: func(cancel context.CancelFunc, _ *Target, _ *grpc.ClientConn) { cancel() },
		},
		{
			name: "StopSubscription",
			stop: func(_ context.CancelFunc, tg *Target, _ *grpc.ClientConn) { tg.StopSubscription("sub1") },
		},
		{
			name: "connection closed",
			stop: func(_ context.CancelFunc, _ *Target, conn *grpc.ClientConn) { conn.Close() },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &fakeGNMIServer{handler: updateAndHold}
			tg, conn := newBufconnTarget(t, srv)
			tg.SetSubscriptionConfig(&types.SubscriptionConfig{Name: "sub1"})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			rspCh, errCh := tg.SubscribeChan(ctx, subscribeRequest(gnmi.SubscriptionList_STREAM), "sub1")
			recvResponse(t, rspCh, errCh)

			tt.stop(cancel, tg, conn)
			waitClosed(t, rspCh, errCh)

			if got := srv.calls.Load(); got != 1 {
				t.Fatalf("expected no retry after intentional cancellation, got %d Subscribe RPCs", got)
			}
			if states := tg.SubscribeClientStates(); states["sub1"] {
				t.Fatal("subscribe client still registered after the subscription stopped")
			}
		})
	}
}

func TestSubscribeChanReplacementKeepsNewAttempt(t *testing.T) {
	srv := &fakeGNMIServer{handler: updateAndHold}
	tg, _ := newBufconnTarget(t, srv)
	tg.SetSubscriptionConfig(&types.SubscriptionConfig{Name: "sub1"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := subscribeRequest(gnmi.SubscriptionList_STREAM)
	oldRspCh, oldErrCh := tg.SubscribeChan(ctx, req, "sub1")
	recvResponse(t, oldRspCh, oldErrCh)

	newRspCh, newErrCh := tg.SubscribeChan(ctx, req, "sub1")
	recvResponse(t, newRspCh, newErrCh)

	// the replaced attempt stops silently and does not retry
	waitClosed(t, oldRspCh, oldErrCh)

	// the replaced attempt's cleanup must leave the replacement running
	select {
	case _, ok := <-newRspCh:
		t.Fatalf("replacement subscription disturbed (response channel open=%v)", ok)
	case terr, ok := <-newErrCh:
		if ok {
			t.Fatalf("replacement subscription reported an error: %v", terr.Err)
		}
		t.Fatal("replacement subscription stopped")
	case <-time.After(5 * testRetryTimer):
	}
	if got := srv.calls.Load(); got != 2 {
		t.Fatalf("expected 2 Subscribe RPCs, got %d", got)
	}
	if states := tg.SubscribeClientStates(); !states["sub1"] {
		t.Fatal("replacement subscribe client was unregistered by the replaced attempt")
	}

	tg.StopSubscription("sub1")
	waitClosed(t, newRspCh, newErrCh)
	if got := srv.calls.Load(); got != 2 {
		t.Fatalf("expected no retry after StopSubscription, got %d Subscribe RPCs", got)
	}
}
