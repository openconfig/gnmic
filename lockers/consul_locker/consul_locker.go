package consul_locker

import (
	"context"
	"errors"
	"log"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/karimra/gnmic/lockers"
)

const (
	defaultAddress    = "localhost:8500"
	defaultSessionTTL = 10 * time.Second
	defaultRetryTimer = 2 * time.Second
	loggingPrefix     = "consul_locker "
)

func init() {
	lockers.Register("consul", func() lockers.Locker {
		return &ConsulLocker{
			Cfg:    &config{},
			m:      new(sync.Mutex),
			locks:  make(map[string]*locks),
			logger: log.New(os.Stderr, loggingPrefix, log.LstdFlags|log.Lmicroseconds),
		}
	})
}

type ConsulLocker struct {
	Cfg    *config
	client *api.Client
	logger *log.Logger
	m      *sync.Mutex
	locks  map[string]*locks
}

type config struct {
	Address    string
	SessionTTL time.Duration
	RetryTimer time.Duration
}

type locks struct {
	sessionID string
	doneChan  chan struct{}
}

func (c *ConsulLocker) Init(ctx context.Context, cfg map[string]interface{}) error {
	err := lockers.DecodeConfig(cfg, c.Cfg)
	if err != nil {
		return err
	}
	err = c.setDefaults()
	if err != nil {
		return err
	}
	c.client, err = api.NewClient(&api.Config{
		Address: c.Cfg.Address,
		Scheme:  "http",
	})
	if err != nil {
		return err
	}
	c.logger.Printf("initialized consul locker with cfg=%+v", c.Cfg)
	return nil
}

func (c *ConsulLocker) Lock(ctx context.Context, key string) (bool, error) {
	var err error
	var acquired = false
	writeOpts := new(api.WriteOptions)
	writeOpts = writeOpts.WithContext(ctx)
	kvPair := &api.KVPair{Key: key}
	for {
		acquired = false
		kvPair.Session, _, err = c.client.Session().Create(
			&api.SessionEntry{
				Behavior: "delete",
				TTL:      c.Cfg.SessionTTL.String(), // is needed in order for other node to be able to acquire the leader key
			},
			writeOpts,
		)
		if err != nil {
			c.logger.Printf("failed creating session: %v", err)
			time.Sleep(c.Cfg.RetryTimer)
			continue
		}

		acquired, _, err = c.client.KV().Acquire(kvPair, writeOpts)
		if err != nil {
			c.logger.Printf("failed acquiring KV: %v", err)
			time.Sleep(c.Cfg.RetryTimer)
			continue
		}
		if acquired {
			doneChan := make(chan struct{})
			go c.client.Session().RenewPeriodic("5s", kvPair.Session, writeOpts, doneChan)

			c.m.Lock()
			c.locks[key] = &locks{sessionID: kvPair.Session, doneChan: doneChan}
			c.m.Unlock()
			return true, nil
		}
		c.logger.Printf("failed acquiring KV: already locked")
		time.Sleep(c.Cfg.RetryTimer)
	}
}

func (c *ConsulLocker) LockMany(ctx context.Context, keys []string, locked chan string) {}

func (c *ConsulLocker) Unlock(key string) error {
	c.m.Lock()
	defer c.m.Unlock()
	if lock, ok := c.locks[key]; ok {
		close(lock.doneChan)
		_, err := c.client.KV().Delete(key, nil)
		if err != nil {
			return err
		}
		_, err = c.client.Session().Destroy(lock.sessionID, nil)
		if err != nil {
			return err
		}
		return nil
	}
	return errors.New("unknown key")
}

func (c *ConsulLocker) Stop() error {
	c.m.Lock()
	defer c.m.Unlock()
	for k := range c.locks {
		c.Unlock(k)
	}
	return nil
}

// helpers

func (c *ConsulLocker) setDefaults() error {
	if c.Cfg.SessionTTL <= 0 {
		c.Cfg.SessionTTL = defaultSessionTTL
	}
	if c.Cfg.RetryTimer <= 0 {
		c.Cfg.RetryTimer = defaultRetryTimer
	}
	return nil
}
