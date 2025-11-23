package targets_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"log/slog"
	"net"
	"os"
	"reflect"
	"slices"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/openconfig/gnmic/pkg/api/target"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/loaders"
	"github.com/openconfig/gnmic/pkg/lockers"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/openconfig/gnmic/pkg/pipeline"
	"github.com/openconfig/gnmic/pkg/store"
	"github.com/openconfig/gnmic/pkg/utils"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
)

type ManagedTarget struct {
	sync.RWMutex
	Name          string
	cfg           *types.TargetConfig
	T             *target.Target
	IntendedState atomic.Value // "enabled|disabled"
	State         atomic.Value // "running|stopped|starting|failed"
	FailedReason  atomic.Value // "error message"

	tunServer *tunnel.Server
	// reader
	readerCtx    context.Context
	readerCancel context.CancelFunc
	mu           *sync.Mutex
	readersCfn   map[string]context.CancelFunc
	readerWG     sync.WaitGroup

	outputs              map[string]struct{}
	appliedSubscriptions []string
}

type targetsStats struct {
	msgCount                  *prometheus.CounterVec
	subscribeResponseReceived *prometheus.CounterVec
}

func newManagedTarget(name string, cfg *types.TargetConfig, tunServer *tunnel.Server) *ManagedTarget {
	nt := target.NewTarget(cfg)
	mt := &ManagedTarget{
		Name:                 name,
		cfg:                  cfg,
		T:                    nt,
		tunServer:            tunServer,
		outputs:              make(map[string]struct{}, len(cfg.Outputs)),
		mu:                   new(sync.Mutex),
		readersCfn:           make(map[string]context.CancelFunc),
		appliedSubscriptions: make([]string, 0, len(cfg.Subscriptions)),
	}
	for _, output := range cfg.Outputs {
		mt.outputs[output] = struct{}{}
	}
	mt.State.Store("stopped")
	return mt
}

// TargetsManager owns target lifecycle (connect/stop) and per-target subscriptions hookups (started by SubscriptionsManager).
type TargetsManager struct {
	ctx    context.Context
	cancel context.CancelFunc
	store  store.Store[any]
	// pipe to outputsManager
	out chan *pipeline.Msg
	// target state
	mu      sync.RWMutex
	targets map[string]*ManagedTarget
	// subscriptions
	subscriptions map[string]*types.SubscriptionConfig
	ts            *tunnelServer
	logger        *slog.Logger
	stats         *targetsStats
	// clustring
	clustering  *config.Clustering
	locker      lockers.Locker
	incluster   bool
	mas         *sync.RWMutex
	assignments map[string]struct{}
	reg         *prometheus.Registry
}

func NewTargetsManager(ctx context.Context, store store.Store[any], pipeline chan *pipeline.Msg, reg *prometheus.Registry) *TargetsManager {
	ctx, cancel := context.WithCancel(ctx)
	ts := newTunnelServer(store, reg)
	tm := &TargetsManager{
		ctx:           ctx,
		cancel:        cancel,
		store:         store,
		out:           pipeline,
		targets:       map[string]*ManagedTarget{},
		subscriptions: map[string]*types.SubscriptionConfig{},
		ts:            ts,
		stats:         newTargetsStats(),
		mas:           new(sync.RWMutex),
		assignments:   make(map[string]struct{}),
		reg:           reg,
	}
	reg.MustRegister(tm.stats.msgCount)
	return tm
}

func newTargetsStats() *targetsStats {
	return &targetsStats{
		msgCount: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "gnmic",
			Subsystem: "targets",
			Name:      "gnmi_msg_receveid_count",
			Help:      "Number of messages received by the targets",
		}, []string{"target", "subscription"}),
		subscribeResponseReceived: subscribeResponseReceivedCounter,
	}
}

