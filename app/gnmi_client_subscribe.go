package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/karimra/gnmic/lockers"
	"github.com/karimra/gnmic/outputs"
	"github.com/openconfig/gnmi/proto/gnmi"
)

type subscriptionRequest struct {
	// subscription name
	name string
	// gNMI subscription request
	req *gnmi.SubscribeRequest
}

func (a *App) TargetSubscribeStream(ctx context.Context, name string) {
	lockKey := a.targetLockKey(name)
START:
	nctx, cancel := context.WithCancel(ctx)
	a.operLock.Lock()
	if cfn, ok := a.targetsLockFn[name]; ok {
		cfn()
	}
	a.targetsLockFn[name] = cancel
	a.operLock.Unlock()
	select {
	// check if the context was canceled before retrying
	case <-nctx.Done():
		return
	default:
		err := a.initTarget(name)
		if err != nil {
			a.Logger.Printf("failed to initialize target %q: %v", name, err)
			return
		}
		select {
		case <-nctx.Done():
			return
		default:
			if a.locker != nil {
				a.Logger.Printf("acquiring lock for target %q", name)
				ok, err := a.locker.Lock(nctx, lockKey, []byte(a.Config.Clustering.InstanceName))
				if err == lockers.ErrCanceled {
					a.Logger.Printf("lock attempt for target %q canceled", name)
					return
				}
				if err != nil {
					a.Logger.Printf("failed to lock target %q: %v", name, err)
					time.Sleep(a.Config.LocalFlags.SubscribeLockRetry)
					goto START
				}
				if !ok {
					time.Sleep(a.Config.LocalFlags.SubscribeLockRetry)
					goto START
				}
				a.Logger.Printf("acquired lock for target %q", name)
			}
			select {
			case <-nctx.Done():
				return
			default:
				a.operLock.RLock()
				a.targetsChan <- a.Targets[name]
				a.operLock.RUnlock()
				a.Logger.Printf("queuing target %q", name)
			}
			a.Logger.Printf("subscribing to target: %q", name)
			go func() {
				err := a.clientSubscribe(nctx, name)
				if err != nil {
					a.Logger.Printf("failed to subscribe: %v", err)
					return
				}
			}()
			if a.locker != nil {
				doneChan, errChan := a.locker.KeepLock(nctx, lockKey)
				for {
					select {
					case <-nctx.Done():
						a.Logger.Printf("target %q stopped: %v", name, nctx.Err())
						return
					case <-doneChan:
						a.Logger.Printf("target lock %q removed", name)
						return
					case err := <-errChan:
						a.Logger.Printf("failed to maintain target %q lock: %v", name, err)
						a.stopTarget(ctx, name)
						if errors.Is(err, context.Canceled) {
							return
						}
						time.Sleep(a.Config.LocalFlags.SubscribeLockRetry)
						goto START
					}
				}
			}
		}
	}
}

func (a *App) TargetSubscribeOnce(ctx context.Context, name string) error {
	nctx, cancel := context.WithCancel(ctx)
	defer cancel()
	err := a.initTarget(name)
	if err != nil {
		a.Logger.Printf("failed to initialize target %q: %v", name, err)
		return err
	}
	a.Logger.Printf("subscribing to target: %q", name)
	err = a.clientSubscribeOnce(nctx, name)
	if err != nil {
		a.Logger.Printf("failed to subscribe: %v", err)
		return err
	}
	return nil
}

func (a *App) TargetSubscribePoll(ctx context.Context, name string) {
	nctx, cancel := context.WithCancel(ctx)
	a.operLock.Lock()
	if cfn, ok := a.targetsLockFn[name]; ok {
		cfn()
	}
	a.targetsLockFn[name] = cancel
	a.operLock.Unlock()
	err := a.initTarget(name)
	if err != nil {
		a.Logger.Printf("failed to initialize target %q: %v", name, err)
		return
	}
	select {
	case <-nctx.Done():
		return
	case a.targetsChan <- a.Targets[name]:
		a.Logger.Printf("queuing target %q", name)
	}
	a.Logger.Printf("subscribing to target: %q", name)
	go func() {
		err := a.clientSubscribe(nctx, name)
		if err != nil {
			a.Logger.Printf("failed to subscribe: %v", err)
			return
		}
	}()
}

