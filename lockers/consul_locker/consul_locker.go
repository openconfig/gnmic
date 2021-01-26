package consul_locker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
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
			logger: log.New(ioutil.Discard, loggingPrefix, log.LstdFlags|log.Lmicroseconds),
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
	Address     string        `mapstructure:"address,omitempty" json:"address,omitempty"`
	SessionTTL  time.Duration `mapstructure:"session-ttl,omitempty" json:"session-ttl,omitempty"`
	RetryTimer  time.Duration `mapstructure:"retry-timer,omitempty" json:"retry-timer,omitempty"`
	RenewPeriod time.Duration `mapstructure:"renew-period,omitempty" json:"renew-period,omitempty"`
	Debug       bool          `mapstructure:"debug,omitempty" json:"debug,omitempty"`
}

type locks struct {
	sessionID string
	doneChan  chan struct{}
}

func (c *ConsulLocker) Init(ctx context.Context, cfg map[string]interface{}, opts ...lockers.Option) error {
	err := lockers.DecodeConfig(cfg, c.Cfg)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(c)
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
	c.logger.Printf("initialized consul locker with cfg=%s", c)
	return nil
}

func (c *ConsulLocker) Lock(ctx context.Context, key string, val []byte) (bool, error) {
	var err error
	var acquired = false
	writeOpts := new(api.WriteOptions)
	writeOpts = writeOpts.WithContext(ctx)
	kvPair := &api.KVPair{Key: key, Value: val}
	for {
		acquired = false
		kvPair.Session, _, err = c.client.Session().Create(
			&api.SessionEntry{
				Behavior: "delete",
				TTL:      c.Cfg.SessionTTL.String(),
			},
			writeOpts,
		)
		if err != nil {
			c.logger.Printf("failed creating session: %v", err)
			time.Sleep(c.Cfg.RetryTimer)
			continue
		}
		//var wm *api.WriteMeta
		acquired, _, err = c.client.KV().Acquire(kvPair, writeOpts)
		if err != nil {
			c.logger.Printf("failed acquiring lock to %q: %v", kvPair.Key, err)
			time.Sleep(c.Cfg.RetryTimer)
			continue
		}

		if acquired {
			doneChan := make(chan struct{})
			//go c.client.Session().RenewPeriodic("5s", kvPair.Session, writeOpts, doneChan)

			c.m.Lock()
			c.locks[key] = &locks{sessionID: kvPair.Session, doneChan: doneChan}
			c.m.Unlock()
			return true, nil
		}
		if c.Cfg.Debug {
			c.logger.Printf("failed acquiring lock to %q: already locked", kvPair.Key)
		}
		time.Sleep(c.Cfg.RetryTimer)
	}
}

func (c *ConsulLocker) KeepLock(ctx context.Context, key string) (chan struct{}, chan error) {
	writeOpts := new(api.WriteOptions)
	writeOpts = writeOpts.WithContext(ctx)

	c.m.Lock()
	sessionID := ""
	doneChan := make(chan struct{})
	if l, ok := c.locks[key]; ok {
		sessionID = l.sessionID
		doneChan = l.doneChan
	}
	c.m.Unlock()
	errChan := make(chan error)
	go func() {
		if sessionID == "" {
			errChan <- fmt.Errorf("unknown key")
			close(doneChan)
			return
		}
		err := c.client.Session().RenewPeriodic(c.Cfg.RenewPeriod.String(), sessionID, writeOpts, doneChan)
		if err != nil {
			errChan <- err
		}
	}()

	return doneChan, errChan
}

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
		delete(c.locks, key)
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

func (c *ConsulLocker) SetLogger(logger *log.Logger) {
	if logger != nil && c.logger != nil {
		c.logger.SetOutput(logger.Writer())
		c.logger.SetFlags(logger.Flags())
	}
}

// helpers

func (c *ConsulLocker) setDefaults() error {
	if c.Cfg.SessionTTL <= 0 {
		c.Cfg.SessionTTL = defaultSessionTTL
	}
	if c.Cfg.RetryTimer <= 0 {
		c.Cfg.RetryTimer = defaultRetryTimer
	}
	if c.Cfg.RenewPeriod <= 0 || c.Cfg.RenewPeriod >= c.Cfg.SessionTTL {
		c.Cfg.RenewPeriod = c.Cfg.SessionTTL / 2
	}
	return nil
}

func (c *ConsulLocker) String() string {
	b, err := json.Marshal(c.Cfg)
	if err != nil {
		return ""
	}
	return string(b)
}
