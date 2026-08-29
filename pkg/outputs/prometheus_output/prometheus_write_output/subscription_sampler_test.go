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
	leaf1 := outputs.Meta{"source": "leaf-1", "subscription-name": "counters"}
	leaf2 := outputs.Meta{"source": "leaf-2", "subscription-name": "counters"}
	info := streamInfo("stream-1")

	assertSampleDecision(t, sampler, info, leaf1, syncResponse(), now, true, false)
	assertSampleDecision(t, sampler, info, leaf1, updateResponse(nil), now, true, true)
	assertSampleDecision(t, sampler, info, leaf1, updateResponse(nil), now.Add(9*time.Second), false, false)
	assertSampleDecision(t, sampler, info, leaf1, updateResponse(nil), now.Add(10*time.Second), true, true)

	assertSampleDecision(t, sampler, info, leaf2, syncResponse(), now, true, false)
	assertSampleDecision(t, sampler, info, leaf2, updateResponse(nil), now.Add(time.Second), true, true)
	assertSampleDecision(t, sampler, info, outputs.Meta{
		"source": "leaf-1", "subscription-name": "state",
	}, updateResponse(nil), now, true, false)
	assertSampleDecision(t, sampler, info, outputs.Meta{
		"subscription-name": "counters",
	}, updateResponse(nil), now, true, false)
}

func TestSubscriptionSamplerPreservesInitialSync(t *testing.T) {
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
	})
	meta := outputs.Meta{"source": "leaf-1", "subscription-name": "counters"}
	initialInfo := initialStreamInfo("stream-1")
	info := streamInfo("stream-1")
	now := time.Unix(100, 0)

	assertSampleDecision(t, sampler, initialInfo, meta, updateResponse(nil), now, true, false)
	assertSampleDecision(t, sampler, initialInfo, meta, updateResponse(nil), now.Add(time.Second), true, false)
	assertSampleDecision(t, sampler, initialInfo, meta, syncResponse(), now.Add(2*time.Second), true, false)
	assertSampleDecision(t, sampler, info, meta, updateResponse(nil), now.Add(3*time.Second), true, true)
	assertSampleDecision(t, sampler, info, meta, updateResponse(nil), now.Add(4*time.Second), false, false)
}

func TestSubscriptionSamplerResetsAfterReconnect(t *testing.T) {
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
	})
	meta := outputs.Meta{"source": "leaf-1", "subscription-name": "counters"}
	now := time.Unix(100, 0)

	assertSampleDecision(t, sampler, streamInfo("stream-1"), meta, syncResponse(), now, true, false)
	assertSampleDecision(t, sampler, streamInfo("stream-1"), meta, updateResponse(nil), now, true, true)
	assertSampleDecision(t, sampler, initialStreamInfo("stream-2"), meta, updateResponse(nil), now.Add(time.Second), true, false)
	assertSampleDecision(t, sampler, initialStreamInfo("stream-2"), meta, updateResponse(nil), now.Add(2*time.Second), true, false)
	assertSampleDecision(t, sampler, initialStreamInfo("stream-2"), meta, syncResponse(), now.Add(3*time.Second), true, false)
	assertSampleDecision(t, sampler, streamInfo("stream-2"), meta, updateResponse(nil), now.Add(4*time.Second), true, true)
}

func TestSubscriptionSamplerControlMessagesDoNotConsumeWindow(t *testing.T) {
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
	})
	meta := outputs.Meta{"source": "leaf-1", "subscription-name": "counters"}
	info := streamInfo("stream-1")
	now := time.Unix(100, 0)

	assertSampleDecision(t, sampler, info, meta, syncResponse(), now, true, false)
	assertSampleDecision(t, sampler, info, meta, updateResponse(nil), now, true, true)
	assertSampleDecision(t, sampler, info, meta, updateResponse(nil), now.Add(time.Second), false, false)
}

func TestSubscriptionSamplerOnlySamplesKnownStreamUpdates(t *testing.T) {
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
	})
	meta := outputs.Meta{"source": "leaf-1", "subscription-name": "counters"}
	now := time.Unix(100, 0)

	assertSampleDecision(t, sampler, outputs.SubscriptionInfo{}, meta, updateResponse(nil), now, true, false)
	assertSampleDecision(t, sampler, outputs.SubscriptionInfo{
		Instance: "once-1", Mode: gnmi.SubscriptionList_ONCE,
	}, meta, updateResponse(nil), now, true, false)
	assertSampleDecision(t, sampler, outputs.SubscriptionInfo{
		Instance: "poll-1", Mode: gnmi.SubscriptionList_POLL,
	}, meta, updateResponse(nil), now, true, false)
	assertSampleDecision(t, sampler, streamInfo("stream-1"), meta, &gnmi.GetResponse{}, now, true, false)
}

