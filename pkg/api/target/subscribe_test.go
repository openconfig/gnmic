// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// fakeSubscribeClient is a minimal gnmi.GNMI_SubscribeClient (a
// grpc.BidiStreamingClient) used to drive Target's subscription retry logic
// without a real gRPC connection.
type fakeSubscribeClient struct {
	ctx    context.Context
	sendFn func(*gnmi.SubscribeRequest) error
	recvFn func() (*gnmi.SubscribeResponse, error)
}

func (f *fakeSubscribeClient) Send(req *gnmi.SubscribeRequest) error {
	if f.sendFn != nil {
		return f.sendFn(req)
	}
	return nil
}

func (f *fakeSubscribeClient) Recv() (*gnmi.SubscribeResponse, error) {
	if f.recvFn != nil {
		return f.recvFn()
	}
	<-f.ctx.Done()
	return nil, f.ctx.Err()
}

func (f *fakeSubscribeClient) Header() (metadata.MD, error) { return nil, nil }
func (f *fakeSubscribeClient) Trailer() metadata.MD         { return nil }
func (f *fakeSubscribeClient) CloseSend() error             { return nil }
func (f *fakeSubscribeClient) Context() context.Context     { return f.ctx }
func (f *fakeSubscribeClient) SendMsg(m any) error          { return nil }
func (f *fakeSubscribeClient) RecvMsg(m any) error          { return nil }

// fakeGNMIClient is a minimal gnmi.GNMIClient whose Subscribe method is
// scripted by newStreamFn, so tests can simulate a target returning terminal
// or transient errors on demand.
type fakeGNMIClient struct {
	mu           sync.Mutex
	subscribeCnt int
	newStreamFn  func(callN int, ctx context.Context) (gnmi.GNMI_SubscribeClient, error)
}

func (f *fakeGNMIClient) Subscribe(ctx context.Context, _ ...grpc.CallOption) (gnmi.GNMI_SubscribeClient, error) {
	f.mu.Lock()
	f.subscribeCnt++
	n := f.subscribeCnt
	f.mu.Unlock()
	return f.newStreamFn(n, ctx)
}

func (f *fakeGNMIClient) subscribeCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.subscribeCnt
}

func (f *fakeGNMIClient) Capabilities(ctx context.Context, in *gnmi.CapabilityRequest, opts ...grpc.CallOption) (*gnmi.CapabilityResponse, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeGNMIClient) Get(ctx context.Context, in *gnmi.GetRequest, opts ...grpc.CallOption) (*gnmi.GetResponse, error) {
	return nil, errors.New("not implemented")
}

func (f *fakeGNMIClient) Set(ctx context.Context, in *gnmi.SetRequest, opts ...grpc.CallOption) (*gnmi.SetResponse, error) {
	return nil, errors.New("not implemented")
}

func newTestTarget(retryTimer time.Duration) *Target {
	t := NewTarget(&types.TargetConfig{
		Name:       "test-target",
		RetryTimer: retryTimer,
		BufferSize: 10,
	})
	return t
}

func streamReq() *gnmi.SubscribeRequest {
	return &gnmi.SubscribeRequest{
		Request: &gnmi.SubscribeRequest_Subscribe{
			Subscribe: &gnmi.SubscriptionList{
				Mode: gnmi.SubscriptionList_STREAM,
			},
		},
	}
}

// drain reads from errCh until it is closed or the deadline elapses,
// returning every received error and whether the channel was actually
// observed to close.
func drainErrors(t *testing.T, errCh chan *TargetError, timeout time.Duration) ([]*TargetError, bool) {
	t.Helper()
	var errs []*TargetError
	deadline := time.After(timeout)
	for {
		select {
		case e, ok := <-errCh:
			if !ok {
				return errs, true
			}
			errs = append(errs, e)
		case <-deadline:
			return errs, false
		}
	}
}