func (tm *TargetsManager) Start(locker lockers.Locker, wg *sync.WaitGroup) error {
	tm.logger = logging.NewLogger(tm.store, "component", "targets-manager")
	tm.logger.Info("starting targets manager")
	tm.locker = locker
	clustering, ok, err := tm.isClustering()
	if err != nil {
		return err
	}
	tm.incluster = ok && clustering != nil
	if tm.incluster {
		tm.logger.Info("clustering is enabled", "clustering", clustering)
		tm.clustering = clustering
	}

	// start tunnel server
	go func() {
		err := tm.ts.startTunnelServer(tm.ctx)
		if err != nil {
			tm.logger.Error("failed to start tunnel server", "error", err)
		}
	}()
	tm.logger.Info("starting targets watcher")
	targetsCh, targetsCancel, err := tm.store.Watch("targets", store.WithInitialReplay[any]())
	if err != nil {
		return err
	}
	tm.logger.Info("starting subscriptions watcher")
	subscriptionsCh, subscriptionsCancel, err := tm.store.Watch("subscriptions", store.WithInitialReplay[any]())
	if err != nil {
		return err
	}
	cfg, ok, err := tm.store.Get("loader", "loader")
	if err != nil {
		return fmt.Errorf("failed to get loader config: %w", err)
	}
	var loaderTargetOpCh <-chan *loaders.TargetOperation
	var loaderCfn context.CancelFunc
	if ok && cfg != nil {
		loaderCfg, ok := cfg.(map[string]any)
		if ok && len(loaderCfg) > 0 {
			loader, err := tm.initLoader(loaderCfg)
			if err != nil {
				return err
			}
			err = loader.Init(tm.ctx, loaderCfg,
				log.New(os.Stderr, "", 0), // TODO: use logger
				loaders.WithRegistry(tm.reg),
				// loaders.WithActions(actionsCfg), // TODO: add actions ? or drop from collector
			)
			if err != nil {
				return err
			}
			tm.logger.Info("starting loader", "loader", loader)

			var ctx context.Context
			ctx, loaderCfn = context.WithCancel(tm.ctx)
			go tm.startLoader(ctx, loader)
		}
	}

	var assignmentsCancel func()
	var assignmentsCh <-chan *store.Event[any]
	if clustering != nil {
		tm.logger.Info("clustering is enabled", "clustering", clustering)
		// watch assignments
		assignmentsCh, assignmentsCancel, err = tm.store.Watch("assignments", store.WithInitialReplay[any]()) // TODO: no initial replay ?
		if err != nil {
			if loaderCfn != nil {
				loaderCfn()
			}
			subscriptionsCancel()
			targetsCancel()
			return fmt.Errorf("failed to watch assignments: %w", err)
		}
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer targetsCancel()
		defer subscriptionsCancel()
		defer func() {
			if loaderCfn != nil {
				loaderCfn()
			}
		}()
		if clustering != nil {
			defer assignmentsCancel()
		}
		for {
			select {
			case <-tm.ctx.Done():
				return
			case ev, ok := <-targetsCh:
				if !ok {
					return
				}
				tm.logger.Debug("got target event", "eventType", ev.EventType, "name", ev.Name)
				if !tm.amIAssigned(ev.Name) {
					tm.logger.Debug("target is not assigned to this instance", "target", ev.Name)
					continue
				} else {
					tm.logger.Debug("target is assigned to this instance", "target", ev.Name)
				}
				switch ev.EventType {
				case store.EventTypeCreate, store.EventTypeUpdate:
					cfg := ev.Object.(*types.TargetConfig)
					tm.apply(ev.Name, cfg)
				case store.EventTypeDelete:
					tm.remove(ev.Name)
				}
			case op, ok := <-loaderTargetOpCh:
				if !ok {
					return
				}
				tm.logger.Info("got loader target operation", "operation", op)
				for _, add := range op.Add {
					_, err := tm.store.Set("targets", add.Name, add)
					if err != nil {
						tm.logger.Error("failed to add target from loader", "error", err, "target", add.Name)
					}
				}
				for _, del := range op.Del {
					_, _, err := tm.store.Delete("targets", del)
					if err != nil {
						tm.logger.Error("failed to delete target from loader", "error", err, "target", del)
					}
				}
			case ev, ok := <-subscriptionsCh:
				if !ok {
					return
				}
				tm.logger.Info("got subscription event", "event", ev, "objectType", reflect.TypeOf(ev.Object))
				cfg, ok := ev.Object.(*types.SubscriptionConfig)
				if !ok {
					continue
				}
				switch ev.EventType {
				case store.EventTypeCreate:
					tm.applySubscription(ev.Name, *cfg)
				case store.EventTypeUpdate:
					tm.applySubscription(ev.Name, *cfg)
				case store.EventTypeDelete:
					tm.removeSubscription(ev.Name, *cfg) // TODO: in this case the config could be nil
				}
			case ev, ok := <-assignmentsCh:
				if !ok {
					return
				}
				tm.logger.Info("got assignment event", "event", ev)
				switch ev.EventType {
				case store.EventTypeCreate:
					tm.setAssigned(ev.Name, true)
				case store.EventTypeUpdate:
					tm.setAssigned(ev.Name, true) // can this happen? yes if I add epoch/term to assignments
				case store.EventTypeDelete:
					tm.setAssigned(ev.Name, false)
				}
				go tm.reconcileAssignment(ev.Name)
			}
		}
	}()

	return nil
}

