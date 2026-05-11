// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/grpctunnel/tunnel"
	"google.golang.org/grpc"

	"github.com/openconfig/gnmic/pkg/api"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/lockers"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/openconfig/gnmic/pkg/utils"
)

type subscriptionRequest struct {
	// subscription name
	name string
	// gNMI subscription request
	req *gnmi.SubscribeRequest
}

func (a *App) TargetSubscribeStream(ctx context.Context, tc *types.TargetConfig) {
	lockKey := a.targetLockKey(tc.Name)
START:
	nctx, cancel := context.WithCancel(ctx)
	a.operLock.Lock()
	if cfn, ok := a.targetsLockFn[tc.Name]; ok {
		cfn()
	}
	a.targetsLockFn[tc.Name] = cancel
	t, err := a.initTarget(tc)
	a.operLock.Unlock()
	if err != nil {
		a.Logger.Printf("failed to initialize target %q: %v", tc.Name, err)
		return
	}
	select {
	// check if the context was canceled before retrying
	case <-nctx.Done():
		return
	default:
		if a.locker != nil {
			a.Logger.Printf("acquiring lock for target %q", tc.Name)
			ok, err := a.locker.Lock(nctx, lockKey, []byte(a.Config.Clustering.InstanceName))
			if err == lockers.ErrCanceled {
				a.Logger.Printf("lock attempt for target %q canceled", tc.Name)
				return
			}
			if err != nil {
				a.Logger.Printf("failed to lock target %q: %v", tc.Name, err)
				time.Sleep(a.Config.LocalFlags.SubscribeLockRetry)
				goto START
			}
			if !ok {
				time.Sleep(a.Config.LocalFlags.SubscribeLockRetry)
				goto START
			}
			a.Logger.Printf("acquired lock for target %q", tc.Name)
		}
		a.Logger.Printf("queuing target %q", tc.Name)
		a.targetsChan <- t
		a.Logger.Printf("subscribing to target: %q", tc.Name)
		go func() {
			err := a.clientSubscribe(nctx, tc)
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
					a.Logger.Printf("target %q stopped: %v", tc.Name, nctx.Err())
					// drain errChan
					err := <-errChan
					a.Logger.Printf("target %q keepLock returned: %v", tc.Name, err)
					return
				case <-doneChan:
					a.Logger.Printf("target lock %q removed", tc.Name)
					return
				case err := <-errChan:
					a.Logger.Printf("failed to maintain target %q lock: %v", tc.Name, err)
					a.stopTarget(ctx, tc.Name)
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

func (a *App) TargetSubscribeOnce(ctx context.Context, tc *types.TargetConfig) error {
	nctx, cancel := context.WithCancel(ctx)
	defer cancel()
	a.operLock.Lock()
	_, err := a.initTarget(tc)
	a.operLock.Unlock()
	if err != nil {
		a.Logger.Printf("failed to initialize target %q: %v", tc.Name, err)
		return err
	}
	a.Logger.Printf("subscribing to target: %q", tc.Name)
	err = a.clientSubscribeOnce(nctx, tc)
	if err != nil {
		a.Logger.Printf("failed to subscribe: %v", err)
		return err
	}
	return nil
}

func (a *App) TargetSubscribePoll(ctx context.Context, tc *types.TargetConfig) {
	nctx, cancel := context.WithCancel(ctx)
	a.operLock.Lock()
	if cfn, ok := a.targetsLockFn[tc.Name]; ok {
		cfn()
	}
	a.targetsLockFn[tc.Name] = cancel
	_, err := a.initTarget(tc)
	a.operLock.Unlock()
	if err != nil {
		a.Logger.Printf("failed to initialize target %q: %v", tc.Name, err)
		return
	}
	a.Logger.Printf("subscribing to target: %q", tc.Name)
	err = a.clientSubscribe(nctx, tc)
	if err != nil {
		a.Logger.Printf("failed to subscribe: %v", err)
		return
	}
}

func (a *App) clientSubscribe(ctx context.Context, tc *types.TargetConfig) error {
	a.operLock.RLock()
	t, ok := a.Targets[tc.Name]
	a.operLock.RUnlock()

	if !ok {
		return fmt.Errorf("unknown target name: %q", tc.Name)
	}

	subscriptionsConfigs := t.Subscriptions
	if len(subscriptionsConfigs) == 0 {
		subscriptionsConfigs = a.Config.Subscriptions
	}
	if len(subscriptionsConfigs) == 0 {
		return fmt.Errorf("target %q has no subscriptions defined", tc.Name)
	}
	subRequests := make([]subscriptionRequest, 0, len(subscriptionsConfigs))
	for scName, sc := range subscriptionsConfigs {
		req, err := utils.CreateSubscribeRequest(sc, tc, a.Config.Encoding)
		if err != nil {
			if errors.Is(errors.Unwrap(err), config.ErrConfig) || errors.Is(errors.Unwrap(err), api.ErrInvalidValue) {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		}
		subRequests = append(subRequests, subscriptionRequest{name: scName, req: req})
	}
	if t.Cfn != nil {
		t.Cfn()
	}
	gnmiCtx, cancel := context.WithCancel(ctx)
	t.Cfn = cancel
CRCLIENT:
	select {
	case <-gnmiCtx.Done():
		return gnmiCtx.Err()
	default:
		targetDialOpts := make([]grpc.DialOption, len(a.dialOpts))
		copy(targetDialOpts, a.dialOpts)
		if a.Config.UseTunnelServer {
			a.ttm.Lock()
			a.tunTargetCfn[tunnel.Target{ID: tc.Name, Type: tc.TunnelTargetType}] = cancel
			a.ttm.Unlock()
			targetDialOpts = append(targetDialOpts,
				grpc.WithContextDialer(a.tunDialerFn(gnmiCtx, tc)),
			)
			// overwrite target address
			t.Config.Address = t.Config.Name
		}
		err := t.CreateGNMIClient(ctx, targetDialOpts...)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				a.Logger.Printf("failed to initialize target %q timeout (%s) reached", tc.Name, t.Config.Timeout)
			} else {
				a.Logger.Printf("failed to initialize target %q: %v", tc.Name, err)
			}
			a.Logger.Printf("retrying target %q in %s", tc.Name, t.Config.RetryTimer)
			time.Sleep(t.Config.RetryTimer)
			goto CRCLIENT
		}
	}
	a.Logger.Printf("target %q gNMI client created", t.Config.Name)

	for _, sreq := range subRequests {
		if a.Config.GlobalFlags.PrintRequest || a.Config.LocalFlags.SubscribeDryRun {
			err := a.PrintMsg(tc.Name, "Subscribe Request:", sreq.req)
			if err != nil {
				a.logError(fmt.Errorf("target %q Subscribe Request printing failed: %v", tc.Name, err))
			}
		}
		if a.Config.LocalFlags.SubscribeDryRun {
			continue
		}
		a.Logger.Printf("sending gNMI SubscribeRequest: subscribe='%+v', mode='%+v', encoding='%+v', to %s",
			sreq.req, sreq.req.GetSubscribe().GetMode(), sreq.req.GetSubscribe().GetEncoding(), t.Config.Name)
		go t.Subscribe(gnmiCtx, sreq.req, sreq.name)
	}
	if a.Config.LocalFlags.SubscribeDryRun {
		os.Exit(0)
	}
	return nil
}

func (a *App) clientSubscribeOnce(ctx context.Context, tc *types.TargetConfig) error {
	a.operLock.RLock()
	t, ok := a.Targets[tc.Name]
	a.operLock.RUnlock()
	if !ok {
		return fmt.Errorf("unknown target name: %q", tc.Name)
	}

	subscriptionsConfigs := t.Subscriptions
	if len(subscriptionsConfigs) == 0 {
		subscriptionsConfigs = a.Config.Subscriptions
	}
	if len(subscriptionsConfigs) == 0 {
		return fmt.Errorf("target %q has no subscriptions defined", tc.Name)
	}
	subRequests := make([]subscriptionRequest, 0)
	for _, sc := range subscriptionsConfigs {
		req, err := utils.CreateSubscribeRequest(sc, tc, a.Config.Encoding)
		if err != nil {
			if errors.Is(errors.Unwrap(err), config.ErrConfig) {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
		}
		subRequests = append(subRequests, subscriptionRequest{name: sc.Name, req: req})
	}
	gnmiCtx, cancel := context.WithCancel(ctx)
	t.Cfn = cancel
CRCLIENT:
	targetDialOpts := a.dialOpts
	if a.Config.UseTunnelServer {
		a.ttm.Lock()
		a.tunTargetCfn[tunnel.Target{ID: tc.Name, Type: tc.TunnelTargetType}] = cancel
		a.ttm.Unlock()
		targetDialOpts = append(targetDialOpts,
			grpc.WithContextDialer(a.tunDialerFn(gnmiCtx, tc)),
		)
		// overwrite target address
		t.Config.Address = t.Config.Name
	}
	if err := t.CreateGNMIClient(ctx, targetDialOpts...); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			a.Logger.Printf("failed to initialize target %q timeout (%s) reached", tc.Name, t.Config.Timeout)
		} else {
			a.Logger.Printf("failed to initialize target %q: %v", tc.Name, err)
		}
		a.Logger.Printf("retrying target %q in %s", tc.Name, t.Config.RetryTimer)
		time.Sleep(t.Config.RetryTimer)
		goto CRCLIENT

	}
	a.Logger.Printf("target %q gNMI client created", t.Config.Name)
OUTER:
	for _, sreq := range subRequests {
		if a.Config.GlobalFlags.PrintRequest || a.Config.LocalFlags.SubscribeDryRun {
			err := a.PrintMsg(tc.Name, "Subscribe Request:", sreq.req)
			if err != nil {
				a.logError(fmt.Errorf("target %q Subscribe Request printing failed: %v", tc.Name, err))
			}
		}
		if a.Config.LocalFlags.SubscribeDryRun {
			continue
		}
		a.Logger.Printf("sending gNMI SubscribeRequest: subscribe='%+v', mode='%+v', encoding='%+v', to %s",
			sreq.req, sreq.req.GetSubscribe().GetMode(), sreq.req.GetSubscribe().GetEncoding(), t.Config.Name)
		rspCh, errCh := t.SubscribeOnceChan(gnmiCtx, sreq.req)
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
				m := outputs.Meta{"source": t.Config.Name, "format": a.Config.Format, "subscription-name": sreq.name}
				a.export(ctx, rsp, m, t.Config.Outputs...)
			}
		}
	}
	return nil
}

func (a *App) clientSubscribePoll(ctx context.Context, targetName, subscriptionName string) error {
	a.operLock.RLock()
	t, ok := a.Targets[targetName]
	a.operLock.RUnlock()
	if !ok {
		return fmt.Errorf("unknown target name %q", targetName)
	}
	return t.SubscribePoll(ctx, subscriptionName)
}
