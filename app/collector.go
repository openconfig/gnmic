package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/karimra/gnmic/outputs"
	"github.com/karimra/gnmic/target"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/protobuf/proto"
)

const (
	subscriptionModeONCE = "ONCE"
	subscriptionModePOLL = "POLL"
)

func (a *App) StartCollector(ctx context.Context) {
	defer func() {
		for _, o := range a.Outputs {
			o.Close()
		}
	}()

	for t := range a.targetsChan {
		if a.Config.Debug {
			a.Logger.Printf("starting target %+v", t)
		}
		if t == nil {
			continue
		}
		a.operLock.RLock()
		_, ok := a.activeTargets[t.Config.Name]
		a.operLock.RUnlock()
		if ok {
			if a.Config.Debug {
				a.Logger.Printf("target %q listener already active", t.Config.Name)
			}
			continue
		}
		a.operLock.Lock()
		a.activeTargets[t.Config.Name] = struct{}{}
		a.operLock.Unlock()

		a.Logger.Printf("starting target %q listener", t.Config.Name)
		go func(t *target.Target) {
			numOnceSubscriptions := t.NumberOfOnceSubscriptions()
			remainingOnceSubscriptions := numOnceSubscriptions
			numSubscriptions := len(t.Subscriptions)
			rspChan, errChan := t.ReadSubscriptions()
			for {
				select {
				case rsp := <-rspChan:
					subscribeResponseReceivedCounter.WithLabelValues(t.Config.Name, rsp.SubscriptionConfig.Name).Add(1)
					if a.Config.Debug {
						a.Logger.Printf("target %q: gNMI Subscribe Response: %+v", t.Config.Name, rsp)
					}
					err := t.DecodeProtoBytes(rsp.Response)
					if err != nil {
						a.Logger.Printf("target %q: failed to decode proto bytes: %v", t.Config.Name, err)
						continue
					}
					m := outputs.Meta{
						"source":            t.Config.Name,
						"format":            a.Config.Format,
						"subscription-name": rsp.SubscriptionName,
					}
					if rsp.SubscriptionConfig.Target != "" {
						m["subscription-target"] = rsp.SubscriptionConfig.Target
					}
					for k, v := range t.Config.EventTags {
						m[k] = v
					}
					if a.subscriptionMode(rsp.SubscriptionName) == subscriptionModeONCE {
						a.Export(ctx, rsp.Response, m, t.Config.Outputs...)
					} else {
						go a.Export(ctx, rsp.Response, m, t.Config.Outputs...)
					}
					if remainingOnceSubscriptions > 0 {
						if a.subscriptionMode(rsp.SubscriptionName) == subscriptionModeONCE {
							switch rsp.Response.Response.(type) {
							case *gnmi.SubscribeResponse_SyncResponse:
								remainingOnceSubscriptions--
							}
						}
					}
					if remainingOnceSubscriptions == 0 && numSubscriptions == numOnceSubscriptions {
						a.operLock.Lock()
						delete(a.activeTargets, t.Config.Name)
						a.operLock.Unlock()
						return
					}
				case tErr := <-errChan:
					if errors.Is(tErr.Err, io.EOF) {
						a.Logger.Printf("target %q: subscription %s closed stream(EOF)", t.Config.Name, tErr.SubscriptionName)
					} else {
						a.Logger.Printf("target %q: subscription %s rcv error: %v", t.Config.Name, tErr.SubscriptionName, tErr.Err)
					}
					if remainingOnceSubscriptions > 0 {
						if a.subscriptionMode(tErr.SubscriptionName) == subscriptionModeONCE {
							remainingOnceSubscriptions--
						}
					}
					if remainingOnceSubscriptions == 0 && numSubscriptions == numOnceSubscriptions {
						a.operLock.Lock()
						delete(a.activeTargets, t.Config.Name)
						a.operLock.Unlock()
						return
					}
				case <-t.StopChan:
					a.operLock.Lock()
					delete(a.activeTargets, t.Config.Name)
					a.operLock.Unlock()
					a.Logger.Printf("target %q: listener stopped", t.Config.Name)
					return
				case <-ctx.Done():
					a.operLock.Lock()
					delete(a.activeTargets, t.Config.Name)
					a.operLock.Unlock()
					return
				}
			}
		}(t)
	}
	for range ctx.Done() {
		return
	}
}

