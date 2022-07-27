package cache

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/karimra/gnmic/utils"
	ocCache "github.com/openconfig/gnmi/cache"
	"github.com/openconfig/gnmi/ctree"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/protobuf/proto"
)

const (
	loggingPrefixOC = "[cache:%s] "
	defaultTimeout  = 10 * time.Second
)

type gnmiCache struct {
	sync.Mutex
	caches     map[string]*ocCache.Cache
	logger     *log.Logger
	expiration time.Duration
	debug      bool
}

func (gc *gnmiCache) loadConfig(gcc *Config) {
	gc.expiration = gcc.Expiration
	// gc.timeout = gcc.Timeout
	gc.logger = log.New(io.Discard, loggingPrefixOC, utils.DefaultLoggingFlags)
	gc.caches = make(map[string]*ocCache.Cache)
	gc.debug = gcc.Debug
}

func newGNMICache(cfg *Config, loggingPrefix string, opts ...Option) *gnmiCache {
	if cfg == nil {
		cfg = new(Config)
	}
	gc := new(gnmiCache)
	cfg.setDefaults()
	gc.loadConfig(cfg)
	for _, opt := range opts {
		opt(gc)
	}
	if gc.logger != nil {
		if loggingPrefix == "" {
			loggingPrefix = "oc"
		}
		gc.logger.SetPrefix(fmt.Sprintf(loggingPrefixOC, loggingPrefix))
	}
	return gc
}

func (gc *gnmiCache) SetLogger(logger *log.Logger) {
	if logger != nil && gc.logger != nil {
		gc.logger.SetOutput(logger.Writer())
		gc.logger.SetFlags(logger.Flags())
	}
}

func (gc *gnmiCache) Write(ctx context.Context, measName string, m proto.Message) {
	var err error
	switch rsp := m.ProtoReflect().Interface().(type) {
	case *gnmi.SubscribeResponse:
		switch rsp := rsp.GetResponse().(type) {
		case *gnmi.SubscribeResponse_Update:
			target := rsp.Update.GetPrefix().GetTarget()
			if target == "" {
				gc.logger.Printf("subscription=%q: response missing target: %v", measName, rsp)
				return
			}
			gc.Lock()
			defer gc.Unlock()
			if _, ok := gc.caches[measName]; !ok {
				gc.caches[measName] = ocCache.New(nil)
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
}

func (gc *gnmiCache) Read(_ context.Context, _ string, ro *ReadOpts) (map[string][]*gnmi.Notification, error) {
	return gc.readNotifications(), nil
}

// TODO: implement
func (gc *gnmiCache) Subscribe(ctx context.Context, so *ReadOpts) chan *notification { return nil }

func (gc *gnmiCache) Stop() {}

func (gc *gnmiCache) readNotifications() map[string][]*gnmi.Notification {
	var err error
	gc.Lock()
	defer gc.Unlock()
	notificationChan := make(chan *notification)
	notifications := make(map[string][]*gnmi.Notification, 0)
	doneCh := make(chan struct{})
	// this go routine will collect all the notifications
	// from the cache queries
	go func() {
		for nn := range notificationChan {
			if _, ok := notifications[nn.name]; !ok {
				notifications[nn.name] = make([]*gnmi.Notification, 0)
			}
			notifications[nn.name] = append(notifications[nn.name], nn.notification)
		}
		close(doneCh)
	}()

	now := time.Now()
	wg := new(sync.WaitGroup)
	wg.Add(len(gc.caches))
	for name, c := range gc.caches {
		go func(c *ocCache.Cache, name string) {
			defer wg.Done()
			err = c.Query("*", []string{},
				func(_ []string, _ *ctree.Leaf, v interface{}) error {
					if err != nil {
						return err
					}
					switch notif := v.(type) {
					case *gnmi.Notification:
						if gc.expiration > 0 &&
							time.Unix(0, notif.Timestamp).Before(now.Add(time.Duration(-gc.expiration))) {
							return nil
						}
						notificationChan <- &notification{
							name:         name,
							notification: notif,
						}
					}
					return nil
				})
			if err != nil {
				gc.logger.Printf("failed cache query:%v", err)
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