func (tm *TargetsManager) Stop() {
	if tm.cancel != nil {
		tm.cancel()
		tm.cancel = nil
	}
}

func (tm *TargetsManager) apply(name string, cfg *types.TargetConfig) {
	tm.logger.Info("applying target config", "name", name, "cfg", cfg)

	var mt *ManagedTarget
	created := false

	tm.mu.Lock()
	mt = tm.targets[name]
	if mt == nil {
		mt = newManagedTarget(name, cfg.DeepCopy(), tm.ts.tunServer)
		tm.targets[name] = mt
		created = true
	}
	tm.mu.Unlock()

	if created {
		tm.logger.Info("starting created target", "name", name)
		mt.Lock()
		defer mt.Unlock()
		if err := tm.start(mt); err != nil {
			tm.logger.Error("failed to start target", "name", name, "error", err)
			mt.FailedReason.Store(err.Error())
			mt.State.Store("failed")
			return
		}
		mt.State.Store("running")
		mt.FailedReason.Store("")
		return
	}

	mt.Lock()
	defer mt.Unlock()

	if mt.T.Config.Equal(cfg) {
		return
	}
	tm.logger.Info("target config changed", "name", name, "old", mt.T.Config, "new", cfg)
	if !shouldReconnect(mt.T.Config, cfg) {
		// subscriptions
		// compare applied subscriptions with new subscriptions.
		// !Do not mutate the current config subscriptions list!.
		if !reflect.DeepEqual(mt.appliedSubscriptions, cfg.Subscriptions) {
			tm.logger.Info("subscriptions changed", "name", name, "old", mt.T.Config.Subscriptions, "new", cfg.Subscriptions)
			if added, removed := tm.compareSubscriptions(mt.T.Config.Subscriptions, cfg.Subscriptions); len(added) > 0 || len(removed) > 0 {
				tm.logger.Info("subscriptions added", "name", name, "added", added)
				tm.logger.Info("subscriptions removed", "name", name, "removed", removed)
				for _, sub := range added {
					tm.logger.Info("starting target subscription", "name", sub, "target", name)
					cfg, exists, err := tm.store.Get("subscriptions", sub)
					if err != nil {
						tm.logger.Error("failed to get subscription", "name", sub, "target", name, "error", err)
						continue
					}
					if !exists {
						tm.logger.Error("subscription not found", "name", sub, "target", name)
						continue
					}
					scfg := cfg.(*types.SubscriptionConfig)
					scfg.Name = sub
					mt.appliedSubscriptions = append(mt.appliedSubscriptions, sub)
					err = tm.startTargetSubscription(mt, scfg)
					if err != nil {
						tm.logger.Error("failed to start target subscription", "name", sub, "target", name, "error", err)
						continue
					}
				}
				for _, sub := range removed {
					mt.mu.Lock()
					cfn, exists := mt.readersCfn[sub]
					if exists {
						cfn()
						delete(mt.readersCfn, sub)
					}
					mt.mu.Unlock()
					tm.logger.Info("stopping target subscription", "name", sub, "target", name)
					mt.T.StopSubscription(sub)
					delete(mt.T.Subscriptions, sub)
					mt.appliedSubscriptions = slices.DeleteFunc(mt.appliedSubscriptions, func(s string) bool {
						return s == sub
					})
					tm.logger.Info("target subscription stopped", "name", sub, "target", name)
				}
			} else {
				tm.logger.Info("subscriptions unchanged", "name", name, "old", mt.T.Config.Subscriptions, "new", cfg.Subscriptions)
			}
			mt.T.Config.Subscriptions = cfg.Subscriptions
		} else {
			tm.logger.Info("subscriptions unchanged", "name", name, "old", mt.T.Config.Subscriptions, "new", cfg.Subscriptions)
		}
		// outputs
		if !reflect.DeepEqual(mt.T.Config.Outputs, cfg.Outputs) {
			tm.logger.Info("outputs changed", "name", name, "old", mt.T.Config.Outputs, "new", cfg.Outputs)
			if added, removed := tm.compareOutputs(mt.T.Config, cfg); len(added) > 0 || len(removed) > 0 {
				tm.logger.Info("outputs added", "name", name, "added", added)
				tm.logger.Info("outputs removed", "name", name, "removed", removed)
				for _, output := range added {
					mt.outputs[output] = struct{}{}
				}
				for _, output := range removed {
					delete(mt.outputs, output)
				}
			} else {
				tm.logger.Info("outputs unchanged", "name", name, "old", mt.T.Config.Outputs, "new", cfg.Outputs)
			}
			mt.T.Config.Outputs = cfg.Outputs
		} else {
			tm.logger.Info("outputs unchanged", "name", name, "old", mt.T.Config.Outputs, "new", cfg.Outputs)
		}
		return
	}

	// simply reconnect
	err := tm.stop(mt)
	if err != nil {
		tm.logger.Error("failed to stop target", "name", name, "error", err)
		mt.FailedReason.Store(err.Error())
		mt.State.Store("failed")
	}
	mt.T.Config = cfg
	err = tm.start(mt)
	if err != nil {
		tm.logger.Error("failed to start target", "name", name, "error", err)
		mt.FailedReason.Store(err.Error())
		mt.State.Store("failed")
	}
}

