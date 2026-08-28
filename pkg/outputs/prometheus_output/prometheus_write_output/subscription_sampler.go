// © 2026 Nokia.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_write_output

import (
	"errors"
	"fmt"
	"hash/fnv"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"

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
	acceptedAt time.Time
	next       time.Time
}

type subscriptionSampler struct {
	mu       sync.Mutex
	rules    map[string]messageSamplingRule
	spread   bool
	accepted *lru.Cache[string, sampleState]
}

func newSubscriptionSampler(cfg *messageSamplingConfig) (*subscriptionSampler, error) {
	if cfg == nil || len(cfg.BySubscription) == 0 {
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
	cacheSize := cfg.CacheSize
	if cacheSize < 0 {
		return nil, errors.New("message-sampling cache-size cannot be negative")
	}
	if cacheSize == 0 {
		cacheSize = defaultMessageSamplingCacheSize
	}
	accepted, err := lru.New[string, sampleState](cacheSize)
	if err != nil {
		return nil, fmt.Errorf("create message-sampling cache: %w", err)
	}
	return &subscriptionSampler{
		rules:    cfg.BySubscription,
		spread:   cfg.Spread,
		accepted: accepted,
	}, nil
}

func (s *subscriptionSampler) Rollback(meta outputs.Meta, acceptedAt time.Time) {
	if s == nil {
		return
	}
	subscription := meta["subscription-name"]
	if _, ok := s.rules[subscription]; !ok {
		return
	}
	source := meta["source"]
	if source == "" {
		return
	}
	key := source + "\x00" + subscription

	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.accepted.Peek(key)
	if !ok || !state.acceptedAt.Equal(acceptedAt) {
		return
	}
	if s.spread {
		state.acceptedAt = time.Time{}
		state.next = acceptedAt
		s.accepted.Add(key, state)
	} else {
		s.accepted.Remove(key)
	}
}

func (s *subscriptionSampler) Allow(meta outputs.Meta, messageBytes int, now time.Time) bool {
	if s == nil {
		return true
	}
	subscription := meta["subscription-name"]
	rule, ok := s.rules[subscription]
	if !ok {
		return true
	}
	if messageBytes < rule.MinimumBytes {
		return true
	}
	source := meta["source"]
	if source == "" {
		return true
	}
	key := source + "\x00" + subscription

	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.accepted.Get(key)
	if s.spread {
		if !ok || state.acceptedAt.IsZero() {
			// The first wide message may be the only complete baseline before a
			// stream switches to narrow deltas, so it must not be discarded.
			state.next = nextSpreadTime(key, now.Add(rule.Interval), rule.Interval)
		} else if now.Before(state.next) {
			return false
		} else {
			state.next = state.next.Add((now.Sub(state.next)/rule.Interval + 1) * rule.Interval)
		}
	} else if ok && now.Sub(state.acceptedAt) < rule.Interval {
		return false
	}
	state.acceptedAt = now
	s.accepted.Add(key, state)
	return true
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
