// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_write_output

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/outputs"
)

func TestSubscriptionSampler(t *testing.T) {
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: 10 * time.Second},
		},
		CacheSize: 2,
	})
	now := time.Unix(100, 0)
	leaf1 := streamInfoFor("stream-1", "leaf-1", "counters")
	leaf2 := streamInfoFor("stream-1", "leaf-2", "counters")

	assertSampleDecision(t, sampler, leaf1, syncResponse(), now, true, false)
	assertSampleDecision(t, sampler, leaf1, updateResponse(nil), now, true, true)
	assertSampleDecision(t, sampler, leaf1, updateResponse(nil), now.Add(9*time.Second), false, false)
	assertSampleDecision(t, sampler, leaf1, updateResponse(nil), now.Add(10*time.Second), true, true)

	assertSampleDecision(t, sampler, leaf2, syncResponse(), now, true, false)
	assertSampleDecision(t, sampler, leaf2, updateResponse(nil), now.Add(time.Second), true, true)
	assertSampleDecision(t, sampler, streamInfoFor("stream-1", "leaf-1", "state"), updateResponse(nil), now, true, false)
	assertSampleDecision(t, sampler, streamInfoFor("stream-1", "", "counters"), updateResponse(nil), now, true, false)
}

func TestSubscriptionSamplerPreservesInitialSync(t *testing.T) {
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
	})
	initialInfo := initialStreamInfo("stream-1")
	info := streamInfo("stream-1")
	now := time.Unix(100, 0)

	assertSampleDecision(t, sampler, initialInfo, updateResponse(nil), now, true, false)
	assertSampleDecision(t, sampler, initialInfo, updateResponse(nil), now.Add(time.Second), true, false)
	assertSampleDecision(t, sampler, initialInfo, syncResponse(), now.Add(2*time.Second), true, false)
	assertSampleDecision(t, sampler, info, updateResponse(nil), now.Add(3*time.Second), true, true)
	assertSampleDecision(t, sampler, info, updateResponse(nil), now.Add(4*time.Second), false, false)
}

func TestSubscriptionSamplerResetsAfterReconnect(t *testing.T) {
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
	})
	now := time.Unix(100, 0)

	assertSampleDecision(t, sampler, streamInfo("stream-1"), syncResponse(), now, true, false)
	assertSampleDecision(t, sampler, streamInfo("stream-1"), updateResponse(nil), now, true, true)
	assertSampleDecision(t, sampler, initialStreamInfo("stream-2"), updateResponse(nil), now.Add(time.Second), true, false)
	assertSampleDecision(t, sampler, initialStreamInfo("stream-2"), updateResponse(nil), now.Add(2*time.Second), true, false)
	assertSampleDecision(t, sampler, initialStreamInfo("stream-2"), syncResponse(), now.Add(3*time.Second), true, false)
	assertSampleDecision(t, sampler, streamInfo("stream-2"), updateResponse(nil), now.Add(4*time.Second), true, true)
}

func TestSubscriptionSamplerInterleavedReconnects(t *testing.T) {
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
	})
	now := time.Unix(100, 0)
	oldStream := streamInfo("stream-1")
	newStream := streamInfo("stream-2")

	assertSampleDecision(t, sampler, oldStream, updateResponse(nil), now, true, true)
	assertSampleDecision(t, sampler, newStream, updateResponse(nil), now.Add(time.Second), true, true)
	assertSampleDecision(t, sampler, oldStream, updateResponse(nil), now.Add(2*time.Second), false, false)
	assertSampleDecision(t, sampler, newStream, updateResponse(nil), now.Add(3*time.Second), false, false)
}

func TestSubscriptionSamplerControlMessagesDoNotConsumeWindow(t *testing.T) {
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
	})
	info := streamInfo("stream-1")
	now := time.Unix(100, 0)

	assertSampleDecision(t, sampler, info, syncResponse(), now, true, false)
	assertSampleDecision(t, sampler, info, updateResponse(nil), now, true, true)
	assertSampleDecision(t, sampler, info, updateResponse(nil), now.Add(time.Second), false, false)
}

func TestSubscriptionSamplerOnlySamplesKnownStreamUpdates(t *testing.T) {
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
	})
	now := time.Unix(100, 0)

	assertSampleDecision(t, sampler, outputs.SubscriptionInfo{}, updateResponse(nil), now, true, false)
	onceInfo := streamInfo("once-1")
	onceInfo.Mode = gnmi.SubscriptionList_ONCE
	assertSampleDecision(t, sampler, onceInfo, updateResponse(nil), now, true, false)
	pollInfo := streamInfo("poll-1")
	pollInfo.Mode = gnmi.SubscriptionList_POLL
	assertSampleDecision(t, sampler, pollInfo, updateResponse(nil), now, true, false)
	assertSampleDecision(t, sampler, streamInfo("stream-1"), &gnmi.GetResponse{}, now, true, false)
}

