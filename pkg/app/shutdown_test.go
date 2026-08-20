package app

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/openconfig/gnmic/pkg/lockers"
)

func TestShutdownCancelsContextAndStopsLockerOnce(t *testing.T) {
	a := New()
	wantErr := errors.New("stop failed")
	locker := &shutdownTestLocker{app: a, stopErr: wantErr}
	a.locker = locker

	if err := a.Shutdown(); !errors.Is(err, wantErr) {
		t.Fatalf("Shutdown() error = %v, want %v", err, wantErr)
	}
	if a.Context().Err() == nil {
		t.Fatal("application context was not canceled")
	}
	if !locker.contextCanceledAtStop {
		t.Fatal("locker stopped before the application context was canceled")
	}
	if locker.stopCalls != 1 {
		t.Fatalf("locker Stop() calls = %d, want 1", locker.stopCalls)
	}

	if err := a.Shutdown(); !errors.Is(err, wantErr) {
		t.Fatalf("second Shutdown() error = %v, want %v", err, wantErr)
	}
	if locker.stopCalls != 1 {
		t.Fatalf("locker Stop() calls after second shutdown = %d, want 1", locker.stopCalls)
	}
}

type shutdownTestLocker struct {
	app                   *App
	stopErr               error
	stopCalls             int
	contextCanceledAtStop bool
}

func (*shutdownTestLocker) Init(context.Context, map[string]any, ...lockers.Option) error {
	return nil
}

func (l *shutdownTestLocker) Stop() error {
	l.stopCalls++
	l.contextCanceledAtStop = l.app.Context().Err() != nil
	return l.stopErr
}

func (*shutdownTestLocker) SetLogger(*slog.Logger) {}

func (*shutdownTestLocker) Lock(context.Context, string, []byte) (bool, error) {
	return false, nil
}

func (*shutdownTestLocker) KeepLock(context.Context, string) (chan struct{}, chan error) {
	return make(chan struct{}), make(chan error)
}

func (*shutdownTestLocker) IsLocked(context.Context, string) (bool, error) {
	return false, nil
}

func (*shutdownTestLocker) Unlock(context.Context, string) error { return nil }

func (*shutdownTestLocker) Register(context.Context, *lockers.ServiceRegistration) error {
	return nil
}

func (*shutdownTestLocker) Deregister(string) error { return nil }

func (*shutdownTestLocker) GetServices(context.Context, string, []string) ([]*lockers.Service, error) {
	return nil, nil
}

func (*shutdownTestLocker) WatchServices(
	context.Context,
	string,
	[]string,
	chan<- []*lockers.Service,
	time.Duration,
) error {
	return nil
}

func (*shutdownTestLocker) List(context.Context, string) (map[string]string, error) {
	return nil, nil
}
