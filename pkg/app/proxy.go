// © 2024 Nokia.
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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api/server"
	"github.com/openconfig/gnmic/pkg/api/target"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type targetSubscribeResponse struct {
	name string
	rsp  *gnmi.SubscribeResponse
}

func (a *App) ProxyPreRunE(cmd *cobra.Command, args []string) error {
	a.Config.SetLocalFlagsFromFile(cmd)
	a.createCollectorDialOpts()
	return nil
}

func (a *App) ProxyRunE(cmd *cobra.Command, args []string) error {
	err := a.Config.GetGNMIServer()
	if err != nil {
		return err
	}
	err = a.Config.GetAPIServer()
	if err != nil {
		return err
	}
	err = a.Config.GetLoader()
	if err != nil {
		return err
	}
	err = a.initTunnelServer(tunnel.ServerConfig{
		AddTargetHandler:    a.tunServerAddTargetSubscribeHandler,
		DeleteTargetHandler: a.tunServerDeleteTargetHandler,
		RegisterHandler:     a.tunServerRegisterHandler,
		Handler:             a.tunServerHandler,
	})
	if err != nil {
		return err
	}
	_, err = a.Config.GetTargets()
	if errors.Is(err, config.ErrNoTargetsFound) {
		if len(a.Config.FileConfig.GetStringMap("loader")) == 0 &&
			!a.Config.UseTunnelServer {
			return fmt.Errorf("failed reading targets config: %v", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed reading targets config: %v", err)
	}

	a.startAPIServer()
	go a.startLoaderProxy(cmd.Context())
	return a.startGNMIProxyServer(cmd.Context())
}

func (a *App) startGNMIProxyServer(ctx context.Context) error {
	s, err := server.New(server.Config{
		Address:              a.Config.GnmiServer.Address,
		MaxUnaryRPC:          a.Config.GnmiServer.MaxUnaryRPC,
		MaxStreamingRPC:      a.Config.GnmiServer.MaxSubscriptions,
		MaxRecvMsgSize:       a.Config.GnmiServer.MaxRecvMsgSize,
		MaxSendMsgSize:       a.Config.GnmiServer.MaxSendMsgSize,
		MaxConcurrentStreams: a.Config.GnmiServer.MaxConcurrentStreams,
		TCPKeepalive:         a.Config.GnmiServer.TCPKeepalive,
		Keepalive:            a.Config.GnmiServer.GRPCKeepalive.Convert(),
		HealthEnabled:        true,
		TLS:                  a.Config.GnmiServer.TLS,
	}, server.WithLogger(a.Logger),
		server.WithGetHandler(a.proxyGetHandler),
		server.WithSetHandler(a.proxySetHandler),
		server.WithSubscribeHandler(a.proxySubscribeHandler))
	if err != nil {
		return err
	}
	return s.Start(ctx)
}

func (a *App) proxyGetHandler(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	targetName := req.GetPrefix().GetTarget()
	pr, _ := peer.FromContext(ctx)
	a.Logger.Printf("received Get request from %q to target %q", pr.Addr, targetName)

	targets, err := a.selectTargets(ctx, targetName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not find targets: %v", err)
	}
	numTargets := len(targets)
	if numTargets == 0 {
		return nil, status.Errorf(codes.NotFound, "unknown target %q", targetName)
	}

	results := make(chan *gnmi.Notification)
	errChan := make(chan error, numTargets)

	response := &gnmi.GetResponse{
		// assume one notification target
		Notification: make([]*gnmi.Notification, 0, numTargets),
	}
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		for {
			select {
			case notif, ok := <-results:
				if !ok {
					close(done)
					return
				}
				response.Notification = append(response.Notification, notif)
			case <-ctx.Done():
				return
			}
		}
	}()

	wg := new(sync.WaitGroup)
	wg.Add(numTargets)

	for name, t := range targets {
		go func(name string, t *target.Target) {
			defer wg.Done()

			creq := proto.Clone(req).(*gnmi.GetRequest)
			if creq.GetPrefix() == nil {
				creq.Prefix = new(gnmi.Path)
			}
			if creq.GetPrefix().GetTarget() == "" || creq.GetPrefix().GetTarget() == "*" {
				creq.Prefix.Target = name
			}
			res, err := t.Get(ctx, creq)
			if err != nil {
				a.Logger.Printf("target %q err: %v", name, err)
				errChan <- fmt.Errorf("target %q err: %v", name, err)
				return
			}

			for _, n := range res.GetNotification() {
				if n.GetPrefix() == nil {
					n.Prefix = new(gnmi.Path)
				}
				if n.GetPrefix().GetTarget() == "" {
					n.Prefix.Target = name
				}
				results <- n
			}
		}(name, t)
	}
	wg.Wait()
	close(results)
	close(errChan)
	for err := range errChan {
		if err != nil {
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
	}
	<-done
	a.Logger.Printf("sending GetResponse to %q: %+v", pr.Addr, response)

	return response, nil
}

func (a *App) proxySetHandler(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	numUpdates := len(req.GetUpdate())
	numReplaces := len(req.GetReplace())
	numDeletes := len(req.GetDelete())
	numUnionReplace := len(req.GetUnionReplace())
	if numUpdates+numReplaces+numDeletes+numUnionReplace == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "missing update/replace/delete path(s)")
	}

	targetName := req.GetPrefix().GetTarget()
	pr, _ := peer.FromContext(ctx)
	a.Logger.Printf("received Set request from %q to target %q", pr.Addr, targetName)

	targets, err := a.selectTargets(ctx, targetName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not find targets: %v", err)
	}
	numTargets := len(targets)
	if numTargets == 0 {
		return nil, status.Errorf(codes.NotFound, "unknown target(s) %q", targetName)
	}
	results := make(chan *gnmi.UpdateResult)
	errChan := make(chan error, numTargets)

	response := &gnmi.SetResponse{
		// assume one update per target, per update/replace/delete
		Response: make([]*gnmi.UpdateResult, 0, numTargets*(numUpdates+numReplaces+numDeletes+numUnionReplace)),
	}
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		for {
			select {
			case upd, ok := <-results:
				if !ok {
					response.Timestamp = time.Now().UnixNano()
					close(done)
					return
				}
				response.Response = append(response.Response, upd)
			case <-ctx.Done():
				return
			}
		}
	}()
	wg := new(sync.WaitGroup)
	wg.Add(numTargets)
	for name, t := range targets {
		go func(name string, t *target.Target) {
			defer wg.Done()

			creq := proto.Clone(req).(*gnmi.SetRequest)
			if creq.GetPrefix() == nil {
				creq.Prefix = new(gnmi.Path)
			}
			if creq.GetPrefix().GetTarget() == "" || creq.GetPrefix().GetTarget() == "*" {
				creq.Prefix.Target = name
			}
			res, err := t.Set(ctx, creq)
			if err != nil {
				a.Logger.Printf("target %q err: %v", name, err)
				errChan <- fmt.Errorf("target %q err: %v", name, err)
				return
			}
			for _, upd := range res.GetResponse() {
				upd.Path.Target = name
				results <- upd
			}
		}(name, t)
	}
	wg.Wait()
	close(results)
	close(errChan)
	for err := range errChan {
		if err != nil {
			return nil, status.Errorf(codes.Internal, "%v", err)
		}
	}
	<-done
	a.Logger.Printf("sending SetResponse to %q: %+v", pr.Addr, response)
	return response, nil
}

