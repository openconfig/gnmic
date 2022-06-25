package cache

import (
	"io"
	"log"
	"sync"
	"time"

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/karimra/gnmic/utils"
	gnmiCache "github.com/openconfig/gnmi/cache"
	"github.com/openconfig/gnmi/ctree"
	"github.com/openconfig/gnmi/proto/gnmi"
)

const (
	loggingPrefix  = "[cache] "
	defaultTimeout = 10 * time.Second
)

type GnmiCacheConfig struct {
	Expiration time.Duration `mapstructure:"expiration,omitempty"`
	Timeout    time.Duration `mapstructure:"timeout,omitempty"`
	Debug      bool          `mapstructure:"debug,omitempty"`
}

func (gcc *GnmiCacheConfig) setDefaults() {
	if gcc.Timeout == 0 {
		gcc.Timeout = defaultTimeout
	}
}

type GnmiCache struct {
	sync.Mutex
	caches     map[string]*gnmiCache.Cache
	logger     *log.Logger
	expiration time.Duration
	timeout    time.Duration
	debug      bool
}

func (gc *GnmiCache) loadConfig(gcc *GnmiCacheConfig) {
	gc.expiration = gcc.Expiration
	gc.timeout = gcc.Timeout
	gc.logger = log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags)
	gc.caches = make(map[string]*gnmiCache.Cache)
	gc.debug = gcc.Debug
}

func New(cfg *GnmiCacheConfig, opts ...Option) *GnmiCache {
	gc := new(GnmiCache)
	cfg.setDefaults()
	gc.loadConfig(cfg)
	for _, opt := range opts {
		opt(gc)
	}
	return gc
}

func (gc *GnmiCache) SetLogger(logger *log.Logger) {
	if logger != nil && gc.logger != nil {
		gc.logger.SetOutput(logger.Writer())
		gc.logger.SetFlags(logger.Flags())
	}
}

func (gc *GnmiCache) Write(measName string, rsp *gnmi.SubscribeResponse) {
	var err error
	switch rsp := rsp.GetResponse().(type) {
	case *gnmi.SubscribeResponse_Update:
		target := rsp.Update.GetPrefix().GetTarget()
		if target == "" {
			gc.logger.Printf("response missing target")
			return
		}
		gc.Lock()
		defer gc.Unlock()
		if _, ok := gc.caches[measName]; !ok {
			gc.caches[measName] = gnmiCache.New(nil)
			gc.caches[measName].Add(target)
		} else if !gc.caches[measName].HasTarget(target) {
			gc.caches[measName].Add(target)
			gc.logger.Printf("target %q added to the local cache", target)
		}
		// do not write updates with nil values to cache.
		notif := &gnmi.Notification{
			Timestamp: rsp.Update.GetTimestamp(),
			Prefix:    rsp.Update.GetPrefix(),
			Update:    make([]*gnmi.Update, 0, len(rsp.Update.GetUpdate())),
			Delete:    rsp.Update.GetDelete(),
			Atomic:    rsp.Update.GetAtomic(),
		}
		for _, upd := range rsp.Update.GetUpdate() {
			if upd.Val == nil {
				continue
			}
			notif.Update = append(notif.Update, upd)
		}
		if len(notif.Update) == 0 {
			return
		}
		err = gc.caches[measName].GnmiUpdate(notif)
		if err != nil && gc.debug {
			gc.logger.Printf("failed to update gNMI cache: %v", err)
		}
		return
	}
}

func (gc *GnmiCache) ReadEvents() []*formatters.EventMsg {
	var err error
	gc.Lock()
	defer gc.Unlock()
	evChan := make(chan []*formatters.EventMsg)
	events := make([]*formatters.EventMsg, 0)
	doneCh := make(chan struct{})
	// this go routine will collect all the events
	// from the cache queries
	go func() {
		for evs := range evChan {
			events = append(events, evs...)
		}
		close(doneCh)
	}()

	now := time.Now()
	wg := new(sync.WaitGroup)
	wg.Add(len(gc.caches))
	for name, c := range gc.caches {
		go func(c *gnmiCache.Cache, name string) {
			defer wg.Done()
			err = c.Query("*", []string{},
				func(_ []string, l *ctree.Leaf, v interface{}) error {
					if err != nil {
						return err
					}
					switch notif := v.(type) {
					case *gnmi.Notification:
						if gc.expiration > 0 &&
							time.Unix(0, notif.Timestamp).Before(now.Add(time.Duration(-gc.expiration))) {
							return nil
						}
						// build events without processors
						events, err := formatters.ResponseToEventMsgs(name,
							&gnmi.SubscribeResponse{
								Response: &gnmi.SubscribeResponse_Update{Update: notif},
							},
							outputs.Meta{"subscription-name": name})
						if err != nil {
							gc.logger.Printf("failed to convert message to event: %v", err)
							return nil
						}
						evChan <- events
					}
					return nil
				})
			if err != nil {
				gc.logger.Printf("failed prometheus cache query:%v", err)
				return
			}
		}(c, name)
	}
	wg.Wait()
	close(evChan)
	// wait for events to be appended to the array
	<-doneCh
	return events
}

func (gc *GnmiCache) ReadNotifications() []*gnmi.Notification {
	var err error
	gc.Lock()
	defer gc.Unlock()
	notificationChan := make(chan *gnmi.Notification)
	notifications := make([]*gnmi.Notification, 0)
	doneCh := make(chan struct{})
	// this go routine will collect all the notifications
	// from the cache queries
	go func() {
		for notif := range notificationChan {
			notifications = append(notifications, notif)
		}
		close(doneCh)
	}()

	now := time.Now()
	wg := new(sync.WaitGroup)
	wg.Add(len(gc.caches))
	for name, c := range gc.caches {
		go func(c *gnmiCache.Cache, name string) {
			defer wg.Done()
			err = c.Query("*", []string{},
				func(_ []string, l *ctree.Leaf, v interface{}) error {
					if err != nil {
						return err
					}
					switch notif := v.(type) {
					case *gnmi.Notification:
						if gc.expiration > 0 &&
							time.Unix(0, notif.Timestamp).Before(now.Add(time.Duration(-gc.expiration))) {
							return nil
						}
						notificationChan <- notif
					}
					return nil
				})
			if err != nil {
				gc.logger.Printf("failed prometheus cache query:%v", err)
				return
			}
		}(c, name)
	}
	wg.Wait()
	close(notificationChan)
	// wait for notifications to be appended to the array
	<-doneCh
	return notifications
}
