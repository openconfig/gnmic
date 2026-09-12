// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	ocCache "github.com/openconfig/gnmi/cache"
	"github.com/openconfig/gnmi/ctree"
	"github.com/openconfig/gnmi/match"
	"github.com/openconfig/gnmi/path"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/subscribe"
	gpath "github.com/openconfig/gnmic/pkg/api/path"
	"google.golang.org/protobuf/proto"
)

const defaultTimeout = 10 * time.Second

type gnmiCache struct {
	m      *sync.Mutex
	caches map[string]*subCache
	// match is the single on-change subscription tree, keyed by
	// [subscription-name, target, path elements...].
	// Readers register their query here before any data exists, so a
	// subscription cache or a target created after the read started is
	// matched without any re-resolution: the openconfig match glob is
	// evaluated on every update.
	match *match.Match

	logger     *slog.Logger
	expiration time.Duration
	debug      bool
}

type subCache struct {
	c *ocCache.Cache
}

// cacheUpdate is the value handed to the match clients: the cache leaf
// together with the name of the subscription cache it came from.
type cacheUpdate struct {
	sub  string
	leaf *ctree.Leaf
}

func (gc *gnmiCache) loadConfig(gcc *Config) {
	gc.expiration = gcc.Expiration
	gc.logger = slog.New(slog.DiscardHandler)
	gc.debug = gcc.Debug
}

func newGNMICache(cfg *Config, _ string, opts ...Option) *gnmiCache {
	if cfg == nil {
		cfg = new(Config)
	}
	gc := &gnmiCache{
		m:      new(sync.Mutex),
		match:  match.New(),
		caches: make(map[string]*subCache),
	}
	cfg.setDefaults()

	gc.loadConfig(cfg)
	for _, opt := range opts {
		opt(gc)
	}
	return gc
}

// updateFn returns the openconfig cache client callback for the subscription
// cache named sub. Every leaf update is pushed through the shared match tree
// under [sub, target, prefix elements...].
func (gc *gnmiCache) updateFn(sub string) func(*ctree.Leaf) {
	return func(n *ctree.Leaf) {
		switch v := n.Value().(type) {
		case *gnmi.Notification:
			// path.ToStrings with usePrefix=true yields [target, elems...]
			ts := path.ToStrings(v.GetPrefix(), true)
			// exact capacity: UpdateNotification appends the update path
			// to this slice for each update, a spare capacity would be shared.
			prefix := make([]string, 0, 1+len(ts))
			prefix = append(prefix, sub)
			prefix = append(prefix, ts...)
			subscribe.UpdateNotification(gc.match, &cacheUpdate{sub: sub, leaf: n}, v, prefix)
		}
	}
}

func (gc *gnmiCache) SetLogger(logger *slog.Logger) {
	gc.logger = BindLogger(logger, "oc")
}

func (gc *gnmiCache) Write(ctx context.Context, measName string, m proto.Message) {
	var err error
	switch rsp := m.ProtoReflect().Interface().(type) {
	case *gnmi.SubscribeResponse:
		switch rsp := rsp.GetResponse().(type) {
		case *gnmi.SubscribeResponse_Update:
			target := rsp.Update.GetPrefix().GetTarget()
			if target == "" {
				gc.logger.Warn("response missing target", "subscription", measName, "response", rsp)
				return
			}

			// if the update does not have a prefix path,
			// check that each update has a path.
			if len(rsp.Update.GetPrefix().GetElem()) == 0 {
				for _, upd := range rsp.Update.GetUpdate() {
					if len(upd.GetPath().GetElem()) == 0 {
						gc.logger.Warn("write failed: update has empty path", "update", upd)
						return
					}
				}
			}
			gc.m.Lock()
			sCache, ok := gc.caches[measName]
			if !ok {
				sCache = &subCache{
					c: ocCache.New(nil),
				}
				sCache.c.SetClient(gc.updateFn(measName))
				sCache.c.Add(target)
				gc.logger.Info("target added to local cache", "target", target, "subscription", measName)
				gc.caches[measName] = sCache
			}
			if !sCache.c.HasTarget(target) {
				sCache.c.Add(target)
				gc.logger.Info("target added to local cache", "target", target, "subscription", measName)
			}
			gc.m.Unlock()
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
			if len(notif.Update) == 0 && len(notif.Delete) == 0 {
				return
			}
			err = sCache.c.GnmiUpdate(notif)
			if err != nil {
				gc.logger.Error("failed to update gNMI cache", "err", err)
				return
			}
			return
		}
	}
}

func (gc *gnmiCache) ReadAll() (map[string][]*gnmi.Notification, error) {
	return gc.read("", "*", nil), nil
}

func (gc *gnmiCache) Read(sub, target string, p *gnmi.Path) (map[string][]*gnmi.Notification, error) {
	return gc.read(sub, target, p), nil
}