func (a *App) proxySubscribeHandler(req *gnmi.SubscribeRequest, stream gnmi.GNMI_SubscribeServer) error {
	switch req.GetRequest().(type) {
	case *gnmi.SubscribeRequest_Poll:
		return status.Errorf(codes.InvalidArgument, "invalid request type: %T", req.GetRequest())
	case *gnmi.SubscribeRequest_Subscribe:
	}

	switch req.GetSubscribe().GetMode() {
	case gnmi.SubscriptionList_ONCE:
	case gnmi.SubscriptionList_STREAM:
	case gnmi.SubscriptionList_POLL:
		return status.Errorf(codes.Unimplemented, "subscribe mode POLL not implemented by the proxy")
	default:
		return status.Errorf(codes.InvalidArgument, "unknown subscribe request mode: %v", req.GetSubscribe().GetMode())
	}

	ctx := stream.Context()
	targetName := getTargetFromSubscribeRequest(req)

	targets, err := a.selectTargets(ctx, targetName)
	if err != nil {
		return status.Errorf(codes.Internal, "could not find target(s): %v", err)
	}
	numTargets := len(targets)
	if numTargets == 0 {
		return status.Errorf(codes.NotFound, "unknown target(s) %q", targetName)
	}

	switch req.GetSubscribe().GetMode() {
	case gnmi.SubscriptionList_ONCE:
		return a.proxySubscribeONCEHandler(req, stream, targets)
	case gnmi.SubscriptionList_STREAM:
		return a.proxySubscribeSTREAMHandler(req, stream, targets)
	}
	return nil
}