func TestSubscriptionSamplerAllowsOneConcurrentMessage(t *testing.T) {
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
	})
	info := streamInfo("stream-1")
	now := time.Unix(100, 0)
	assertSampleDecision(t, sampler, info, syncResponse(), now, true, false)

	var accepted atomic.Int64
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, reservation := sampler.Reserve(info, updateResponse(nil), now)
			if allowed {
				accepted.Add(1)
				if reservation == nil {
					t.Error("accepted sampled message did not return a reservation")
					return
				}
				sampler.Commit(reservation)
			}
		}()
	}
	wg.Wait()
	if got := accepted.Load(); got != 1 {
		t.Fatalf("accepted = %d, want 1", got)
	}
}

func TestNewSubscriptionSamplerValidation(t *testing.T) {
	tests := []struct {
		name string
		cfg  *messageSamplingConfig
	}{
		{
			name: "empty subscription",
			cfg: &messageSamplingConfig{BySubscription: map[string]messageSamplingRule{
				"": {Interval: time.Second},
			}},
		},
		{
			name: "non-positive interval",
			cfg: &messageSamplingConfig{BySubscription: map[string]messageSamplingRule{
				"counters": {},
			}},
		},
		{
			name: "negative minimum bytes",
			cfg: &messageSamplingConfig{BySubscription: map[string]messageSamplingRule{
				"counters": {Interval: time.Second, MinimumBytes: -1},
			}},
		},
		{
			name: "negative cache size",
			cfg: &messageSamplingConfig{
				BySubscription: map[string]messageSamplingRule{
					"counters": {Interval: time.Second},
				},
				CacheSize: -1,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := newSubscriptionSampler(test.cfg); err == nil {
				t.Fatal("newSubscriptionSampler() error = nil, want error")
			}
		})
	}
}

func TestMessageSamplingConfigsEqual(t *testing.T) {
	rules := map[string]messageSamplingRule{"counters": {Interval: time.Minute}}
	if !messageSamplingConfigsEqual(nil, &messageSamplingConfig{}) {
		t.Fatal("disabled configurations should be equivalent")
	}
	if !messageSamplingConfigsEqual(
		&messageSamplingConfig{BySubscription: rules},
		&messageSamplingConfig{BySubscription: rules, CacheSize: defaultMessageSamplingCacheSize},
	) {
		t.Fatal("default and explicit cache sizes should be equivalent")
	}
	if messageSamplingConfigsEqual(
		&messageSamplingConfig{BySubscription: rules},
		&messageSamplingConfig{BySubscription: rules, Spread: true},
	) {
		t.Fatal("different spread settings should not be equivalent")
	}
}

func TestSubscriptionSamplerRollback(t *testing.T) {
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
	})
	info := streamInfo("stream-1")
	now := time.Unix(100, 0)
	assertSampleDecision(t, sampler, info, syncResponse(), now, true, false)

	allowed, reservation := sampler.Reserve(info, updateResponse(nil), now)
	if !allowed || reservation == nil {
		t.Fatal("first post-sync update was not sampled")
	}
	sampler.Rollback(reservation)
	allowed, newer := sampler.Reserve(info, updateResponse(nil), now.Add(time.Second))
	if !allowed || newer == nil {
		t.Fatal("message after rollback was not accepted")
	}
	sampler.Commit(newer)

	sampler.Rollback(reservation)
	assertSampleDecision(t, sampler, info, updateResponse(nil), now.Add(2*time.Second), false, false)
}

func TestSubscriptionSamplerRollbackDoesNotEvictCommittedState(t *testing.T) {
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
		CacheSize: 1,
	})
	now := time.Unix(100, 0)
	leaf1 := streamInfoFor("stream-1", "leaf-1", "counters")
	leaf2 := streamInfoFor("stream-1", "leaf-2", "counters")
	assertSampleDecision(t, sampler, leaf1, updateResponse(nil), now, true, true)

	allowed, reservation := sampler.Reserve(leaf2, updateResponse(nil), now.Add(time.Second))
	if !allowed || reservation == nil {
		t.Fatal("second source did not reserve a sampling window")
	}
	sampler.Rollback(reservation)

	assertSampleDecision(t, sampler, leaf1, updateResponse(nil), now.Add(2*time.Second), false, false)
}

