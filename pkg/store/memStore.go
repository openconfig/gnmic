// © 2025 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package store

import (
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type memStore[T any] struct {
	mu sync.RWMutex
	// kind -> (key -> obj)
	kinds map[string]map[string]T
	// kind -> validation function
	validationFns map[string]ValidationFunc[T]
	// kind -> (watcherID -> chan)
	watchers map[string]map[string]chan *Event[T]
	// compare func
	compareFn CompareFunc[T]
	closed    bool
}

func NewMemStore[T any](opt StoreOptions[T]) Store[T] {
	ms := &memStore[T]{
		kinds:         make(map[string]map[string]T),
		watchers:      make(map[string]map[string]chan *Event[T]),
		validationFns: make(map[string]ValidationFunc[T]),
		compareFn:     opt.compareFn,
	}
	if ms.compareFn == nil {
		ms.compareFn = DefaultCompareFunc[T]
	}
	if opt.validateFns != nil {
		maps.Copy(ms.validationFns, opt.validateFns)
	}
	return ms
}

func (s *memStore[T]) ensureKind(kind string) {
	if _, ok := s.kinds[kind]; !ok {
		s.kinds[kind] = make(map[string]T)
	}
	if _, ok := s.watchers[kind]; !ok {
		s.watchers[kind] = make(map[string]chan *Event[T])
	}
}

func cloneMap[T any](in map[string]T) map[string]T {
	if in == nil {
		return map[string]T{}
	}
	out := make(map[string]T, len(in))
	maps.Copy(out, in) // note: this is a shallow copy
	return out
}

func (s *memStore[T]) Get(kind, key string) (T, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		var zero T
		return zero, false, errors.New("store closed")
	}
	m := s.kinds[kind]
	v, ok := m[key]
	return v, ok, nil
}

func (s *memStore[T]) List(kind string, filters ...FilterFunc[T]) (map[string]T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errors.New("store closed")
	}
	rs := make(map[string]T, len(s.kinds[kind]))
	for k, v := range s.kinds[kind] {
		for _, f := range filters {
			if f != nil && !f(k, v) {
				continue
			}
		}
		rs[k] = v
	}
	return rs, nil
}

func (s *memStore[T]) Keys(kind string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errors.New("store closed")
	}
	keys := make([]string, 0, len(s.kinds[kind]))
	for k := range s.kinds[kind] {
		keys = append(keys, k)
	}
	return keys, nil
}

func (s *memStore[T]) Values(kind string) ([]KeyValue[T], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return nil, errors.New("store closed")
	}
	values := make([]KeyValue[T], 0, len(s.kinds[kind]))
	for k, v := range s.kinds[kind] {
		values = append(values, KeyValue[T]{Key: k, Value: v})
	}
	return values, nil
}

func (s *memStore[T]) Count(kind string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return 0, errors.New("store closed")
	}
	return len(s.kinds[kind]), nil
}

func (s *memStore[T]) Set(kind, key string, value T) (bool, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return false, errors.New("store closed")
	}
	s.ensureKind(kind)

	if fn, ok := s.validationFns[kind]; ok {
		if err := fn(value); err != nil {
			s.mu.Unlock()
			return false, err
		}
	}

	prev, existed := s.kinds[kind][key]
	s.kinds[kind][key] = value

	if s.compareFn(prev, value) {
		s.mu.Unlock()
		return false, nil
	}

	// copy watchers then unlock
	wchs := make([]chan *Event[T], 0, len(s.watchers[kind]))
	for _, ch := range s.watchers[kind] {
		wchs = append(wchs, ch)
	}
	s.mu.Unlock()

	evType := EventTypeUpdate
	if !existed {
		evType = EventTypeCreate
	}
	ev := &Event[T]{Kind: kind, Name: key, EventType: evType, Object: value}
	for _, ch := range wchs {
		select {
		case ch <- ev:
		default:
		}
	}
	return !existed, nil
}