func TestSubscriptionSamplerAllowsOneConcurrentMessage(t *testing.T) {
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
	})
	meta := outputs.Meta{"source": "leaf-1", "subscription-name": "counters"}
	info := streamInfo("stream-1")
	now := time.Unix(100, 0)
	assertSampleDecision(t, sampler, info, meta, syncResponse(), now, true, false)

	var accepted atomic.Int64
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed, acceptance := sampler.Allow(info, meta, updateResponse(nil), now)
			if allowed {
				accepted.Add(1)
				if acceptance == nil {
					t.Error("accepted sampled message did not return rollback state")
				}
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
	meta := outputs.Meta{"source": "leaf-1", "subscription-name": "counters"}
	info := streamInfo("stream-1")
	now := time.Unix(100, 0)
	assertSampleDecision(t, sampler, info, meta, syncResponse(), now, true, false)

	allowed, acceptance := sampler.Allow(info, meta, updateResponse(nil), now)
	if !allowed || acceptance == nil {
		t.Fatal("first post-sync update was not sampled")
	}
	sampler.Rollback(acceptance)
	allowed, newer := sampler.Allow(info, meta, updateResponse(nil), now.Add(time.Second))
	if !allowed || newer == nil {
		t.Fatal("message after rollback was not accepted")
	}

	// A stale rollback must not remove a newer acceptance.
	sampler.Rollback(acceptance)
	assertSampleDecision(t, sampler, info, meta, updateResponse(nil), now.Add(2*time.Second), false, false)
}

func TestSubscriptionSamplerSpread(t *testing.T) {
	const interval = 10 * time.Second
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: interval},
		},
		Spread: true,
	})
	meta := outputs.Meta{"source": "leaf-1", "subscription-name": "counters"}
	info := streamInfo("stream-1")
	key := "leaf-1\x00counters"
	now := time.Unix(123, 456)
	next := nextSpreadTime(key, now.Add(interval), interval)
	assertSampleDecision(t, sampler, info, meta, syncResponse(), now, true, false)

	allowed, acceptance := sampler.Allow(info, meta, updateResponse(nil), now)
	if !allowed || acceptance == nil {
		t.Fatal("first post-sync update was not accepted")
	}
	assertSampleDecision(t, sampler, info, meta, updateResponse(nil), next.Add(-time.Nanosecond), false, false)
	assertSampleDecision(t, sampler, info, meta, updateResponse(nil), next, true, true)
	assertSampleDecision(t, sampler, info, meta, updateResponse(nil), next.Add(interval-time.Nanosecond), false, false)
	assertSampleDecision(t, sampler, info, meta, updateResponse(nil), next.Add(interval), true, true)

	sampler = mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{"counters": {Interval: interval}},
		Spread:         true,
	})
	assertSampleDecision(t, sampler, info, meta, syncResponse(), now, true, false)
	allowed, acceptance = sampler.Allow(info, meta, updateResponse(nil), now)
	sampler.Rollback(acceptance)
	assertSampleDecision(t, sampler, info, meta, updateResponse(nil), now.Add(time.Second), true, true)
}

func TestSubscriptionSamplerPreservesNarrowUpdates(t *testing.T) {
	sampler := mustSubscriptionSampler(t, &messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: 10 * time.Second, MinimumBytes: 1024},
		},
	})
	meta := outputs.Meta{"source": "leaf-1", "subscription-name": "counters"}
	info := streamInfo("stream-1")
	now := time.Unix(100, 0)
	assertSampleDecision(t, sampler, info, meta, syncResponse(), now, true, false)

	assertSampleDecision(t, sampler, info, meta, updateResponse(nil), now, true, false)
	assertSampleDecision(t, sampler, info, meta, updateResponse(nil), now.Add(time.Second), true, false)
	wide := updateResponse(make([]byte, 2048))
	assertSampleDecision(t, sampler, info, meta, wide, now.Add(2*time.Second), true, true)
	assertSampleDecision(t, sampler, info, meta, wide, now.Add(3*time.Second), false, false)
	assertSampleDecision(t, sampler, info, meta, updateResponse(nil), now.Add(4*time.Second), true, false)
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
	return outputs.SubscriptionInfo{
		Instance:            instance,
		Mode:                gnmi.SubscriptionList_STREAM,
		InitialSyncComplete: true,
	}
}

func initialStreamInfo(instance string) outputs.SubscriptionInfo {
	return outputs.SubscriptionInfo{Instance: instance, Mode: gnmi.SubscriptionList_STREAM}
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
	meta outputs.Meta,
	message proto.Message,
	now time.Time,
	wantAllowed bool,
	wantRecorded bool,
) {
	t.Helper()
	allowed, acceptance := sampler.Allow(info, meta, message, now)
	if allowed != wantAllowed {
		t.Fatalf("Allow() = %v, want %v", allowed, wantAllowed)
	}
	if gotRecorded := acceptance != nil; gotRecorded != wantRecorded {
		t.Fatalf("Allow() recorded acceptance = %v, want %v", gotRecorded, wantRecorded)
	}
}
