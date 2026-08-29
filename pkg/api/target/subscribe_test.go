package target

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api/types"
)

type subscribeClientStub struct {
	gnmi.GNMI_SubscribeClient
	responses []*gnmi.SubscribeResponse
}

func (s *subscribeClientStub) Recv() (*gnmi.SubscribeResponse, error) {
	if len(s.responses) == 0 {
		return nil, io.EOF
	}
	response := s.responses[0]
	s.responses = s.responses[1:]
	return response, nil
}

func TestStreamSubscriptionResponseInstance(t *testing.T) {
	target := &Target{}
	responses := make(chan *SubscribeResponse, 3)
	stream := &subscribeClientStub{responses: []*gnmi.SubscribeResponse{
		{Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{}}},
		{Response: &gnmi.SubscribeResponse_SyncResponse{SyncResponse: true}},
	}}
	if err := target.handleStreamSubscriptionRcv(context.Background(), stream, "sub1", nil, responses); !errors.Is(err, io.EOF) {
		t.Fatalf("handleStreamSubscriptionRcv() error = %v, want EOF", err)
	}
	first := <-responses
	second := <-responses
	if first.SubscriptionInstance == "" || first.SubscriptionInstance != second.SubscriptionInstance {
		t.Fatalf("subscription instance mismatch: %q, %q", first.SubscriptionInstance, second.SubscriptionInstance)
	}
	if first.SubscriptionMode != gnmi.SubscriptionList_STREAM || second.SubscriptionMode != gnmi.SubscriptionList_STREAM {
		t.Fatalf("subscription mode = %v, %v, want STREAM", first.SubscriptionMode, second.SubscriptionMode)
	}
	if first.InitialSyncComplete || !second.InitialSyncComplete {
		t.Fatalf("initial sync state = %v, %v, want false, true", first.InitialSyncComplete, second.InitialSyncComplete)
	}

	nextStream := &subscribeClientStub{responses: []*gnmi.SubscribeResponse{
		{Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{}}},
	}}
	if err := target.handleStreamSubscriptionRcv(context.Background(), nextStream, "sub1", nil, responses); !errors.Is(err, io.EOF) {
		t.Fatalf("second handleStreamSubscriptionRcv() error = %v, want EOF", err)
	}
	third := <-responses
	if third.SubscriptionInstance == first.SubscriptionInstance {
		t.Fatal("new subscription attempt reused its instance")
	}
	if third.InitialSyncComplete {
		t.Fatal("new subscription attempt inherited initial sync state")
	}
}

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