func TestIsRetryableSubscribeError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		// terminal request/authentication errors: must not retry
		{"nil_error", nil, false},
		{"canceled", status.Error(codes.Canceled, "canceled"), false},
		{"raw_context_canceled", context.Canceled, false},
		{"invalid_argument", status.Error(codes.InvalidArgument, "bad path"), false},
		{"not_found", status.Error(codes.NotFound, "not found"), false},
		{"already_exists", status.Error(codes.AlreadyExists, "already exists"), false},
		{"permission_denied", status.Error(codes.PermissionDenied, "denied"), false},
		{"resource_exhausted", status.Error(codes.ResourceExhausted, "exhausted"), false},
		{"failed_precondition", status.Error(codes.FailedPrecondition, "precondition"), false},
		{"out_of_range", status.Error(codes.OutOfRange, "out of range"), false},
		{"unimplemented", status.Error(codes.Unimplemented, "unimplemented"), false},
		{"data_loss", status.Error(codes.DataLoss, "data loss"), false},
		{"unauthenticated", status.Error(codes.Unauthenticated, "unauthenticated"), false},

		// transient errors: must keep retrying
		{"deadline_exceeded", status.Error(codes.DeadlineExceeded, "timeout"), true},
		{"aborted", status.Error(codes.Aborted, "aborted"), true},
		{"unavailable", status.Error(codes.Unavailable, "unavailable"), true},
		{"unknown", status.Error(codes.Unknown, "unknown"), true},
		{"internal", status.Error(codes.Internal, "internal"), true},

		// no canonical gRPC status: preserve backward-compatible retry behavior
		{"plain_error", errors.New("connection reset by peer"), true},
		{"io_eof_like_plain_error", fmt.Errorf("stream closed"), true},

		// wrapped gRPC status errors must still classify correctly
		{"wrapped_permission_denied", fmt.Errorf("attempt failed: %w", status.Error(codes.PermissionDenied, "denied")), false},
		{"wrapped_resource_exhausted", fmt.Errorf("attempt failed: %w", status.Error(codes.ResourceExhausted, "exhausted")), false},
		{"wrapped_unavailable", fmt.Errorf("attempt failed: %w", status.Error(codes.Unavailable, "unavailable")), true},
		{"doubly_wrapped_permission_denied", fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", status.Error(codes.PermissionDenied, "denied"))), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isRetryableSubscribeError(tt.err)
			if got != tt.want {
				t.Errorf("isRetryableSubscribeError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// TestSubscribeChan_TerminalErrorOnCreate_StopsAfterOneAttempt covers item:
// "PERMISSION_DENIED does not start another attempt" (also exercises
// RESOURCE_EXHAUSTED), for the Subscribe-client-creation call site.
func TestSubscribeChan_TerminalErrorOnCreate_StopsAfterOneAttempt(t *testing.T) {
	for _, code := range []codes.Code{codes.PermissionDenied, codes.ResourceExhausted} {
		t.Run(code.String(), func(t *testing.T) {
			fc := &fakeGNMIClient{
				newStreamFn: func(callN int, ctx context.Context) (gnmi.GNMI_SubscribeClient, error) {
					return nil, status.Error(code, "terminal")
				},
			}
			tg := newTestTarget(5 * time.Millisecond)
			tg.Client = fc

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			_, errCh := tg.SubscribeChan(ctx, streamReq(), "sub1")

			errs, closed := drainErrors(t, errCh, 2*time.Second)
			if !closed {
				t.Fatalf("errCh was not closed; subscription kept retrying a terminal error")
			}
			if len(errs) != 1 {
				t.Fatalf("got %d errors, want exactly 1: %v", len(errs), errs)
			}
			st, ok := status.FromError(errs[0].Err)
			if !ok || st.Code() != code {
				t.Fatalf("reported error = %v, want gRPC status %s", errs[0].Err, code)
			}
			if got := fc.subscribeCalls(); got != 1 {
				t.Fatalf("Client.Subscribe called %d times, want exactly 1 (no retry after terminal error)", got)
			}
		})
	}
}

// TestSubscribeChan_TerminalSendError covers the "failure to send the
// SubscribeRequest" call site with a terminal gRPC error (RESOURCE_EXHAUSTED).
func TestSubscribeChan_TerminalSendError(t *testing.T) {
	fc := &fakeGNMIClient{
		newStreamFn: func(callN int, ctx context.Context) (gnmi.GNMI_SubscribeClient, error) {
			return &fakeSubscribeClient{
				ctx: ctx,
				sendFn: func(*gnmi.SubscribeRequest) error {
					return status.Error(codes.ResourceExhausted, "too many subscriptions")
				},
			}, nil
		},
	}
	tg := newTestTarget(5 * time.Millisecond)
	tg.Client = fc

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, errCh := tg.SubscribeChan(ctx, streamReq(), "sub1")

	errs, closed := drainErrors(t, errCh, 2*time.Second)
	if !closed {
		t.Fatalf("errCh was not closed; subscription kept retrying a terminal send error")
	}
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want exactly 1: %v", len(errs), errs)
	}
	if st, ok := status.FromError(errs[0].Err); !ok || st.Code() != codes.ResourceExhausted {
		t.Fatalf("reported error = %v, want gRPC status RESOURCE_EXHAUSTED", errs[0].Err)
	}
	if got := fc.subscribeCalls(); got != 1 {
		t.Fatalf("Client.Subscribe called %d times, want exactly 1 (no retry after terminal send error)", got)
	}
}

// TestSubscribeChan_UnavailableRemainsRetryable covers the STREAM receive
// call site and item: "UNAVAILABLE remains retryable".
func TestSubscribeChan_UnavailableRemainsRetryable(t *testing.T) {
	fc := &fakeGNMIClient{
		newStreamFn: func(callN int, ctx context.Context) (gnmi.GNMI_SubscribeClient, error) {
			return &fakeSubscribeClient{
				ctx: ctx,
				recvFn: func() (*gnmi.SubscribeResponse, error) {
					return nil, status.Error(codes.Unavailable, "target rebooting")
				},
			}, nil
		},
	}
	tg := newTestTarget(2 * time.Millisecond)
	tg.Client = fc

	ctx, cancel := context.WithCancel(context.Background())
	_, errCh := tg.SubscribeChan(ctx, streamReq(), "sub1")

	// Observe at least two attempts, proving the loop retried after a
	// transient UNAVAILABLE error instead of stopping.
	for i := 0; i < 2; i++ {
		select {
		case e, ok := <-errCh:
			if !ok {
				t.Fatalf("errCh closed after %d attempt(s); UNAVAILABLE should keep retrying", i)
			}
			if st, ok := status.FromError(e.Err); !ok || st.Code() != codes.Unavailable {
				t.Fatalf("reported error = %v, want gRPC status UNAVAILABLE", e.Err)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for attempt %d", i+1)
		}
	}

	cancel()
	if _, closed := drainErrors(t, errCh, 2*time.Second); !closed {
		t.Fatalf("errCh did not close after context cancellation")
	}
	if got := fc.subscribeCalls(); got < 2 {
		t.Fatalf("Client.Subscribe called %d times, want at least 2", got)
	}
}

// TestSubscribeChan_ContextCancellationTerminatesCleanly covers: "context
// cancellation terminates cleanly" -- no error is reported for an
// intentional cancellation, and both channels are closed.
func TestSubscribeChan_ContextCancellationTerminatesCleanly(t *testing.T) {
	var calls int32
	recvStarted := make(chan struct{})
	var closeRecvStarted sync.Once
	fc := &fakeGNMIClient{
		newStreamFn: func(callN int, ctx context.Context) (gnmi.GNMI_SubscribeClient, error) {
			atomic.AddInt32(&calls, 1)
			return &fakeSubscribeClient{
				ctx: ctx,
				recvFn: func() (*gnmi.SubscribeResponse, error) {
					closeRecvStarted.Do(func() { close(recvStarted) })
					// Block until the per-attempt context is canceled, then
					// return the canonical gRPC CANCELED status a real
					// stream would surface, so the test verifies the
					// locally caused CANCELED is treated as silent.
					<-ctx.Done()
					return nil, status.Error(codes.Canceled, "local cancellation")
				},
			}, nil
		},
	}
	tg := newTestTarget(5 * time.Millisecond)
	tg.Client = fc

	ctx, cancel := context.WithCancel(context.Background())
	responseCh, errCh := tg.SubscribeChan(ctx, streamReq(), "sub1")

	// Wait until the goroutine has actually reached the blocking Recv call
	// before canceling, so the cancellation always exercises that path.
	select {
	case <-recvStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Recv to start")
	}
	cancel()

	errs, closed := drainErrors(t, errCh, 2*time.Second)
	if !closed {
		t.Fatalf("errCh did not close after context cancellation")
	}
	if len(errs) != 0 {
		t.Fatalf("got %d errors on cancellation, want 0 (cancellation should be silent): %v", len(errs), errs)
	}

	select {
	case _, ok := <-responseCh:
		if ok {
			t.Fatalf("responseCh delivered an unexpected response")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("responseCh was not closed after context cancellation")
	}

	if atomic.LoadInt32(&calls) < 1 {
		t.Fatalf("expected at least one Subscribe call before cancellation")
	}
}

// TestSubscribeChan_RemoteCanceledStopsAfterOneAttempt covers a target
// remotely returning a gRPC CANCELED status on the STREAM receive call site
// (e.g. the target canceled the RPC server-side) while the local context is
// still active. isLocalCancellation only treats CANCELED as intentional when
// the local context itself is done (see subscribe.go), so this must be
// reported as a terminal error exactly once and must not be confused with a
// locally initiated cancellation, which is silent and produces no error (see
// TestSubscribeChan_ContextCancellationTerminatesCleanly).
func TestSubscribeChan_RemoteCanceledStopsAfterOneAttempt(t *testing.T) {
	fc := &fakeGNMIClient{
		newStreamFn: func(callN int, ctx context.Context) (gnmi.GNMI_SubscribeClient, error) {
			return &fakeSubscribeClient{
				ctx: ctx,
				recvFn: func() (*gnmi.SubscribeResponse, error) {
					return nil, status.Error(codes.Canceled, "remote cancellation")
				},
			}, nil
		},
	}
	tg := newTestTarget(5 * time.Millisecond)
	tg.Client = fc

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	responseCh, errCh := tg.SubscribeChan(ctx, streamReq(), "sub1")

	errs, closed := drainErrors(t, errCh, 2*time.Second)

	// The local context must never have been canceled: the CANCELED status
	// originated remotely, from the fake target, not from us.
	if ctx.Err() != nil {
		t.Fatalf("local context was canceled, want it to remain active: %v", ctx.Err())
	}

	if !closed {
		t.Fatalf("errCh was not closed; retry goroutine did not terminate cleanly")
	}
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want exactly 1 (delivered once): %v", len(errs), errs)
	}
	st, ok := status.FromError(errs[0].Err)
	if !ok || st.Code() != codes.Canceled {
		t.Fatalf("reported error = %v, want gRPC status CANCELED", errs[0].Err)
	}

	select {
	case _, ok := <-responseCh:
		if ok {
			t.Fatalf("responseCh delivered an unexpected response")
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("responseCh was not closed; retry goroutine did not terminate cleanly")
	}

	if got := fc.subscribeCalls(); got != 1 {
		t.Fatalf("Client.Subscribe called %d times, want exactly 1 (no second Subscribe attempt)", got)
	}
}

// TestOldSubscribe_TerminalErrorStopsRetrying exercises the older,
// goto-based Target.Subscribe implementation to ensure it is also fixed:
// a terminal error must be reported once and must not trigger another
// Subscribe attempt.
func TestOldSubscribe_TerminalErrorStopsRetrying(t *testing.T) {
	fc := &fakeGNMIClient{
		newStreamFn: func(callN int, ctx context.Context) (gnmi.GNMI_SubscribeClient, error) {
			return nil, status.Error(codes.PermissionDenied, "denied")
		},
	}
	tg := newTestTarget(5 * time.Millisecond)
	tg.Client = fc

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		tg.Subscribe(ctx, streamReq(), "sub1")
		close(done)
	}()

	select {
	case e := <-tg.errors:
		if st, ok := status.FromError(e.Err); !ok || st.Code() != codes.PermissionDenied {
			t.Fatalf("reported error = %v, want gRPC status PERMISSION_DENIED", e.Err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for the terminal error to be reported")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Subscribe goroutine did not return after a terminal error")
	}

	if got := fc.subscribeCalls(); got != 1 {
		t.Fatalf("Client.Subscribe called %d times, want exactly 1 (no retry after terminal error)", got)
	}
}
