package app

import (
	"errors"
	"testing"

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
	lockers.Locker
	app                   *App
	stopErr               error
	stopCalls             int
	contextCanceledAtStop bool
}

func (l *shutdownTestLocker) Stop() error {
	l.stopCalls++
	l.contextCanceledAtStop = l.app.Context().Err() != nil
	return l.stopErr
}
