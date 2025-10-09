package store

import "reflect"

type Store[T any] interface {
	// Read
	Get(kind, key string) (val T, ok bool, err error)
	List(kind string, filter ...FilterFunc[T]) (map[string]T, error)

	// Write
	Set(kind, key string, value T) (created bool, err error)
	LoadAndSet(kind, key string, fn func(v T) (T, error)) (changed bool, err error)
	SetAll(kind string, values map[string]T) error
	Delete(kind, key string) (existed bool, prev T, err error)

	// Watch
	Watch(kind string, opts ...WatchOption[T]) (r <-chan *Event[T], cancel func(), err error)

	Close() error
	// to remove
	Dump() string
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
	compare CompareFunc[T]
}

// compare func
type CompareFunc[T any] func(prev, new T) bool

func DefaultCompareFunc[T any](prev, new T) bool {
	return reflect.DeepEqual(prev, new)
}
