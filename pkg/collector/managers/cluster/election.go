package cluster_manager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/lockers"
)

const (
	// campaignPeriod        = 1 * time.Second
	recampaignBackoff     = 200 * time.Millisecond
	recampaignJitterRatio = 0.2
)

type Election interface {
	// Blocks until this node becomes leader (i.e., acquires the leader lock) or ctx is done.
	// Returns a monotonically increasing term for observability/metrics.
	Campaign(ctx context.Context) (term int64, err error)
	// Closes when leadership is lost (or returns nil if you don't need it).
	Observe(ctx context.Context) <-chan struct{} // closes/receives when leadership is lost (optional: return nil if N/A)
	// Withdraw withdraws from the leader position
	Withdraw() error
}

type election struct {
	nodeID      string
	clusterName string
	RenewEvery  time.Duration // renew every (e.g., 1/2 of TTL)
	locker      lockers.Locker
	logger      *slog.Logger
	//
	// internals
	term            atomic.Int64
	held            atomic.Bool
	loseOnce        sync.Once
	loseCh          chan struct{}
	cancelKeepAlive context.CancelFunc

	// backend-specific release fn for the held lock
	releaseFn func() error
	mu        sync.Mutex
}

func NewElection(locker lockers.Locker, clustering *config.Clustering, logger *slog.Logger) (Election, error) {
	var renewEvery time.Duration
	sTTL, ok := clustering.Locker["session-ttl"]
	if ok {
		switch st := sTTL.(type) {
		case string:
			var err error
			renewEvery, err = time.ParseDuration(st)
			if err != nil {
				return nil, err
			}
			if renewEvery <= 0 {
				return nil, errors.New("session-ttl must be greater than 0")
			}
			renewEvery = renewEvery / 2
		default:
			return nil, errors.New("session-ttl must be a string")
		}
	} else {
		renewEvery = 5 * time.Second
	}

	return &election{
		locker:      locker,
		nodeID:      clustering.InstanceName,
		clusterName: clustering.ClusterName,
		RenewEvery:  renewEvery,
		logger:      logger,
	}, nil
}

func (e *election) Campaign(ctx context.Context) (term int64, err error) {
	e.logger.Info("campaigning for leader", "node", e.nodeID, "cluster", e.clusterName)
	// reinitialize loseCh for this term
	e.mu.Lock()
	e.loseOnce = sync.Once{}
	e.loseCh = make(chan struct{})
	e.mu.Unlock()
	key := e.leaderKey()
	// try lock
	// keep trying until ctx canceled
	ticker := time.NewTimer(0) // fire immediately first time
	defer ticker.Stop()

	for {
		if !ticker.Stop() {
			select {
			case <-ticker.C:
			default:
			}
		}
		e.logger.Info("trying to acquire leader lock", "node", e.nodeID, "cluster", e.clusterName, "term", term)
		// Try to acquire the leader lock
		ok, release, err := tryAcquire(ctx, e.locker, key, []byte(e.nodeID))
		if err != nil {
			e.logger.Error("failed to acquire leader lock", "node", e.nodeID, "cluster", e.clusterName, "term", term, "error", err)
			// locker error... backoff a bit
			delay := jittered(recampaignBackoff)
			ticker.Reset(delay)
			select {
			case <-ctx.Done():
				return 0, ctx.Err()
			case <-ticker.C:
				continue
			}
		}
		if ok {
			e.logger.Info("acquired leader lock", "node", e.nodeID, "cluster", e.clusterName, "term", term)
			// I'm the captain now!
			e.mu.Lock()
			e.releaseFn = release
			e.mu.Unlock()

			e.held.Store(true)
			term := e.term.Add(1)

			// start keepalive loop bound to this leadership session
			keepCtx, cancel := context.WithCancel(ctx)
			e.cancelKeepAlive = cancel
			go e.keepalive(keepCtx, key)

			return term, nil
		}
		e.logger.Info("not acquired leader lock", "node", e.nodeID, "cluster", e.clusterName, "term", term)
		// lock not acquired, add a jitter and retry
		delay := jittered(recampaignBackoff)
		ticker.Reset(delay)
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-ticker.C:
			continue
		}
	}
}

// Observe closes when this node loses leadership.
// (Safe to call multiple times, same channel is returned.)
func (e *election) Observe(ctx context.Context) <-chan struct{} {
	e.mu.Lock()
	ch := e.loseCh
	e.mu.Unlock()
	return ch
}