// assumes the managed target is locked
func (tm *TargetsManager) start(mt *ManagedTarget) error {
	tm.logger.Info("starting target", "name", mt.Name)
	if mt.State.Load() == "running" {
		return nil
	}
	mt.State.Store("starting")
	ctx, cfn := context.WithCancel(tm.ctx)
	mt.T.Cfn = cfn

	tm.logger.Info("creating gNMI client", "name", mt.Name)
	err := mt.T.CreateGNMIClient(ctx, tm.targetGRPCOpts(ctx, mt)...)
	if err != nil {
		tm.logger.Error("failed to create gNMI client", "name", mt.Name, "error", err)
		mt.State.Store("failed")
		mt.FailedReason.Store(err.Error())
		return err
	}
	if tm.locker != nil {
		tm.logger.Info("acquiring lock for target", "name", mt.Name)
		ok, err := tm.locker.Lock(ctx, tm.targetLockKey(mt.Name), []byte(tm.clustering.InstanceName))
		if err != nil {
			tm.logger.Error("failed to acquire lock for target", "name", mt.Name, "error", err)
			mt.State.Store("failed")
			mt.FailedReason.Store(err.Error())
			_ = tm.stop(mt)
			return err
		}
		if !ok {
			tm.logger.Error("failed to acquire lock for target", "name", mt.Name)
			mt.State.Store("failed")
			mt.FailedReason.Store("lock not acquired")
			_ = tm.stop(mt)
			return err
		}
		// TODO: keep lock
		go func() {
			doneCh, errCh := tm.locker.KeepLock(ctx, tm.targetLockKey(mt.Name))
			for {
				select {
				case <-doneCh:
					tm.logger.Info("lock for target released", "name", mt.Name)
					return
				case err := <-errCh:
					tm.logger.Error("failed to maintain lock for target", "name", mt.Name, "error", err)
					_ = tm.stop(mt)
					mt.State.Store("failed")
					mt.FailedReason.Store(err.Error())
					return
				case <-ctx.Done():
					tm.logger.Info("lock for target released", "name", mt.Name)
					_ = tm.stop(mt)
					return
				}
			}
		}()
	}
	tm.logger.Info("gNMI client created", "name", mt.Name)
	mt.State.Store("running")
	mt.FailedReason.Store("")
	tm.logger.Info("target started", "name", mt.Name)
	_, err = mt.T.Capabilities(ctx)
	if err != nil {
		tm.logger.Error("failed capabilities request", "name", mt.Name, "error", err)
		mt.State.Store("failed")
		mt.FailedReason.Store(err.Error())
		return err
	}
	tm.logger.Info("capabilities request successful", "name", mt.Name)

	// start subscriptions
	subs := mt.T.Config.Subscriptions
	if len(subs) == 0 {
		// if target has no explicit subs, attach all known subs
		tm.mu.RLock()
		subs = make([]string, 0, len(tm.subscriptions))
		for name := range tm.subscriptions {
			subs = append(subs, name)
		}
		tm.mu.RUnlock()
		// reflect the effective subs into the target's config so future diffs see them
		mt.appliedSubscriptions = append(mt.appliedSubscriptions, subs...)
	}
	for _, sub := range subs {
		tm.logger.Info("starting target subscription", "name", sub, "target", mt.Name)
		tm.mu.RLock()
		cfg := tm.subscriptions[sub]
		tm.mu.RUnlock()
		if cfg == nil {
			obj, exists, err := tm.store.Get("subscriptions", sub)
			if err != nil {
				tm.logger.Error("failed to get subscription", "name", sub, "target", mt.Name, "error", err)
				continue
			}
			if !exists {
				tm.logger.Error("subscription not found", "name", sub, "target", mt.Name)
				continue
			}
			c := obj.(*types.SubscriptionConfig)
			cfg = c
		}
		cfg.Name = sub
		err = tm.startTargetSubscription(mt, cfg)
		if err != nil {
			tm.logger.Error("failed to start target subscription", "name", sub, "target", mt.Name, "error", err)
			continue
		}
	}
	return nil
}

