package testutil

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/openconfig/gnmic/pkg/lockers"
)

// FakeLocker is an in-memory lockers.Locker for manager integration tests.
type FakeLocker struct {
	mu    sync.Mutex
	locks map[string]string // key -> holder id
	log   *slog.Logger
}

func NewFakeLocker() *FakeLocker {
	return &FakeLocker{
		locks: make(map[string]string),
		log:   slog.Default(),
	}
}

func (f *FakeLocker) Init(context.Context, map[string]any, ...lockers.Option) error {
	return nil
}

func (f *FakeLocker) Stop() error { return nil }

func (f *FakeLocker) SetLogger(l *slog.Logger) {
	if l != nil {
		f.log = l
	}
}

func (f *FakeLocker) Lock(_ context.Context, key string, val []byte) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.locks[key]; ok {
		return false, nil
	}
	f.locks[key] = string(val)
	return true, nil
}

func (f *FakeLocker) KeepLock(ctx context.Context, key string) (chan struct{}, chan error) {
	done := make(chan struct{})
	errCh := make(chan error, 1)
	go func() {
		<-ctx.Done()
		close(done)
	}()
	return done, errCh
}

func (f *FakeLocker) IsLocked(_ context.Context, key string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.locks[key]
	return ok, nil
}

func (f *FakeLocker) Unlock(_ context.Context, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.locks, key)
	return nil
}

func (f *FakeLocker) Register(context.Context, *lockers.ServiceRegistration) error { return nil }

func (f *FakeLocker) Deregister(string) error { return nil }

func (f *FakeLocker) GetServices(context.Context, string, []string) ([]*lockers.Service, error) {
	return nil, nil
}

func (f *FakeLocker) WatchServices(ctx context.Context, _ string, _ []string, ch chan<- []*lockers.Service, _ time.Duration) error {
	<-ctx.Done()
	return ctx.Err()
}

func (f *FakeLocker) List(_ context.Context, prefix string) (map[string]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make(map[string]string)
	for k, v := range f.locks {
		if strings.HasPrefix(k, prefix) {
			out[k] = v
		}
	}
	return out, nil
}

// SetLock seeds a lock as if held by holder (member id).
func (f *FakeLocker) SetLock(key, holder string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.locks[key] = holder
}