func (s *memStore[T]) SetAll(kind string, values map[string]T) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("store closed")
	}
	s.ensureKind(kind)
	maps.Copy(s.kinds[kind], values)

	// copy watchers then unlock
	wchs := make([]chan *Event[T], 0, len(s.watchers[kind]))
	for _, ch := range s.watchers[kind] {
		wchs = append(wchs, ch)
	}
	s.mu.Unlock()

	for _, ch := range wchs {
		for k, v := range values {
			select {
			case ch <- &Event[T]{Kind: kind, Name: k, EventType: EventTypeCreate, Object: v}:
			default: // no blocking
			}
		}
	}
	return nil
}

func (s *memStore[T]) Delete(kind, key string) (bool, T, error) {
	var zero T

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return false, zero, errors.New("store closed")
	}
	s.ensureKind(kind)

	prev, existed := s.kinds[kind][key]
	if existed {
		delete(s.kinds[kind], key)
	}

	if !existed {
		s.mu.Unlock()
		return false, zero, nil
	}

	// copy watchers then unlock
	wchs := make([]chan *Event[T], 0, len(s.watchers[kind]))
	for _, ch := range s.watchers[kind] {
		wchs = append(wchs, ch)
	}
	s.mu.Unlock()

	ev := &Event[T]{Kind: kind, Name: key, EventType: EventTypeDelete, Object: prev}
	for _, ch := range wchs {
		select {
		case ch <- ev:
		default:
		}
	}
	return existed, prev, nil
}

func (s *memStore[T]) SetFn(kind, key string, fn func(v T) (T, error)) (bool, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return false, errors.New("store closed")
	}
	s.ensureKind(kind)

	prev, existed := s.kinds[kind][key]
	value, err := fn(prev)
	if err != nil {
		s.mu.Unlock()
		return false, err
	}
	// update value
	s.kinds[kind][key] = value
	// copy watchers then unlock
	wchs := make([]chan *Event[T], 0, len(s.watchers[kind]))
	for _, ch := range s.watchers[kind] {
		wchs = append(wchs, ch)
	}
	s.mu.Unlock()

	evType := EventTypeUpdate
	if !existed {
		evType = EventTypeCreate
	}
	ev := &Event[T]{
		Kind:      kind,
		Name:      key,
		EventType: evType,
		Object:    value,
	}
	for _, ch := range wchs {
		select {
		case ch <- ev:
		default: // no blocking
		}
	}
	return !existed, nil
}

func (s *memStore[T]) Watch(kind string, opts ...WatchOption[T]) (<-chan *Event[T], func(), error) {
	if kind == "" {
		return nil, nil, errors.New("kind required")
	}
	cfg := &watchCfg[T]{}
	for _, o := range opts {
		o(cfg)
	}

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, nil, errors.New("store closed")
	}
	s.ensureKind(kind)

	id := uuid.NewString()
	ch := make(chan *Event[T], 128) // buffered
	s.watchers[kind][id] = ch

	// capture snapshot for optional initial replay
	var snap map[string]T
	if cfg.initial {
		snap = cloneMap(s.kinds[kind])
	}
	s.mu.Unlock()

	// used to cancel the initial snapshot goroutine
	doneCh := make(chan struct{})
	// send initial snapshot
	if cfg.initial && len(snap) > 0 {
		go func(m map[string]T) {
			for k, v := range m {
				ev := &Event[T]{
					Kind:      kind,
					Name:      k,
					EventType: EventTypeCreate,
					Object:    v,
				}
				select {
				case ch <- ev:
				case <-doneCh:
					return
				}
			}
		}(snap)
	}

	// build cancel function
	cancel := func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if w, ok := s.watchers[kind]; ok {
			if c, ok := w[id]; ok {
				delete(w, id)
				close(doneCh)
				close(c)
			}
		}
	}
	return ch, cancel, nil
}

func (s *memStore[T]) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	for _, m := range s.watchers {
		for id, ch := range m {
			delete(m, id)
			close(ch)
		}
	}
	return nil
}

func (s *memStore[T]) Dump() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sb := strings.Builder{}
	for kind, m := range s.kinds {
		sb.WriteString(fmt.Sprintf("%s:\n", kind))
		for k, v := range m {
			sb.WriteString(fmt.Sprintf("  %s: %+v\n", k, v))
		}
	}
	return sb.String()
}
