// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_write_output

import (
	"errors"
	"fmt"
	"hash/fnv"
	"maps"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/outputs"
)

const defaultMessageSamplingCacheSize = 100_000

type messageSamplingConfig struct {
	BySubscription map[string]messageSamplingRule `mapstructure:"by-subscription,omitempty" json:"by-subscription,omitempty"`
	CacheSize      int                            `mapstructure:"cache-size,omitempty" json:"cache-size,omitempty"`
	Spread         bool                           `mapstructure:"spread,omitempty" json:"spread,omitempty"`
}

type messageSamplingRule struct {
	Interval     time.Duration `mapstructure:"interval" json:"interval"`
	MinimumBytes int           `mapstructure:"minimum-bytes,omitempty" json:"minimum-bytes,omitempty"`
}

type sampleState struct {
	instance   string
	acceptedAt time.Time
	next       time.Time
}

type sampleAcceptance struct {
	key        string
	instance   string
	acceptedAt time.Time
	previous   sampleState
}

type subscriptionSampler struct {
	mu       sync.Mutex
	rules    map[string]messageSamplingRule
	spread   bool
	accepted *lru.Cache[string, sampleState]
}

func newSubscriptionSampler(cfg *messageSamplingConfig) (*subscriptionSampler, error) {
	if messageSamplingDisabled(cfg) {
		return nil, nil
	}
	for subscription, rule := range cfg.BySubscription {
		if subscription == "" {
			return nil, errors.New("message-sampling subscription cannot be empty")
		}
		if rule.Interval <= 0 {
			return nil, fmt.Errorf("message-sampling interval for %q must be positive", subscription)
		}
		if rule.MinimumBytes < 0 {
			return nil, fmt.Errorf("message-sampling minimum-bytes for %q cannot be negative", subscription)
		}
	}
	cacheSize := messageSamplingCacheSize(cfg)
	if cacheSize < 0 {
		return nil, errors.New("message-sampling cache-size cannot be negative")
	}
	accepted, err := lru.New[string, sampleState](cacheSize)
	if err != nil {
		return nil, fmt.Errorf("create message-sampling cache: %w", err)
	}
	return &subscriptionSampler{
		rules:    maps.Clone(cfg.BySubscription),
		spread:   cfg.Spread,
		accepted: accepted,
	}, nil
}

func messageSamplingConfigsEqual(a, b *messageSamplingConfig) bool {
	if messageSamplingDisabled(a) || messageSamplingDisabled(b) {
		return messageSamplingDisabled(a) == messageSamplingDisabled(b)
	}
	return messageSamplingCacheSize(a) == messageSamplingCacheSize(b) &&
		a.Spread == b.Spread && maps.Equal(a.BySubscription, b.BySubscription)
}

func messageSamplingDisabled(cfg *messageSamplingConfig) bool {
	return cfg == nil || len(cfg.BySubscription) == 0
}

func messageSamplingCacheSize(cfg *messageSamplingConfig) int {
	if cfg.CacheSize == 0 {
		return defaultMessageSamplingCacheSize
	}
	return cfg.CacheSize
}

func (s *subscriptionSampler) Rollback(acceptance *sampleAcceptance) {
	if s == nil || acceptance == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.accepted.Peek(acceptance.key)
	if !ok || state.instance != acceptance.instance || !state.acceptedAt.Equal(acceptance.acceptedAt) {
		return
	}
	s.accepted.Add(acceptance.key, acceptance.previous)
}

func (s *subscriptionSampler) Allow(
	info outputs.SubscriptionInfo,
	meta outputs.Meta,
	message proto.Message,
	now time.Time,
) (bool, *sampleAcceptance) {
	if s == nil || info.Instance == "" || info.Mode != gnmi.SubscriptionList_STREAM {
		return true, nil
	}
	subscription := meta["subscription-name"]
	rule, ok := s.rules[subscription]
	if !ok {
		return true, nil
	}
	response, ok := message.(*gnmi.SubscribeResponse)
	if !ok {
		return true, nil
	}
	if _, ok := response.Response.(*gnmi.SubscribeResponse_Update); !ok {
		return true, nil
	}
	// Initial synchronization may span multiple Update responses. Preserve
	// every one until the server marks the snapshot complete.
	if !info.InitialSyncComplete {
		return true, nil
	}
	source := meta["source"]
	if source == "" {
		return true, nil
	}
	if proto.Size(message) < rule.MinimumBytes {
		return true, nil
	}
	key := source + "\x00" + subscription

	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.accepted.Get(key)
	if !ok || state.instance != info.Instance {
		state = sampleState{instance: info.Instance}
	}

	previous := state
	if s.spread {
		if state.acceptedAt.IsZero() {
			state.next = nextSpreadTime(key, now.Add(rule.Interval), rule.Interval)
		} else if now.Before(state.next) {
			return false, nil
		} else {
			state.next = state.next.Add((now.Sub(state.next)/rule.Interval + 1) * rule.Interval)
		}
	} else if !state.acceptedAt.IsZero() && now.Sub(state.acceptedAt) < rule.Interval {
		return false, nil
	}
	state.acceptedAt = now
	s.accepted.Add(key, state)
	return true, &sampleAcceptance{
		key:        key,
		instance:   info.Instance,
		acceptedAt: now,
		previous:   previous,
	}
}

func nextSpreadTime(key string, now time.Time, interval time.Duration) time.Time {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(key))
	phase := time.Duration(hash.Sum64() % uint64(interval))
	remainder := time.Duration(now.UnixNano() % int64(interval))
	if remainder < 0 {
		remainder += interval
	}
	delay := (phase - remainder + interval) % interval
	return now.Add(delay)
}
