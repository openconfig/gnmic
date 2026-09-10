// SPDX-License-Identifier: Apache-2.0

package k8s_locker

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"

	"github.com/openconfig/gnmic/pkg/lockers"
)

const maxConcurrentReleases = 16

var errLeaseRenewalFailed = errors.New("kubernetes lease renewal failed")

type leaseSession struct {
	ctx      context.Context
	cancel   context.CancelCauseFunc
	identity string
	acquired chan struct{}
	stopped  chan struct{}
}

func (k *k8sLocker) Lock(ctx context.Context, key string, value []byte) (bool, error) {
	ctx = klog.NewContext(ctx, logr.FromSlogHandler(k.logger.Handler()))
	ctx, cancel := context.WithCancelCause(ctx)
	session := &leaseSession{ctx: ctx, cancel: cancel, identity: uuid.NewString(), acquired: make(chan struct{}), stopped: make(chan struct{})}
	name := leaseName(key)
	elector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock: &resourcelock.LeaseLock{
			LeaseMeta:  metav1.ObjectMeta{Name: name, Namespace: k.Cfg.Namespace},
			Client:     annotatedLeaseGetter{LeasesGetter: k.clientset.CoordinationV1(), annotations: map[string]string{origKeyName: key, origValueName: string(value)}},
			Labels:     leaseLabels(key, value),
			LockConfig: resourcelock.ResourceLockConfig{Identity: session.identity},
		},
		LeaseDuration: k.Cfg.LeaseDuration,
		RenewDeadline: k.Cfg.RenewDeadline,
		RetryPeriod:   k.Cfg.RetryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(context.Context) { close(session.acquired) },
			OnStoppedLeading: func() {},
		},
		Name: name,
	})
	if err != nil {
		cancel(context.Canceled)
		return false, err
	}
	k.mu.Lock()
	if k.stopped {
		k.mu.Unlock()
		cancel(context.Canceled)
		return false, lockers.ErrCanceled
	}
	if previous, exists := k.locks[key]; exists && previous.ctx.Err() == nil {
		k.mu.Unlock()
		cancel(context.Canceled)
		return false, fmt.Errorf("lock %q is already active in this instance", key)
	}
	k.locks[key] = session
	k.mu.Unlock()
	go func() {
		defer close(session.stopped)
		elector.Run(ctx)
		if ctx.Err() == nil {
			cancel(fmt.Errorf("%w for %q: %w", errLeaseRenewalFailed, key, context.DeadlineExceeded))
		}
	}()
	select {
	case <-session.acquired:
		select {
		case <-session.stopped:
		default:
			if ctx.Err() == nil {
				return true, nil
			}
		}
	case <-session.stopped:
	case <-ctx.Done():
	}
	err = context.Cause(ctx)
	cancel(context.Canceled)
	k.forgetSession(key, session)
	if err != nil {
		return false, err
	}
	return false, fmt.Errorf("lost lease %q while acquiring it", key)
}

func (k *k8sLocker) KeepLock(ctx context.Context, key string) (chan struct{}, chan error) {
	done, errs := make(chan struct{}), make(chan error, 1)
	k.mu.Lock()
	session := k.locks[key]
	k.mu.Unlock()
	go func() {
		if session == nil {
			errs <- fmt.Errorf("lock %q is not active", key)
			return
		}
		select {
		case <-ctx.Done():
			session.cancel(context.Cause(ctx))
			close(done)
		case <-session.stopped:
			cause := context.Cause(session.ctx)
			if errors.Is(cause, errLeaseRenewalFailed) {
				errs <- cause
				return
			}
			close(done)
		}
	}()
	return done, errs
}

func (k *k8sLocker) forgetSession(key string, session *leaseSession) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.locks[key] == session {
		delete(k.locks, key)
	}
}

func (k *k8sLocker) release(ctx context.Context, key string, session *leaseSession) error {
	session.cancel(context.Canceled)
	stopCtx, cancel := context.WithTimeout(ctx, k.Cfg.RetryPeriod)
	select {
	case <-session.stopped:
		cancel()
	case <-stopCtx.Done():
		err := stopCtx.Err()
		cancel()
		return err
	}
	client := k.clientset.CoordinationV1().Leases(k.Cfg.Namespace)
	getCtx, cancel := context.WithTimeout(ctx, k.Cfg.RetryPeriod)
	lease, err := client.Get(getCtx, leaseName(key), metav1.GetOptions{})
	cancel()
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if lease.Spec.HolderIdentity == nil || *lease.Spec.HolderIdentity != session.identity {
		return nil
	}
	deleteCtx, cancel := context.WithTimeout(ctx, k.Cfg.RetryPeriod)
	defer cancel()
	err = client.Delete(deleteCtx, lease.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &lease.UID, ResourceVersion: &lease.ResourceVersion}})
	if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
		return nil
	}
	return err
}

func (k *k8sLocker) Unlock(ctx context.Context, key string) error {
	k.mu.Lock()
	session := k.locks[key]
	delete(k.locks, key)
	k.mu.Unlock()
	if session == nil {
		return nil
	}
	return k.release(ctx, key, session)
}

func (k *k8sLocker) Stop() error {
	k.mu.Lock()
	k.stopped = true
	sessions := k.locks
	k.locks = make(map[string]*leaseSession)
	for _, session := range sessions {
		session.cancel(context.Canceled)
	}
	k.mu.Unlock()
	if k.stopCache != nil {
		k.stopCache()
		<-k.cacheDone
	}
	var group errgroup.Group
	group.SetLimit(maxConcurrentReleases)
	errCh := make(chan error, len(sessions))
	for key, session := range sessions {
		group.Go(func() error {
			if err := k.release(context.Background(), key, session); err != nil {
				errCh <- fmt.Errorf("release lock %q: %w", key, err)
			}
			return nil
		})
	}
	_ = group.Wait()
	close(errCh)
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