func (gc *gnmiCache) Subscribe(ctx context.Context, ro *ReadOpts) chan *Notification {
	if ro == nil {
		ro = new(ReadOpts)
	}

	ro.setDefaults()
	ch := make(chan *Notification)
	go gc.subscribe(ctx, ro, ch)

	return ch
}

func (gc *gnmiCache) subscribe(ctx context.Context, ro *ReadOpts, ch chan *Notification) {
	defer close(ch)
	switch ro.Mode {
	case ReadMode_Once:
		gc.handleSingleQuery(ctx, ro, ch)
	case ReadMode_StreamOnChange: // default:
		ro.SuppressRedundant = false
		gc.handleOnChangeQuery(ctx, ro, ch)
	case ReadMode_StreamSample:
		gc.handleSampledQuery(ctx, ro, ch)
	}
}

func (gc *gnmiCache) handleSingleQuery(ctx context.Context, ro *ReadOpts, ch chan *Notification) {
	if gc.debug {
		gc.logger.Debug("running single query", "target", ro.Target)
	}

	caches := gc.getCaches(ro.Subscription)

	if gc.debug {
		gc.logger.Debug("single query got caches", "count", len(caches))
	}
	wg := new(sync.WaitGroup)
	wg.Add(len(caches))

	for name, c := range caches {
		go func(name string, c *subCache) {
			defer wg.Done()
			if !c.c.HasTarget(ro.Target) {
				if gc.debug {
					gc.logger.Debug("subscription-cache does not have target", "subscription", name, "target", ro.Target)
				}
				return
			}
			for _, p := range ro.Paths {
				fp, err := path.CompletePath(p, nil)
				if err != nil {
					gc.logger.Error("failed to generate CompletePath", "path", p)
					ch <- &Notification{Name: name, Err: err}
					return
				}
				err = c.c.Query(ro.Target, fp,
					func(_ []string, l *ctree.Leaf, _ interface{}) error {
						if err != nil {
							return err
						}
						switch gl := l.Value().(type) {
						case *gnmi.Notification:
							if ro.OverrideTS {
								// override timestamp
								gl = proto.Clone(gl).(*gnmi.Notification)
								gl.Timestamp = time.Now().UnixNano()
							}
							//no suppress redundant, send to channel and return
							if !ro.SuppressRedundant {
								ch <- &Notification{Name: name, Notification: gl}
								return nil
							}
							// suppress redundant part
							if ro.lastSent == nil {
								ro.lastSent = make(map[string]*gnmi.TypedValue)
								ro.m = new(sync.RWMutex)
							}

							prefix := gpath.GnmiPathToXPath(gl.GetPrefix(), true)
							target := gl.GetPrefix().GetTarget()
							for _, upd := range gl.GetUpdate() {
								p := gpath.GnmiPathToXPath(upd.GetPath(), true)
								valXPath := strings.Join([]string{target, prefix, p}, "/")
								ro.m.RLock()
								sv, ok := ro.lastSent[valXPath]
								ro.m.RUnlock()
								if !ok || !proto.Equal(sv, upd.Val) {
									ch <- &Notification{
										Name: name,
										Notification: &gnmi.Notification{
											Timestamp: gl.GetTimestamp(),
											Prefix:    gl.GetPrefix(),
											Update:    []*gnmi.Update{upd},
										},
									}
									ro.m.Lock()
									ro.lastSent[valXPath] = upd.Val
									ro.m.Unlock()
								}
							}

							if gl.GetDelete() != nil {
								ch <- &Notification{
									Name: name,
									Notification: &gnmi.Notification{
										Timestamp: gl.GetTimestamp(),
										Prefix:    gl.GetPrefix(),
										Delete:    gl.GetDelete(),
									},
								}
							}
							return nil
						}
						return nil
					})
				if err != nil {
					gc.logger.Error("target failed internal cache query", "target", ro.Target, "err", err)
					ch <- &Notification{Name: name, Err: err}
					return
				}
			}
		}(name, c)
	}
	wg.Wait()
}

func (gc *gnmiCache) handleSampledQuery(ctx context.Context, ro *ReadOpts, ch chan *Notification) {
	if !ro.UpdatesOnly {
		gc.handleSingleQuery(ctx, ro, ch)
	}

	ticker := time.NewTicker(ro.SampleInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); gc.logger != nil && err != nil {
				if errors.Is(err, context.Canceled) {
					gc.logger.Info("periodic query stopped", "target", ro.Target, "err", err)
				} else {
					gc.logger.Error("periodic query stopped", "target", ro.Target, "err", err)
				}
			}
			return
		case <-ticker.C:
			gc.handleSingleQuery(ctx, ro, ch)
		}
	}
}