func (e *election) Withdraw() error {
	if !e.held.Load() {
		return nil
	}

	e.mu.Lock()
	release := e.releaseFn
	e.releaseFn = nil
	cancel := e.cancelKeepAlive
	e.cancelKeepAlive = nil
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if release != nil {
		_ = release() // ignore error
	}

	// signal loss
	e.loseOnce.Do(func() {
		e.held.Store(false)
		if e.loseCh != nil {
			close(e.loseCh)
		}
	})

	e.logger.Info("leadership withdrawn", "term", e.term.Load(), "node", e.nodeID, "cluster", e.clusterName)

	return nil
}

func (e *election) leaderKey() string {
	return fmt.Sprintf("gnmic/%s/leader", e.clusterName)
}

// keepalive periodically renews the lock and detects loss.
// On failure (or if the holder changes), it signals loss and cleans up.
func (e *election) keepalive(ctx context.Context, key string) {
	t := time.NewTicker(e.RenewEvery)
	defer t.Stop()
	e.logger.Info("starting keepalive loop", "node", e.nodeID, "cluster", e.clusterName, "term", e.term.Load())
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.logger.Info("renewing leader lock", "node", e.nodeID, "cluster", e.clusterName, "term", e.term.Load())
			// Renew our lease,if that fails or another node took over, we lost leadership.
			if err := renew(ctx, e.locker, key, []byte(e.nodeID)); err != nil {
				e.signalLoss()
				return
			}
			e.logger.Info("renewed leader lock", "node", e.nodeID, "cluster", e.clusterName, "term", e.term.Load())
			if h, ok := holder(ctx, e.locker, key); ok && h != e.nodeID {
				// someone else is now the holder → we lost
				e.signalLoss()
				return
			}
		}
	}
}

func (e *election) signalLoss() {
	// release our lock (best effort)
	e.mu.Lock()
	release := e.releaseFn
	e.releaseFn = nil
	e.mu.Unlock()
	if release != nil {
		_ = release() // ignore error, we already lost
	}

	// stop renew loop
	if e.cancelKeepAlive != nil {
		e.cancelKeepAlive()
	}

	// signal once
	e.loseOnce.Do(func() {
		e.held.Store(false)
		close(e.loseCh)
	})
	e.logger.Warn("lost leadership", "term", e.term.Load(), "node", e.nodeID, "cluster", e.clusterName)
}

// tryAcquire tries to acquire key with value=holder and TTL.
// Returns (true, releaseFn, nil) if acquired, (false, nil, nil) if not acquired, or (false, nil, err) on backend error.
func tryAcquire(ctx context.Context, lk lockers.Locker, key string, holder []byte) (bool, func() error, error) {
	// Lock() attempts to acquire the lock, it returns (true,nil) if successful,
	// (false,nil) if already locked, or (false,err) if backend error.
	ok, err := lk.Lock(ctx, key, holder)
	if err != nil {
		return false, nil, err
	}
	if !ok {
		// someone else already holds the lock
		return false, nil, nil
	}

	// Start a keepalive session for this lock.
	kaCtx, cancel := context.WithCancel(context.Background())
	doneCh, errCh := lk.KeepLock(kaCtx, key)

	// Release function closes keepalive and unlocks.
	release := func() error {
		cancel()
		// drain both channels to avoid goroutine leaks
		select {
		case <-doneCh:
		default:
		}
		select {
		case <-errCh:
		default:
		}
		return lk.Unlock(context.Background(), key)
	}

	// Background watcher: if KeepLock fails (err or done), cancel leadership early.
	go func() {
		select {
		case <-doneCh:
			// Lock lost gracefully (KeepLock closed)
			cancel()
		case err := <-errCh:
			// Renewal failed or backend issue
			_ = err
			cancel()
		case <-kaCtx.Done():
		}
	}()

	return true, release, nil
}

// renew refreshes the TTL for a lock we hold.
func renew(ctx context.Context, lk lockers.Locker, key string, holder []byte) error {
	// In this Locker API, TTL renewals are managed by KeepLock().
	// So "renew" doesn’t need to explicitly refresh, just check if lock is still held.
	held, err := lk.IsLocked(ctx, key)
	if err != nil {
		return err
	}
	if !held {
		return fmt.Errorf("lock %q lost", key)
	}
	return nil
}

// holder returns current holder id (stringified from value) if locked.
func holder(ctx context.Context, lk lockers.Locker, key string) (string, bool) {
	m, err := lk.List(ctx, key)
	if err != nil {
		return "", false
	}
	// The Locker.List returns map[string]string{ lockName -> holderID }
	if len(m) == 0 {
		return "", false
	}
	if v, ok := m[key]; ok {
		return v, true
	}
	return "", false
}
