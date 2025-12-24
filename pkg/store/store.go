// © 2025 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package store

import "reflect"

type Store[T any] interface {
	// Read
	Get(kind, key string) (val T, ok bool, err error)
	List(kind string, filter ...FilterFunc[T]) (map[string]T, error)
	Count(kind string) (int, error)
	Keys(kind string) ([]string, error)
	Values(kind string) ([]KeyValue[T], error)
	// Write
	Set(kind, key string, value T) (created bool, err error)
	SetFn(kind, key string, fn func(v T) (T, error)) (changed bool, err error)
	SetAll(kind string, values map[string]T) error
	Delete(kind, key string) (existed bool, prev T, err error)
	// Watch
	Watch(kind string, opts ...WatchOption[T]) (r <-chan *Event[T], cancel func(), err error)
	Close() error
	// to remove
	Dump() string
	GetAll() (map[string]map[string]T, error)
}

type KeyValue[T any] struct {
	Key   string
	Value T
}

type FilterFunc[T any] func(key string, val T) bool

type Event[T any] struct {
	Kind      string
	Name      string
	EventType EventType
	Object    T // for delete: previous value
}

type EventType string

const (
	EventTypeCreate EventType = "create"
	EventTypeUpdate EventType = "update"
	EventTypeDelete EventType = "delete"
)

// Watch options

type WatchOption[T any] func(*watchCfg[T])

type watchCfg[T any] struct {
	initial bool // send current keys as create events immediately
}

func WithInitialReplay[T any]() WatchOption[T] {
	return func(w *watchCfg[T]) {
		w.initial = true
	}
}

type StoreOptions[T any] struct {
	compareFn   CompareFunc[T]
	validateFns map[string]ValidationFunc[T]
}

type ValidationFunc[T any] func(v T) error

type CompareFunc[T any] func(prev, new T) bool

func DefaultCompareFunc[T any](prev, new T) bool {
	return reflect.DeepEqual(prev, new)
}
