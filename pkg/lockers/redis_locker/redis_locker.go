package redis_locker

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	goredislib "github.com/redis/go-redis/v9"

	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/lockers"
)

const (
	defaultLeaseDuration = 10 * time.Second
	defaultRetryTimer    = 2 * time.Second
	defaultPollTimer     = 10 * time.Second
	loggingPrefix        = "[redis_locker] "
)

func init() {
	lockers.Register("redis", func() lockers.Locker {
		return &redisLocker{
			Cfg:             &config{},
			m:               new(sync.RWMutex),
			acquiredLocks:   make(map[string]*redsync.Mutex),
			attemptingLocks: make(map[string]*redsync.Mutex),
			registerLock:    make(map[string]context.CancelFunc),
			logger:          log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

type redisLocker struct {
	Cfg             *config
	logger          *log.Logger
	m               *sync.RWMutex
	acquiredLocks   map[string]*redsync.Mutex
	attemptingLocks map[string]*redsync.Mutex
	registerLock    map[string]context.CancelFunc

	client      goredislib.UniversalClient
	redisLocker *redsync.Redsync
}

type config struct {
	Servers       []string      `mapstructure:"servers,omitempty" json:"servers,omitempty"`
	MasterName    string        `mapstructure:"master-name,omitempty" json:"master-name,omitempty"`
	Password      string        `mapstructure:"password,omitempty" json:"password,omitempty"`
	LeaseDuration time.Duration `mapstructure:"lease-duration,omitempty" json:"lease-duration,omitempty"`
	RenewPeriod   time.Duration `mapstructure:"renew-period,omitempty" json:"renew-period,omitempty"`
	RetryTimer    time.Duration `mapstructure:"retry-timer,omitempty" json:"retry-timer,omitempty"`
	PollTimer     time.Duration `mapstructure:"poll-timer,omitempty" json:"poll-timer,omitempty"`
	Debug         bool          `mapstructure:"debug,omitempty" json:"debug,omitempty"`
}

func (k *redisLocker) Init(ctx context.Context, cfg map[string]interface{}, opts ...lockers.Option) error {
	err := lockers.DecodeConfig(cfg, k.Cfg)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(k)
	}
	err = k.setDefaults()
	if err != nil {
		return err
	}
	k.client = goredislib.NewUniversalClient(&goredislib.UniversalOptions{
		Addrs:      k.Cfg.Servers,
		MasterName: k.Cfg.MasterName,
		Password:   k.Cfg.Password,
	})
	if err := k.client.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("cannot contact redis server: %w", err)
	}
	k.redisLocker = redsync.New(goredis.NewPool(k.client))
	return nil
}

func (k *redisLocker) Lock(ctx context.Context, key string, val []byte) (bool, error) {
	if k.Cfg.Debug {
		k.logger.Printf("attempting to lock=%s", key)
	}
	mu := k.redisLocker.NewMutex(
		key,
		redsync.WithGenValueFunc(func() (string, error) {
			rand, err := k.genRandValue()
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s-%s", val, rand), nil
		}),
		redsync.WithExpiry(k.Cfg.LeaseDuration),
	)
	k.m.Lock()
	k.attemptingLocks[key] = mu
	k.m.Unlock()
	defer func() {
		k.m.Lock()
		defer k.m.Unlock()
		delete(k.attemptingLocks, key)
	}()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
			err := mu.LockContext(ctx)
			if err != nil {
				switch err.(type) {
				case *redsync.ErrTaken:
					if k.Cfg.Debug {
						k.logger.Printf("lock already taken lock=%s: %v", key, err)
					}
					return false, nil
				default:
					return false, fmt.Errorf("failed to acquire lock=%s: %w", key, err)
				}
			}

			k.m.Lock()
			k.acquiredLocks[key] = mu
			k.m.Unlock()
			return true, nil
		}
	}
}

func (k *redisLocker) KeepLock(ctx context.Context, key string) (chan struct{}, chan error) {
	doneChan := make(chan struct{})
	errChan := make(chan error)

	go func() {
		defer close(doneChan)
		ticker := time.NewTicker(k.Cfg.RenewPeriod)
		k.m.RLock()
		lock, ok := k.acquiredLocks[key]
		k.m.RUnlock()
		for {
			select {
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			case <-doneChan:
				return
			case <-ticker.C:
				if !ok {
					errChan <- fmt.Errorf("unable to maintain lock %q: not found in acquiredlocks", key)
					return
				}
				ok, err := lock.ExtendContext(ctx)
				if err != nil {
					errChan <- err
					return
				}
				if !ok {
					errChan <- fmt.Errorf("could not keep lock")
					return
				}

			}
		}
	}()
	return doneChan, errChan
}

func (k *redisLocker) Unlock(ctx context.Context, key string) error {
	k.m.Lock()
	defer k.m.Unlock()
	if lock, ok := k.acquiredLocks[key]; ok {
		delete(k.acquiredLocks, key)
		ok, err := lock.Unlock()
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("failed to unlock lock %s", key)
		}
	}
	if lock, ok := k.attemptingLocks[key]; ok {
		delete(k.attemptingLocks, key)
		_, err := lock.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}

func (k *redisLocker) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	keys := []string{}
	k.m.RLock()
	for key := range k.acquiredLocks {
		keys = append(keys, key)
	}
	k.m.RUnlock()
	for _, key := range keys {
		k.Unlock(ctx, key)
	}
	return k.Deregister("")
}

func (k *redisLocker) SetLogger(logger *log.Logger) {
	if logger != nil && k.logger != nil {
		k.logger.SetOutput(logger.Writer())
		k.logger.SetFlags(logger.Flags())
	}
}

// helpers

func (k *redisLocker) setDefaults() error {
	if k.Cfg.LeaseDuration <= 0 {
		k.Cfg.LeaseDuration = defaultLeaseDuration
	}
	if k.Cfg.RenewPeriod <= 0 || k.Cfg.RenewPeriod >= k.Cfg.LeaseDuration {
		k.Cfg.RenewPeriod = k.Cfg.LeaseDuration / 2
	}
	if k.Cfg.RetryTimer <= 0 {
		k.Cfg.RetryTimer = defaultRetryTimer
	}
	if k.Cfg.PollTimer <= 0 {
		k.Cfg.PollTimer = defaultPollTimer
	}
	return nil
}

func (k *redisLocker) String() string {
	b, err := json.Marshal(k.Cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

// genRandValue is required to generate a random value
// so that the redislock algorithm works properly
// especially in multi-server setups.
func (k *redisLocker) genRandValue() (string, error) {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
