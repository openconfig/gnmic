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
		a.Logger.Info("failed to initialize target", "target", tc.Name, "err", err)
		return
	}
	select {
	// check if the context was canceled before retrying
	case <-nctx.Done():
		return
	default:
		if a.locker != nil {
			a.Logger.Info("acquiring lock for target", "target", tc.Name)
			ok, err := a.locker.Lock(nctx, lockKey, []byte(a.Config.Clustering.InstanceName))
			if err == lockers.ErrCanceled {
				a.Logger.Info("lock attempt for target canceled", "target", tc.Name)
				return
			}
			if err != nil {
				a.Logger.Info("failed to lock target", "target", tc.Name, "err", err)
				time.Sleep(a.Config.LocalFlags.SubscribeLockRetry)
				goto START
			}
			if !ok {
				time.Sleep(a.Config.LocalFlags.SubscribeLockRetry)
				goto START
			}
			a.Logger.Info("acquired lock for target", "target", tc.Name)
		}
		a.Logger.Info("queuing target", "target", tc.Name)
		a.targetsChan <- t
		a.Logger.Info("subscribing to target", "target", tc.Name)
		go func() {
			err := a.clientSubscribe(nctx, tc)
			if err != nil {
				a.Logger.Info("failed to subscribe", "err", err)
				return
			}
		}()
		if a.locker != nil {
			doneChan, errChan := a.locker.KeepLock(nctx, lockKey)
			for {
				select {
				case <-nctx.Done():
					a.Logger.Info("target stopped", "target", tc.Name, "err", nctx.Err())
					// drain errChan
					err := <-errChan
					a.Logger.Info("target keepLock returned", "target", tc.Name, "err", err)
					return
				case <-doneChan:
					a.Logger.Info("target lock removed", "target", tc.Name)
					return
				case err := <-errChan:
					a.Logger.Info("failed to maintain target lock", "target", tc.Name, "err", err)
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
		a.Logger.Info("failed to initialize target", "target", tc.Name, "err", err)
		return err
	}
	a.Logger.Info("subscribing to target", "target", tc.Name)
	err = a.clientSubscribeOnce(nctx, tc)
	if err != nil {
		a.Logger.Info("failed to subscribe", "err", err)
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
		a.Logger.Info("failed to initialize target", "target", tc.Name, "err", err)
		return
	}
	a.Logger.Info("subscribing to target", "target", tc.Name)
	err = a.clientSubscribe(nctx, tc)
	if err != nil {
		a.Logger.Info("failed to subscribe", "err", err)
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
				a.Logger.Info("failed to initialize target: timeout reached", "target", tc.Name, "timeout", t.Config.Timeout)
			} else {
				a.Logger.Info("failed to initialize target", "target", tc.Name, "err", err)
			}
			a.Logger.Info("retrying target", "target", tc.Name, "after", t.Config.RetryTimer)
			time.Sleep(t.Config.RetryTimer)
			goto CRCLIENT
		}
	}
	a.Logger.Info("target gNMI client created", "target", t.Config.Name)

	for _, sreq := range subRequests {
		a.Logger.Info("sending gNMI SubscribeRequest",
			"target", t.Config.Name,
			"subscribe", sreq.req,
			"mode", sreq.req.GetSubscribe().GetMode(),
			"encoding", sreq.req.GetSubscribe().GetEncoding(),
		)
		go t.Subscribe(gnmiCtx, sreq.req, sreq.name)
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
			a.Logger.Info("failed to initialize target: timeout reached", "target", tc.Name, "timeout", t.Config.Timeout)
		} else {
			a.Logger.Info("failed to initialize target", "target", tc.Name, "err", err)
		}
		a.Logger.Info("retrying target", "target", tc.Name, "after", t.Config.RetryTimer)
		time.Sleep(t.Config.RetryTimer)
		goto CRCLIENT

	}
	a.Logger.Info("target gNMI client created", "target", t.Config.Name)
OUTER:
	for _, sreq := range subRequests {
		a.Logger.Info("sending gNMI SubscribeRequest",
			"target", t.Config.Name,
			"subscribe", sreq.req,
			"mode", sreq.req.GetSubscribe().GetMode(),
			"encoding", sreq.req.GetSubscribe().GetEncoding(),
		)
		rspCh, errCh := t.SubscribeOnceChan(gnmiCtx, sreq.req)
		for {
			select {
			case err := <-errCh:
				if errors.Is(err, io.EOF) {
					a.Logger.Info("subscription closed stream (EOF)", "target", t.Config.Name, "subscription", sreq.name)
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