func (tm *TargetsManager) targetGRPCOpts(ctx context.Context, mt *ManagedTarget) []grpc.DialOption {
	if mt.cfg.TunnelTargetType != "" {
		return []grpc.DialOption{grpc.WithContextDialer(tm.tunDialerFn(ctx, mt))}
	}
	return nil
}

func (tm *TargetsManager) tunDialerFn(ctx context.Context, mt *ManagedTarget) func(context.Context, string) (net.Conn, error) {
	return func(_ context.Context, _ string) (net.Conn, error) {
		tt := tunnel.Target{ID: mt.cfg.Name, Type: mt.cfg.TunnelTargetType}
		ctx, cancel := context.WithTimeout(ctx, mt.cfg.Timeout)
		defer cancel()
		conn, err := tunnel.ServerConn(ctx, tm.ts.tunServer, &tt)
		if err != nil {
			tm.logger.Error("failed dialing tunnel connection for target", "name", mt.Name, "error", err)
			return nil, err
		}
		return conn, nil
	}
}

func (tm *TargetsManager) stop(mt *ManagedTarget) error {
	if mt.State.Load() == "stopped" {
		return nil
	}
	mt.State.Store("stopping")

	// stop reader loop
	if mt.readerCancel != nil {
		mt.readerCancel()
		mt.readerWG.Wait()
		mt.readerCancel = nil
	}

	// stop all per-target subscriptions and locker if any
	if mt.T.Cfn != nil {
		mt.T.Cfn()
	}
	tm.logger.Info("closing target", "name", mt.Name)
	err := mt.T.Close()
	if err != nil {
		tm.logger.Error("failed to close target", "name", mt.Name, "error", err)
	} else {
		tm.logger.Info("closed target", "name", mt.Name)
	}
	mt.State.Store("stopped")
	mt.FailedReason.Store("")
	if tm.locker != nil {
		tm.logger.Info("releasing lock for target", "name", mt.Name)
		err := tm.locker.Unlock(tm.ctx, tm.targetLockKey(mt.Name))
		if err != nil {
			tm.logger.Error("failed to release lock for target", "name", mt.Name, "error", err)
		}
	}
	return nil
}

func (tm *TargetsManager) remove(name string) {
	tm.mu.Lock()
	mt := tm.targets[name]
	delete(tm.targets, name)
	tm.mu.Unlock()
	if mt != nil {
		mt.Lock()
		_ = tm.stop(mt)
		mt.T = nil
		mt.outputs = nil
		mt.readerCtx = nil
		mt.readerCancel = nil
		mt.Unlock()
	}
}