// handleOnChangeQuery serves a STREAM on-change read.
//
// The live query is registered in the shared match tree first, for every
// requested path, regardless of which subscription caches and targets exist
// at that moment. The current state is then sent from the caches that do
// exist. The read stays open until ctx is done: a subscription cache or a
// target that appears later is matched by the tree, nothing is resolved at
// subscribe time.
func (gc *gnmiCache) handleOnChangeQuery(ctx context.Context, ro *ReadOpts, ch chan *Notification) {
	sub := ro.Subscription
	if sub == "" || sub == match.Glob {
		sub = match.Glob
	}
	for _, p := range ro.Paths {
		cp, err := path.CompletePath(p, nil)
		if err != nil {
			gc.logger.Error("failed to generate CompletePath", "path", p)
			ch <- &Notification{Err: err}
			return
		}
		// live updates: [subscription, target, path...]
		fp := make([]string, 0, len(cp)+2)
		fp = append(fp, sub, ro.Target)
		fp = append(fp, cp...)
		remove := gc.match.AddQuery(fp, &matchClient{ch: ch})
		defer remove()

		// current state, from the caches that exist now.
		// Registered after the live query so no update falls in between.
		if !ro.UpdatesOnly {
			for name, c := range gc.getCaches(ro.Subscription) {
				if !c.c.HasTarget(ro.Target) {
					if gc.debug {
						gc.logger.Debug("subscription-cache does not have target", "subscription", name, "target", ro.Target)
					}
					continue
				}
				err = c.c.Query(ro.Target, cp,
					func(_ []string, l *ctree.Leaf, _ interface{}) error {
						switch gl := l.Value().(type) {
						case *gnmi.Notification:
							ch <- &Notification{Name: name, Notification: gl}
						}
						return nil
					})
				if err != nil {
					gc.logger.Error("failed to run cache query", "subscription", name, "target", ro.Target, "path", cp, "err", err)
					ch <- &Notification{Name: name, Err: err}
					return
				}
			}
		}
	}

	// on-change heartbeat: a sampled read over the same paths at the
	// heartbeat interval. The current state was sent above, so this one
	// starts with the first tick.
	if ro.HeartbeatInterval > 0 {
		gc.handleSampledQuery(ctx, &ReadOpts{
			Subscription:   ro.Subscription,
			Target:         ro.Target,
			Paths:          ro.Paths,
			Mode:           ReadMode_StreamSample,
			SampleInterval: ro.HeartbeatInterval,
			OverrideTS:     ro.OverrideTS,
			UpdatesOnly:    true,
		}, ch)
		return
	}
	<-ctx.Done()
}

func (gc *gnmiCache) Stop() {}

func (gc *gnmiCache) read(sub, target string, p *gnmi.Path) map[string][]*gnmi.Notification {
	notificationChan := make(chan *Notification)
	notifications := make(map[string][]*gnmi.Notification, 0)
	doneCh := make(chan struct{})
	// this go routine will collect all the notifications
	// from the cache queries
	go func() {
		for nn := range notificationChan {
			if _, ok := notifications[nn.Name]; !ok {
				notifications[nn.Name] = make([]*gnmi.Notification, 0)
			}
			notifications[nn.Name] = append(notifications[nn.Name], nn.Notification)
		}
		close(doneCh)
	}()
	if sub == "*" {
		sub = ""
	}
	now := time.Now()
	wg := new(sync.WaitGroup)
	caches := gc.getCaches(sub)
	wg.Add(len(caches))

	for name, c := range caches {
		go func(c *subCache, name string) {
			defer wg.Done()
			cp, err := path.CompletePath(p, nil)
			if err != nil {
				gc.logger.Error("failed to generate CompletePath", "path", p)
				return
			}
			err = c.c.Query(target, cp,
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
						notificationChan <- &Notification{
							Name:         name,
							Notification: notif,
						}
					}
					return nil
				})
			if err != nil {
				gc.logger.Error("failed cache query", "err", err)
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

func (gc *gnmiCache) getCaches(names ...string) map[string]*subCache {
	gc.m.Lock()
	defer gc.m.Unlock()

	caches := make(map[string]*subCache)
	numCaches := len(names)
	if numCaches == 0 || (numCaches == 1 && (names[0] == "" || names[0] == match.Glob)) {
		for n, c := range gc.caches {
			caches[n] = c
		}
		return caches
	}
	for _, n := range names {
		if c, ok := gc.caches[n]; ok {
			caches[n] = c
		}
	}
	return caches
}

func (gc *gnmiCache) DeleteTarget(name string) {
	caches := gc.getCaches()
	for _, c := range caches {
		c.c.Remove(name)
	}
}

// matchClient forwards the updates matched in the shared tree to a reader.
type matchClient struct {
	ch chan *Notification
}

func (m *matchClient) Update(n interface{}) {
	u, ok := n.(*cacheUpdate)
	if !ok {
		return
	}
	switch v := u.leaf.Value().(type) {
	case *gnmi.Notification:
		m.ch <- &Notification{
			Name:         u.sub,
			Notification: v,
		}
	}
}