func TestSubscriptionSamplerSpread(t *testing.T) {
	const interval = 10 * time.Second
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: interval},
		},
		Spread: true,
	})
	info := streamInfo("stream-1")
	key := sampleKey{source: info.Source, subscription: info.Name, instance: info.Instance}
	now := time.Unix(123, 456)
	next := nextSpreadTime(key, now.Add(interval), interval)
	assertSampleDecision(t, sampler, info, syncResponse(), now, true, false)

	allowed, reservation := sampler.Reserve(info, updateResponse(nil), now)
	if !allowed || reservation == nil {
		t.Fatal("first post-sync update was not accepted")
	}
	sampler.Commit(reservation)
	assertSampleDecision(t, sampler, info, updateResponse(nil), next.Add(-time.Nanosecond), false, false)
	assertSampleDecision(t, sampler, info, updateResponse(nil), next, true, true)
	assertSampleDecision(t, sampler, info, updateResponse(nil), next.Add(interval-time.Nanosecond), false, false)
	assertSampleDecision(t, sampler, info, updateResponse(nil), next.Add(interval), true, true)

	sampler = mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{"counters": {Interval: interval}},
		Spread:         true,
	})
	assertSampleDecision(t, sampler, info, syncResponse(), now, true, false)
	allowed, reservation = sampler.Reserve(info, updateResponse(nil), now)
	sampler.Rollback(reservation)
	assertSampleDecision(t, sampler, info, updateResponse(nil), now.Add(time.Second), true, true)
}

func TestSubscriptionSamplerPreservesNarrowUpdates(t *testing.T) {
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: 10 * time.Second, MinimumBytes: 1024},
		},
	})
	info := streamInfo("stream-1")
	now := time.Unix(100, 0)
	assertSampleDecision(t, sampler, info, syncResponse(), now, true, false)

	assertSampleDecision(t, sampler, info, updateResponse(nil), now, true, false)
	assertSampleDecision(t, sampler, info, updateResponse(nil), now.Add(time.Second), true, false)
	wide := updateResponse(make([]byte, 2048))
	assertSampleDecision(t, sampler, info, wide, now.Add(2*time.Second), true, true)
	assertSampleDecision(t, sampler, info, wide, now.Add(3*time.Second), false, false)
	assertSampleDecision(t, sampler, info, updateResponse(nil), now.Add(4*time.Second), true, false)
}

func mustSubscriptionSampler(t *testing.T, cfg *messageSamplingConfig) *subscriptionSampler {
	t.Helper()
	sampler, err := newSubscriptionSampler(cfg)
	if err != nil {
		t.Fatalf("newSubscriptionSampler() error = %v", err)
	}
	return sampler
}

func streamInfo(instance string) outputs.SubscriptionInfo {
	return streamInfoFor(instance, "leaf-1", "counters")
}

func streamInfoFor(instance, source, name string) outputs.SubscriptionInfo {
	return outputs.SubscriptionInfo{
		Source:              source,
		Name:                name,
		Instance:            instance,
		Mode:                gnmi.SubscriptionList_STREAM,
		InitialSyncComplete: true,
	}
}

func initialStreamInfo(instance string) outputs.SubscriptionInfo {
	info := streamInfo(instance)
	info.InitialSyncComplete = false
	return info
}

func syncResponse() *gnmi.SubscribeResponse {
	return &gnmi.SubscribeResponse{Response: &gnmi.SubscribeResponse_SyncResponse{SyncResponse: true}}
}

func updateResponse(value []byte) *gnmi.SubscribeResponse {
	notification := &gnmi.Notification{}
	if value != nil {
		notification.Update = []*gnmi.Update{{Val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_BytesVal{BytesVal: value},
		}}}
	}
	return &gnmi.SubscribeResponse{Response: &gnmi.SubscribeResponse_Update{Update: notification}}
}

func assertSampleDecision(
	t *testing.T,
	sampler *subscriptionSampler,
	info outputs.SubscriptionInfo,
	message proto.Message,
	now time.Time,
	wantAllowed bool,
	wantRecorded bool,
) {
	t.Helper()
	allowed, reservation := sampler.Reserve(info, message, now)
	if allowed != wantAllowed {
		t.Fatalf("Reserve() = %v, want %v", allowed, wantAllowed)
	}
	if gotRecorded := reservation != nil; gotRecorded != wantRecorded {
		t.Fatalf("Reserve() returned reservation = %v, want %v", gotRecorded, wantRecorded)
	}
	if reservation != nil {
		sampler.Commit(reservation)
	}
}

func BenchmarkSubscriptionSamplerRejected(b *testing.B) {
	sampler, err := newSubscriptionSampler(&messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	info := streamInfo("stream-1")
	message := updateResponse(nil)
	now := time.Unix(100, 0)
	allowed, reservation := sampler.Reserve(info, message, now)
	if !allowed || reservation == nil {
		b.Fatal("failed to seed sampler")
	}
	sampler.Commit(reservation)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		allowed, _ := sampler.Reserve(info, message, now.Add(time.Second))
		if allowed {
			b.Fatal("message inside sampling interval was accepted")
		}
	}
}