// apply subscription to all targets that reference it or to those that do not reference any subscription
func (tm *TargetsManager) applySubscription(name string, cfg types.SubscriptionConfig) {
	tm.logger.Info("applying subscription", "name", name, "cfg", cfg)
	cfg.Name = name
	tm.mu.Lock()
	tm.subscriptions[name] = &cfg
	tm.logger.Info("subscriptions", "subscriptions", tm.subscriptions)
	for _, mt := range tm.targets {
		tm.logger.Info("target", "target", mt.Name, "subscriptions", mt.T.Config.Subscriptions)
		if len(mt.T.Config.Subscriptions) > 0 {
			if !slices.Contains(mt.T.Config.Subscriptions, name) {
				tm.logger.Info("subscription not in target's explicit list", "subscription", name, "target", mt.Name)
				continue
			}
		}
		tm.logger.Info("(re)starting target subscription", "name", name, "target", mt.Name)
		// Stop and WAIT for the old subscription to fully terminate
		mt.mu.Lock()
		cfn, exists := mt.readersCfn[name]
		if exists {
			tm.logger.Info("canceling subscription context", "name", name, "target", mt.Name)
			cfn() // Cancel the context
			tm.logger.Info("deleted subscription context", "name", name, "target", mt.Name)
			delete(mt.readersCfn, name) // Remove from map
		}
		mt.mu.Unlock()
		tm.logger.Info("stopping target subscription", "name", name, "target", mt.Name)
		mt.T.StopSubscription(name)
		tm.logger.Info("stopped target subscription", "name", name, "target", mt.Name)
		// Wait for the reader goroutine to finish
		mt.T.Subscriptions[name] = &cfg
		err := tm.startTargetSubscription(mt, &cfg)
		if err != nil {
			tm.logger.Error("failed to start target subscription", "subscription", name, "target", mt.Name, "error", err)
		}
	}
	tm.mu.Unlock()
}

// TODO: remove subscription from targets that already reference it and have it running
func (tm *TargetsManager) removeSubscription(name string, _ types.SubscriptionConfig) {
	tm.mu.Lock()
	delete(tm.subscriptions, name)
	for _, mt := range tm.targets {
		mt.mu.Lock()
		cfn, exists := mt.readersCfn[name]
		if exists {
			cfn()
			delete(mt.readersCfn, name)
		}
		mt.mu.Unlock()
		mt.T.StopSubscription(name)
		delete(mt.T.Subscriptions, name)
	}
	tm.mu.Unlock()
}

func (tm *TargetsManager) reconcileAssignment(name string) {
	if !tm.amIAssigned(name) {
		if mt := tm.Lookup(name); mt != nil && mt.State.Load() == "running" {
			_ = tm.stop(mt)
		}
		return
	}
	// get targetConfig
	cfg, ok := tm.getConfig(name)
	if !ok {
		tm.logger.Info("assigned but config not present yet; will retry on next event", "target", name)
		return
	}
	// Ensure ManagedTarget exists
	tm.mu.Lock()
	mt := tm.targets[name]
	if mt == nil {
		mt = newManagedTarget(name, cfg, tm.ts.tunServer)
		tm.targets[name] = mt
	}
	tm.mu.Unlock()

	// lock managed target
	mt.Lock()
	defer mt.Unlock()

	// check if config has changed
	if reflect.DeepEqual(mt.T.Config, cfg) {
		return
	}

	// check if should reconnect
	shouldReconnect := shouldReconnect(mt.T.Config, cfg)
	if !shouldReconnect {
		return
	}

	// simply reconnect
	err := tm.stop(mt)
	if err != nil {
		tm.logger.Error("failed to stop target", "name", name, "error", err)
		mt.FailedReason.Store(err.Error())
		mt.State.Store("failed")
	}
	mt.T.Config = cfg
	err = tm.start(mt)
	if err != nil {
		tm.logger.Error("failed to start target", "name", name, "error", err)
		mt.FailedReason.Store(err.Error())
		mt.State.Store("failed")
	}
}

func (tm *TargetsManager) getConfig(name string) (*types.TargetConfig, bool) {
	v, ok, err := tm.store.Get("targets", name)
	if err != nil || !ok || v == nil {
		return nil, false
	}
	cfg, ok := v.(*types.TargetConfig)
	return cfg, ok
}

func (tm *TargetsManager) Lookup(name string) *ManagedTarget {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.targets[name]
}