func (a *App) InitProxyFlags(cmd *cobra.Command) {
	cmd.ResetFlags()

}

func (a *App) proxySubscribeONCEHandler(req *gnmi.SubscribeRequest, stream gnmi.GNMI_SubscribeServer, targets map[string]*target.Target) error {
	ctx := stream.Context()
	numTargets := len(targets)

	results := make(chan *targetSubscribeResponse)
	errChan := make(chan error, numTargets)
	done := make(chan struct{})
	stop := make(chan struct{})

	go func() {
		defer close(done)
		syncs := make(map[string]struct{})
		for {
			select {
			case <-ctx.Done():
				return
			case r, ok := <-results:
				if !ok {
					return
				}
				switch r.rsp.Response.(type) {
				case *gnmi.SubscribeResponse_Update:
					if r.rsp.GetUpdate().GetPrefix() == nil {
						r.rsp.GetUpdate().Prefix = new(gnmi.Path)
					}
					if r.rsp.GetUpdate().GetPrefix().GetTarget() == "" {
						r.rsp.GetUpdate().GetPrefix().Target = r.name
					}
					err := stream.Send(r.rsp)
					if err != nil {
						close(stop)
						a.Logger.Printf("proxy stream send failed: %v", err)
						return
					}
				case *gnmi.SubscribeResponse_SyncResponse:
					syncs[r.name] = struct{}{}
					if len(syncs) >= numTargets {
						// send a single sync and stop
						err := stream.Send(&gnmi.SubscribeResponse{Response: &gnmi.SubscribeResponse_SyncResponse{SyncResponse: true}})
						if err != nil {
							a.Logger.Printf("proxy stream send Sync response failed: %v", err)
						}
						return
					}
				}
			}
		}
	}()

	wg := new(sync.WaitGroup)
	wg.Add(numTargets)

	for name, t := range targets {
		go func(name string, t *target.Target) {
			defer wg.Done()

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			creq := proto.Clone(req).(*gnmi.SubscribeRequest)
			if creq.GetSubscribe().GetPrefix() == nil {
				creq.GetSubscribe().Prefix = new(gnmi.Path)
			}
			if creq.GetSubscribe().GetPrefix().GetTarget() == "" || creq.GetSubscribe().GetPrefix().GetTarget() == "*" {
				creq.GetSubscribe().Prefix.Target = name
			}

			resCh, errCh := t.SubscribeOnceChan(ctx, creq)
			for {
				select {
				case <-ctx.Done():
					return
				case <-stop:
					cancel()
					return
				case r, ok := <-resCh:
					if !ok {
						return
					}

					results <- &targetSubscribeResponse{
						name: name,
						rsp:  r,
					}
				case err := <-errCh:
					if errors.Is(err, io.EOF) {
						a.Logger.Printf("target %q: closed stream(EOF)", t.Config.Name)
					} else {
						errChan <- err
					}
					return
				}
			}
		}(name, t)
	}
	wg.Wait()
	close(results)
	close(errChan)
	for err := range errChan {
		if err != nil {
			return status.Errorf(codes.Internal, "%v", err)
		}
	}
	<-done
	return nil
}

