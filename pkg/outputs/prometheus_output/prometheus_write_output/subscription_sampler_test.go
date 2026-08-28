// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_write_output

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/outputs"
)

func TestSubscriptionSampler(t *testing.T) {
	sampler, err := newSubscriptionSampler(&messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: 10 * time.Second},
		},
		CacheSize: 2,
	})
	if err != nil {
		t.Fatalf("newSubscriptionSampler() error = %v", err)
	}
	now := time.Unix(100, 0)

	tests := []struct {
		name string
		meta outputs.Meta
		at   time.Time
		want bool
	}{
		{
			name: "unconfigured subscription",
			meta: outputs.Meta{"source": "leaf-1", "subscription-name": "state"},
			at:   now,
			want: true,
		},
		{
			name: "missing source",
			meta: outputs.Meta{"subscription-name": "counters"},
			at:   now,
			want: true,
		},
		{
			name: "first message",
			meta: outputs.Meta{"source": "leaf-1", "subscription-name": "counters"},
			at:   now,
			want: true,
		},
		{
			name: "same stream inside interval",
			meta: outputs.Meta{"source": "leaf-1", "subscription-name": "counters"},
			at:   now.Add(9 * time.Second),
			want: false,
		},
		{
			name: "different source",
			meta: outputs.Meta{"source": "leaf-2", "subscription-name": "counters"},
			at:   now.Add(9 * time.Second),
			want: true,
		},
		{
			name: "interval elapsed",
			meta: outputs.Meta{"source": "leaf-1", "subscription-name": "counters"},
			at:   now.Add(10 * time.Second),
			want: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sampler.Allow(test.meta, 0, test.at); got != test.want {
				t.Fatalf("Allow() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestSubscriptionSamplerAllowsOneConcurrentMessage(t *testing.T) {
	sampler, err := newSubscriptionSampler(&messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
	})
	if err != nil {
		t.Fatalf("newSubscriptionSampler() error = %v", err)
	}
	meta := outputs.Meta{"source": "leaf-1", "subscription-name": "counters"}
	now := time.Unix(100, 0)
	var accepted atomic.Int64
	var wg sync.WaitGroup
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if sampler.Allow(meta, 0, now) {
				accepted.Add(1)
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

func TestSubscriptionSamplerRollback(t *testing.T) {
	sampler, err := newSubscriptionSampler(&messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
	})
	if err != nil {
		t.Fatalf("newSubscriptionSampler() error = %v", err)
	}
	meta := outputs.Meta{"source": "leaf-1", "subscription-name": "counters"}
	acceptedAt := time.Unix(100, 0)
	if !sampler.Allow(meta, 0, acceptedAt) {
		t.Fatal("first message was not accepted")
	}
	sampler.Rollback(meta, acceptedAt)
	if !sampler.Allow(meta, 0, acceptedAt.Add(time.Second)) {
		t.Fatal("message after rollback was not accepted")
	}

	// A stale rollback must not remove a newer acceptance.
	sampler.Rollback(meta, acceptedAt)
	if sampler.Allow(meta, 0, acceptedAt.Add(2*time.Second)) {
		t.Fatal("stale rollback removed newer acceptance")
	}
}

func TestSubscriptionSamplerSpreadAcceptsInitialBaseline(t *testing.T) {
	const interval = 10 * time.Second
	sampler, err := newSubscriptionSampler(&messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: interval},
		},
		Spread: true,
	})
	if err != nil {
		t.Fatalf("newSubscriptionSampler() error = %v", err)
	}
	meta := outputs.Meta{"source": "leaf-1", "subscription-name": "counters"}
	key := "leaf-1\x00counters"
	now := time.Unix(123, 456)
	next := nextSpreadTime(key, now.Add(interval), interval)

	if !sampler.Allow(meta, 0, now) {
		t.Fatal("initial baseline was not accepted")
	}
	if sampler.Allow(meta, 0, next.Add(-time.Nanosecond)) {
		t.Fatal("second message before stable phase was accepted")
	}
	if !sampler.Allow(meta, 0, next) {
		t.Fatal("message at stable phase was not accepted")
	}
	if sampler.Allow(meta, 0, next.Add(interval-time.Nanosecond)) {
		t.Fatal("second message inside interval was accepted")
	}
	if !sampler.Allow(meta, 0, next.Add(interval)) {
		t.Fatal("message in next stable phase was not accepted")
	}
}

func TestSubscriptionSamplerSpreadRollbackRetriesInitialBaseline(t *testing.T) {
	sampler, err := newSubscriptionSampler(&messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: time.Minute},
		},
		Spread: true,
	})
	if err != nil {
		t.Fatalf("newSubscriptionSampler() error = %v", err)
	}
	meta := outputs.Meta{"source": "leaf-1", "subscription-name": "counters"}
	acceptedAt := time.Unix(100, 0)
	if !sampler.Allow(meta, 0, acceptedAt) {
		t.Fatal("initial baseline was not accepted")
	}

	sampler.Rollback(meta, acceptedAt)
	if !sampler.Allow(meta, 0, acceptedAt.Add(time.Second)) {
		t.Fatal("initial baseline was not accepted after enqueue rollback")
	}
}

func TestSubscriptionSamplerPreservesNarrowUpdates(t *testing.T) {
	sampler, err := newSubscriptionSampler(&messageSamplingConfig{
		BySubscription: map[string]messageSamplingRule{
			"counters": {Interval: 10 * time.Second, MinimumBytes: 1024},
		},
	})
	if err != nil {
		t.Fatalf("newSubscriptionSampler() error = %v", err)
	}
	meta := outputs.Meta{"source": "leaf-1", "subscription-name": "counters"}
	now := time.Unix(100, 0)

	if !sampler.Allow(meta, 512, now) || !sampler.Allow(meta, 512, now.Add(time.Second)) {
		t.Fatal("narrow incremental update was sampled")
	}
	if !sampler.Allow(meta, 2048, now.Add(2*time.Second)) {
		t.Fatal("first wide message was not accepted")
	}
	if sampler.Allow(meta, 2048, now.Add(3*time.Second)) {
		t.Fatal("second wide message inside interval was accepted")
	}
	if !sampler.Allow(meta, 512, now.Add(4*time.Second)) {
		t.Fatal("narrow update inside wide-message interval was sampled")
	}
}