func (tm *TargetsManager) ForEach(fn func(*ManagedTarget)) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	for _, mt := range tm.targets {
		fn(mt)
	}
}

func (tm *TargetsManager) SetIntendedState(name string, state string) bool {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	mt := tm.targets[name]
	if mt == nil {
		return false
	}
	mt.Lock()
	defer mt.Unlock()

	currentState := mt.State.Load()
	switch state {
	case "enabled":
		if currentState == "running" || currentState == "starting" {
			return false
		} else {
			_ = tm.start(mt)
			mt.IntendedState.Store(state)
		}
	case "disabled":
		if currentState == "stopped" || currentState == "stopping" {
			return false
		} else {
			_ = tm.stop(mt)
			mt.IntendedState.Store(state)
		}
	}
	return true
}

func (tm *TargetsManager) GetIntendedState(name string) string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	mt := tm.targets[name]
	if mt == nil {
		return ""
	}
	v := mt.IntendedState.Load()
	str, ok := v.(string)
	if !ok {
		return ""
	}
	return str
}

func (tm *TargetsManager) startTargetSubscription(mt *ManagedTarget, cfg *types.SubscriptionConfig) error {
	tm.logger.Info("starting target subscription", "name", cfg.Name, "target", mt.Name)
	var defaultEncoding = "json"
	defaultEncodingVal, exists, err := tm.store.Get("globalConfig", "defaultEncoding")
	if err != nil {
		tm.logger.Error("failed to get default encoding", "error", err)
		return err
	}
	if exists {
		var ok bool
		defaultEncoding, ok = defaultEncodingVal.(string)
		if !ok {
			tm.logger.Error("default encoding is not a string", "defaultEncodingVal", defaultEncodingVal)
		}
	}
	subreq, err := utils.CreateSubscribeRequest(cfg, mt.T.Config, defaultEncoding)
	if err != nil {
		tm.logger.Error("failed to create subscribe request", "error", err)
		return err
	}
	tm.logger.Info("starting target Subscribe RPC", "name", cfg.Name, "target", mt.Name)

	mt.T.Subscriptions[cfg.Name] = cfg
	mt.readerWG.Add(1)
	sctx, cfn := context.WithCancel(tm.ctx)
	mt.mu.Lock()
	mt.readersCfn[cfg.Name] = cfn
	mt.mu.Unlock()

	respCh, errCh := mt.T.SubscribeChan(sctx, subreq, cfg.Name)
	go func() {
		defer mt.readerWG.Done()
		// defer cfn()
		for {
			select {
			case <-sctx.Done():
				return
			case resp, ok := <-respCh:
				if !ok {
					return
				}
				outs := func() map[string]struct{} {
					mt.RLock()
					defer mt.RUnlock()
					cp := make(map[string]struct{}, len(mt.outputs))
					for k := range mt.outputs {
						cp[k] = struct{}{}
					}
					return cp
				}()
				select {
				case tm.out <- &pipeline.Msg{
					Msg: resp.Response,
					Meta: outputs.Meta{
						"source":            mt.Name,
						"subscription-name": resp.SubscriptionName,
					},
					Outputs: outs,
				}:
				default:
					// If downstream is slow, you can drop, count, or block; here we drop to keep reader healthy.
					tm.logger.Warn("pipeline backpressure: dropping response", "target", mt.Name)
				}
			case err, ok := <-errCh:
				if !ok {
					return
				}
				tm.logger.Error("subscription error", "error", err)
			}
		}
	}()
	return nil
}

func shouldReconnect(old, new *types.TargetConfig) bool {
	if old == nil && new != nil {
		return true
	}
	if new == nil && old != nil {
		return true
	}

	ho, _ := hashConnSpec(old)
	hn, _ := hashConnSpec(new)
	return ho != hn
}