func (a *App) proxySubscribeSTREAMHandler(req *gnmi.SubscribeRequest, stream gnmi.GNMI_SubscribeServer, targets map[string]*target.Target) error {
	ctx := stream.Context()
	numTargets := len(targets)

	results := make(chan *targetSubscribeResponse)
	errChan := make(chan error, numTargets)
	done := make(chan struct{})
	// used to stop target subscriptions if
	// the northbound subscription stops.
	stop := make(chan struct{})

	go func() {
		defer close(done)
		for {
			select {
			case <-ctx.Done():
				return
			case r, ok := <-results:
				if !ok {
					return
				}
				switch r.rsp.Response.(type) {
				case *gnmi.SubscribeResponse_Update:
					if r.rsp.GetUpdate().GetPrefix() == nil {
						r.rsp.GetUpdate().Prefix = new(gnmi.Path)
					}
					if r.rsp.GetUpdate().GetPrefix().GetTarget() == "" {
						r.rsp.GetUpdate().GetPrefix().Target = r.name
					}
				}
				err := stream.Send(r.rsp)
				if err != nil {
					close(stop)
					a.Logger.Printf("proxy stream send failed: %v", err)
					return
				}
			}
		}
	}()

	pr, _ := peer.FromContext(ctx)

	wg := new(sync.WaitGroup)
	wg.Add(numTargets)

	for name, t := range targets {
		go func(name string, t *target.Target) {
			defer wg.Done()

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			creq := proto.Clone(req).(*gnmi.SubscribeRequest)
			if creq.GetSubscribe().GetPrefix() == nil {
				creq.GetSubscribe().Prefix = new(gnmi.Path)
			}
			if creq.GetSubscribe().GetPrefix().GetTarget() == "" || creq.GetSubscribe().GetPrefix().GetTarget() == "*" {
				creq.GetSubscribe().Prefix.Target = name
			}
			subName := pr.Addr.String() + "-" + name + "-" + strconv.Itoa(time.Now().Nanosecond())
			rspCh, errCh := t.SubscribeStreamChan(ctx, creq, subName)
			defer t.StopSubscription(subName)
			for {
				select {
				case <-ctx.Done():
					return
				case <-stop:
					cancel()
					return
				case r, ok := <-rspCh:
					if !ok {
						return
					}
					results <- &targetSubscribeResponse{
						name: name,
						rsp:  r,
					}
				case err, ok := <-errCh:
					if !ok {
						return
					}
					errChan <- err
					return
				}
			}
		}(name, t)
	}
	wg.Wait()
	close(results)
	close(errChan)
	for err := range errChan {
		if err != nil {
			return status.Errorf(codes.Internal, "%v", err)
		}
	}
	<-done
	return nil
}

func getTargetFromSubscribeRequest(req *gnmi.SubscribeRequest) string {
	switch req.GetRequest().(type) {
	case *gnmi.SubscribeRequest_Poll:
	case *gnmi.SubscribeRequest_Subscribe:
		return req.GetSubscribe().GetPrefix().GetTarget()
	}
	return ""
}

func (a *App) selectTargets(ctx context.Context, tn string) (map[string]*target.Target, error) {
	targets := make(map[string]*target.Target)

	a.operLock.Lock()
	defer a.operLock.Unlock()
	a.configLock.Lock()
	defer a.configLock.Unlock()

	if tn == "" || tn == "*" {
		for n, tc := range a.Config.Targets {
			targetName := utils.GetHost(n)
			if t, ok := a.Targets[targetName]; ok {
				targets[targetName] = t
				continue
			}
			t, err := a.createTarget(ctx, tc)
			if err != nil {
				return nil, err
			}
			a.Targets[targetName] = t
			targets[n] = t
		}
		return targets, nil
	}
	targetsNames := strings.Split(tn, ",")

	for i := range targetsNames {
		for n, t := range a.Targets {
			if utils.GetHost(n) == targetsNames[i] {
				targets[n] = t
			}
		}
	}
	if len(targets) == len(targetsNames) {
		return targets, nil
	}

OUTER:
	for i := range targetsNames {
		for n, tc := range a.Config.Targets {
			targetName := utils.GetHost(n)
			if _, ok := targets[targetName]; !ok && targetName == targetsNames[i] {
				t, err := a.createTarget(ctx, tc)
				if err != nil {
					return nil, err
				}
				a.Targets[targetName] = t
				targets[n] = t
				continue OUTER
			}
		}
		return nil, status.Errorf(codes.NotFound, "target %q is not known", targetsNames[i])
	}
	return targets, nil
}

func (a *App) createTarget(ctx context.Context, tc *types.TargetConfig) (*target.Target, error) {
	t := target.NewTarget(tc)
	targetDialOpts := a.dialOpts
	if a.Config.UseTunnelServer {
		targetDialOpts = append(targetDialOpts,
			grpc.WithContextDialer(a.tunDialerFn(ctx, tc)),
		)
		t.Config.Address = t.Config.Name
	}
	err := t.CreateGNMIClient(ctx, targetDialOpts...)
	if err != nil {
		return nil, err
	}
	return t, nil
}