func (a *App) Export(ctx context.Context, rsp *gnmi.SubscribeResponse, m outputs.Meta, outs ...string) {
	if rsp == nil {
		return
	}
	go a.updateCache(rsp, m)
	wg := new(sync.WaitGroup)
	// target has no outputs explicitly defined
	if len(outs) == 0 {
		wg.Add(len(a.Outputs))
		for _, o := range a.Outputs {
			go func(o outputs.Output) {
				defer wg.Done()
				defer a.operLock.RUnlock()
				a.operLock.RLock()
				o.Write(ctx, rsp, m)
			}(o)
		}
		wg.Wait()
		return
	}
	// write to the outputs defined under the target
	for _, name := range outs {
		a.operLock.RLock()
		if o, ok := a.Outputs[name]; ok {
			wg.Add(1)
			go func(o outputs.Output) {
				defer wg.Done()
				o.Write(ctx, rsp, m)
			}(o)
		}
		a.operLock.RUnlock()
	}
	wg.Wait()
}

func (a *App) updateCache(rsp *gnmi.SubscribeResponse, m outputs.Meta) {
	if a.c == nil {
		return
	}
	r := proto.Clone(rsp).(*gnmi.SubscribeResponse)
	switch r := r.Response.(type) {
	case *gnmi.SubscribeResponse_Update:
		if r.Update.GetPrefix() == nil {
			r.Update.Prefix = new(gnmi.Path)
		}
		if r.Update.GetPrefix().GetTarget() == "" {
			r.Update.Prefix.Target = utils.GetHost(m["source"])
		}
		target := r.Update.GetPrefix().GetTarget()
		if target == "" {
			a.Logger.Printf("response missing target")
			return
		}
		if !a.c.HasTarget(target) {
			a.c.Add(target)
			a.Logger.Printf("target %q added to the local cache", target)
		}
		if a.Config.Debug {
			a.Logger.Printf("updating target %q local cache", target)
		}
		err := a.c.GnmiUpdate(r.Update)
		if err != nil {
			a.Logger.Printf("failed to update gNMI cache: %v", err)
			return
		}
	}
}

func (a *App) subscriptionMode(name string) string {
	if sub, ok := a.Config.Subscriptions[name]; ok {
		return strings.ToUpper(sub.Mode)
	}
	return ""
}

func (a *App) GetModels(ctx context.Context, tc *types.TargetConfig) ([]*gnmi.ModelData, error) {
	capRsp, err := a.ClientCapabilities(ctx, tc)
	if err != nil {
		return nil, err
	}
	return capRsp.GetSupportedModels(), nil
}

// PolledSubscriptionsTargets returns a map of target name to a list of subscription names that have Mode == POLL
func (a *App) PolledSubscriptionsTargets() map[string][]string {
	result := make(map[string][]string)
	for tn, target := range a.Targets {
		for _, sub := range target.Subscriptions {
			if strings.ToUpper(sub.Mode) == subscriptionModePOLL {
				if result[tn] == nil {
					result[tn] = make([]string, 0)
				}
				result[tn] = append(result[tn], sub.Name)
			}
		}
	}
	return result
}

func (a *App) CreateTarget(name string) error {
	a.configLock.RLock()
	defer a.configLock.RUnlock()
	if tc, ok := a.Config.Targets[name]; ok {
		if _, ok := a.Targets[name]; !ok {
			a.operLock.Lock()
			a.Targets[tc.Name] = target.NewTarget(tc)
			a.operLock.Unlock()
		}
		return nil
	}
	return fmt.Errorf("unknown target %q", name)
}