func (a *App) clientSubscribe(ctx context.Context, tName string) error {
	if t, ok := a.Targets[tName]; ok {
		subscriptionsConfigs := t.Subscriptions
		if len(subscriptionsConfigs) == 0 {
			subscriptionsConfigs = a.Config.Subscriptions
		}
		if len(subscriptionsConfigs) == 0 {
			return fmt.Errorf("target %q has no subscriptions defined", tName)
		}
		subRequests := make([]subscriptionRequest, 0)
		for _, sc := range subscriptionsConfigs {
			req, err := sc.CreateSubscribeRequest(tName)
			if err != nil {
				return err
			}
			subRequests = append(subRequests, subscriptionRequest{name: sc.Name, req: req})
		}
		gnmiCtx, cancel := context.WithCancel(ctx)
		t.Cfn = cancel
	CRCLIENT:
		select {
		case <-gnmiCtx.Done():
		default:
			if err := t.CreateGNMIClient(gnmiCtx, a.dialOpts...); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					a.Logger.Printf("failed to initialize target %q timeout (%s) reached", tName, t.Config.Timeout)
				} else {
					a.Logger.Printf("failed to initialize target %q: %v", tName, err)
				}
				a.Logger.Printf("retrying target %q in %s", tName, t.Config.RetryTimer)
				time.Sleep(t.Config.RetryTimer)
				goto CRCLIENT
			}
		}
		a.Logger.Printf("target %q gNMI client created", t.Config.Name)

		for _, sreq := range subRequests {
			a.Logger.Printf("sending gNMI SubscribeRequest: subscribe='%+v', mode='%+v', encoding='%+v', to %s",
				sreq.req, sreq.req.GetSubscribe().GetMode(), sreq.req.GetSubscribe().GetEncoding(), t.Config.Name)
			go t.Subscribe(gnmiCtx, sreq.req, sreq.name)
		}
		return nil
	}
	return fmt.Errorf("unknown target name: %q", tName)
}

func (a *App) clientSubscribeOnce(ctx context.Context, tName string) error {
	if t, ok := a.Targets[tName]; ok {
		subscriptionsConfigs := t.Subscriptions
		if len(subscriptionsConfigs) == 0 {
			subscriptionsConfigs = a.Config.Subscriptions
		}
		if len(subscriptionsConfigs) == 0 {
			return fmt.Errorf("target %q has no subscriptions defined", tName)
		}
		subRequests := make([]subscriptionRequest, 0)
		for _, sc := range subscriptionsConfigs {
			req, err := sc.CreateSubscribeRequest(tName)
			if err != nil {
				return err
			}
			subRequests = append(subRequests, subscriptionRequest{name: sc.Name, req: req})
		}
		gnmiCtx, cancel := context.WithCancel(ctx)
		t.Cfn = cancel
	CRCLIENT:
		if err := t.CreateGNMIClient(gnmiCtx, a.dialOpts...); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				a.Logger.Printf("failed to initialize target %q timeout (%s) reached", tName, t.Config.Timeout)
			} else {
				a.Logger.Printf("failed to initialize target %q: %v", tName, err)
			}
			a.Logger.Printf("retrying target %q in %s", tName, t.Config.RetryTimer)
			time.Sleep(t.Config.RetryTimer)
			goto CRCLIENT

		}
		a.Logger.Printf("target %q gNMI client created", t.Config.Name)
	OUTER:
		for _, sreq := range subRequests {
			a.Logger.Printf("sending gNMI SubscribeRequest: subscribe='%+v', mode='%+v', encoding='%+v', to %s",
				sreq.req, sreq.req.GetSubscribe().GetMode(), sreq.req.GetSubscribe().GetEncoding(), t.Config.Name)
			rspCh, errCh := t.SubscribeOnce(gnmiCtx, sreq.req, sreq.name)
			for {
				select {
				case err := <-errCh:
					if errors.Is(err, io.EOF) {
						a.Logger.Printf("target %q, subscription %q closed stream(EOF)", t.Config.Name, sreq.name)
						close(rspCh)
						// next subscription or end
						continue OUTER
					}
					return err
				case rsp := <-rspCh:
					switch rsp.Response.(type) {
					case *gnmi.SubscribeResponse_SyncResponse:
						a.Logger.Printf("target %q, subscription %q received sync response", t.Config.Name, sreq.name)
						return nil
					default:
						m := outputs.Meta{"source": t.Config.Name, "format": a.Config.Format, "subscription-name": sreq.name}
						a.Export(ctx, rsp, m, t.Config.Outputs...)
					}
				}
			}
		}
		return nil
	}
	return fmt.Errorf("unknown target name: %q", tName)
}

// clientSubscribePoll sends a gnmi.SubscribeRequest_Poll to targetName and returns the response and an error,
// it uses the targetName and the subscriptionName strings to find the gnmi.GNMI_SubscribeClient
func (a *App) clientSubscribePoll(targetName, subscriptionName string) (*gnmi.SubscribeResponse, error) {
	a.operLock.RLock()
	defer a.operLock.RUnlock()
	if t, ok := a.Targets[targetName]; ok {
		if sub, ok := t.Subscriptions[subscriptionName]; ok {
			if strings.ToUpper(sub.Mode) != "POLL" {
				return nil, fmt.Errorf("subscription %q is not a POLL subscription", subscriptionName)
			}
			if subClient, ok := t.SubscribeClients[subscriptionName]; ok {
				err := subClient.Send(&gnmi.SubscribeRequest{
					Request: &gnmi.SubscribeRequest_Poll{
						Poll: new(gnmi.Poll),
					},
				})
				if err != nil {
					return nil, err
				}
				return subClient.Recv()
			}
			return nil, fmt.Errorf("subscribe-client not found %q", subscriptionName)
		}
		return nil, fmt.Errorf("unknown subscription name %q", subscriptionName)
	}
	return nil, fmt.Errorf("unknown target name %q", targetName)
}