// TODO: optimize this
func (tm *TargetsManager) compareSubscriptions(old, new []string) (added, removed []string) {
	var subscriptionsList []string
	var err error
	if len(new) == 0 || len(old) == 0 {
		// get all subscriptions from the store
		subscriptionsList, err = tm.store.Keys("subscriptions")
		if err != nil {
			tm.logger.Error("failed to get subscriptions from store", "error", err)
			return nil, nil
		}
	}
	sort.Strings(subscriptionsList)
	if len(new) == 0 {
		new = subscriptionsList
	}
	if len(old) == 0 {
		old = subscriptionsList
	}

	oldSubs := make(map[string]struct{}, len(old))
	newSubs := make(map[string]struct{}, len(new))
	for _, sub := range old {
		oldSubs[sub] = struct{}{}
	}
	for _, sub := range new {
		newSubs[sub] = struct{}{}
	}
	for _, sub := range old {
		if _, ok := newSubs[sub]; !ok {
			removed = append(removed, sub)
		}
	}
	for _, sub := range new {
		if _, ok := oldSubs[sub]; !ok {
			added = append(added, sub)
		}
	}
	return added, removed
}

func (tm *TargetsManager) compareOutputs(old, new *types.TargetConfig) (added, removed []string) {
	if len(new.Outputs) == 0 {
		// get all outputs from the store
		outputs, err := tm.store.List("outputs")
		if err != nil {
			tm.logger.Error("failed to get outputs", "error", err)
			return nil, nil
		}
		new.Outputs = keys(outputs)
	}
	if len(old.Outputs) == 0 {
		// get all outputs from the store
		outputs, err := tm.store.List("outputs")
		if err != nil {
			tm.logger.Error("failed to get outputs", "error", err)
			return nil, nil
		}
		old.Outputs = keys(outputs)
		return nil, old.Outputs
	}
	oldOutputs := make(map[string]struct{}, len(old.Outputs))
	newOutputs := make(map[string]struct{}, len(new.Outputs))
	for _, output := range old.Outputs {
		oldOutputs[output] = struct{}{}
	}
	for _, output := range new.Outputs {
		newOutputs[output] = struct{}{}
	}
	for _, output := range old.Outputs {
		if _, ok := newOutputs[output]; !ok {
			removed = append(removed, output)
		}
	}
	for _, output := range new.Outputs {
		if _, ok := oldOutputs[output]; !ok {
			added = append(added, output)
		}
	}
	return added, removed
}

func keys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// connSpec is the set of target parameters that affect the connection
type connSpec struct {
	Address    string
	Username   string
	Password   string
	AuthScheme string
	Token      string
	Proxy      string

	Timeout       time.Duration
	TCPKeepalive  time.Duration
	GRPCKeepalive *types.ClientKeepalive

	// TLS
	Insecure      bool
	TLSCA         string
	TLSCert       string
	TLSKey        string
	SkipVerify    bool
	TLSServerName string
	TLSMinVersion string
	TLSMaxVersion string
	TLSVersion    string
	CipherSuites  []string

	// Dial options that affect transport
	Encoding string
	Gzip     bool
}

func hashConnSpec(cfg *types.TargetConfig) (uint64, error) {
	spec := connSpecFrom(cfg)
	b, err := json.Marshal(spec)
	if err != nil {
		return 0, err
	}
	h := fnv.New64a()
	_, _ = h.Write(b)
	return h.Sum64(), nil
}

func connSpecFrom(tc *types.TargetConfig) connSpec {
	cs := make([]string, len(tc.CipherSuites))
	copy(cs, tc.CipherSuites)
	sort.Strings(cs)

	spec := connSpec{
		Address:       tc.Address,
		Username:      val(tc.Username),
		Password:      val(tc.Password),
		AuthScheme:    tc.AuthScheme,
		Token:         val(tc.Token),
		Proxy:         tc.Proxy,
		Timeout:       tc.Timeout,
		TCPKeepalive:  tc.TCPKeepalive,
		GRPCKeepalive: tc.GRPCKeepalive,
		Insecure:      val(tc.Insecure),
		TLSCA:         val(tc.TLSCA),
		TLSCert:       val(tc.TLSCert),
		TLSKey:        val(tc.TLSKey),
		SkipVerify:    val(tc.SkipVerify),
		TLSServerName: tc.TLSServerName,
		TLSMinVersion: tc.TLSMinVersion,
		TLSMaxVersion: tc.TLSMaxVersion,
		TLSVersion:    tc.TLSVersion,
		CipherSuites:  cs,
		Encoding:      val(tc.Encoding),
		Gzip:          val(tc.Gzip),
	}
	return spec
}

func val[T any](p *T) T {
	var z T
	if p == nil {
		return z
	}
	return *p
}
