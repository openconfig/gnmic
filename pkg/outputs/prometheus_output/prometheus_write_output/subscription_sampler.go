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

type sampleKey struct {
	source       string
	subscription string
	instance     string
}

type sampleState struct {
	acceptedAt time.Time
	next       time.Time
}

type sampleReservation struct {
	key   sampleKey
	state sampleState
}

type subscriptionSampler struct {
	mu       sync.Mutex
	rules    map[string]messageSamplingRule
	spread   bool
	accepted *lru.Cache[sampleKey, sampleState]
	pending  map[sampleKey]*sampleReservation
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
	accepted, err := lru.New[sampleKey, sampleState](cacheSize)
	if err != nil {
		return nil, fmt.Errorf("create message-sampling cache: %w", err)
	}
	return &subscriptionSampler{
		rules:    maps.Clone(cfg.BySubscription),
		spread:   cfg.Spread,
		accepted: accepted,
		pending:  make(map[sampleKey]*sampleReservation),
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

func (s *subscriptionSampler) Commit(reservation *sampleReservation) {
	if s == nil || reservation == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending[reservation.key] != reservation {
		return
	}
	delete(s.pending, reservation.key)
	s.accepted.Add(reservation.key, reservation.state)
}

func (s *subscriptionSampler) Rollback(reservation *sampleReservation) {
	if s == nil || reservation == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending[reservation.key] == reservation {
		delete(s.pending, reservation.key)
	}
}

func (s *subscriptionSampler) Reserve(
	info outputs.SubscriptionInfo,
	message proto.Message,
	now time.Time,
) (bool, *sampleReservation) {
	if s == nil || info.Source == "" || info.Name == "" || info.Instance == "" || info.Mode != gnmi.SubscriptionList_STREAM {
		return true, nil
	}
	rule, ok := s.rules[info.Name]
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
	if proto.Size(message) < rule.MinimumBytes {
		return true, nil
	}
	key := sampleKey{
		source:       info.Source,
		subscription: info.Name,
		instance:     info.Instance,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.pending[key]; ok {
		return false, nil
	}
	state, _ := s.accepted.Peek(key)
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
	reservation := &sampleReservation{
		key:   key,
		state: state,
	}
	s.pending[key] = reservation
	return true, reservation
}

func nextSpreadTime(key sampleKey, now time.Time, interval time.Duration) time.Time {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(key.source))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(key.subscription))
	phase := time.Duration(hash.Sum64() % uint64(interval))
	remainder := time.Duration(now.UnixNano() % int64(interval))
	if remainder < 0 {
		remainder += interval
	}
	delay := (phase - remainder + interval) % interval
	return now.Add(delay)
}
