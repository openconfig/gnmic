// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package gnmiserver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/target"
	"github.com/openconfig/gnmic/pkg/logging"
)

// Get handles a gNMI Get RPC. Requests with paths under the `gnmic` origin
// are served by the internal Get handler, other requests are relayed to the
// target(s) selected via the request Prefix.Target field.
func (h *Handlers) Get(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	numPaths := len(req.GetPath())
	if numPaths == 0 && req.GetPrefix() == nil {
		return nil, status.Errorf(codes.InvalidArgument, "missing path")
	}

	origins := make(map[string]struct{})
	for _, p := range req.GetPath() {
		origins[p.GetOrigin()] = struct{}{}
		if p.GetOrigin() != "gnmic" {
			if _, ok := origins["gnmic"]; ok {
				return nil, status.Errorf(codes.InvalidArgument, "combining `gnmic` origin with other origin values is not supported")
			}
		}
	}

	if _, ok := origins["gnmic"]; ok {
		if h.internalGet == nil {
			return nil, status.Errorf(codes.Unimplemented, "`gnmic` origin is not supported")
		}
		return h.internalGet(ctx, req)
	}

	targetName := req.GetPrefix().GetTarget()
	pr, _ := peer.FromContext(ctx)
	h.logger.Info("received Get request", "peer", pr.Addr, "target", targetName)

	targets, cleanup, err := h.selectTargets(ctx, targetName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not find targets: %v", err)
	}
	defer cleanup()
	numTargets := len(targets)
	if numTargets == 0 {
		return nil, status.Errorf(codes.NotFound, "unknown target %q", targetName)
	}
	results := make(chan *gnmi.Notification)
	errChan := make(chan error, numTargets)

	response := &gnmi.GetResponse{
		// assume one notification per path per target
		Notification: make([]*gnmi.Notification, 0, numTargets*numPaths),
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
				logging.LogErrUnlessCanceled(h.logger, err, "target Get error", "target", name)
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
	if h.cfg.Debug {
		h.logger.Debug("sending GetResponse", "peer", pr.Addr, "response", response)
	}
	return response, nil
}

// Set handles a gNMI Set RPC, relaying it to the target(s) selected via the
// request Prefix.Target field.
func (h *Handlers) Set(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	numUpdates := len(req.GetUpdate())
	numReplaces := len(req.GetReplace())
	numDeletes := len(req.GetDelete())
	numUnionReplace := len(req.GetUnionReplace())
	if numUpdates+numReplaces+numDeletes+numUnionReplace == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "missing update/replace/delete path(s)")
	}

	targetName := req.GetPrefix().GetTarget()
	pr, _ := peer.FromContext(ctx)
	h.logger.Info("received Set request", "peer", pr.Addr, "target", targetName)

	targets, cleanup, err := h.selectTargets(ctx, targetName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not find targets: %v", err)
	}
	defer cleanup()
	numTargets := len(targets)
	if numTargets == 0 {
		return nil, status.Errorf(codes.NotFound, "unknown target(s) %q", targetName)
	}
	results := make(chan *gnmi.UpdateResult)
	errChan := make(chan error, numTargets)

	response := &gnmi.SetResponse{
		// assume one update per target, per update/replace/delete
		Response: make([]*gnmi.UpdateResult, 0, numTargets*(numUpdates+numReplaces+numDeletes)),
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
				logging.LogErrUnlessCanceled(h.logger, err, "target Set error", "target", name)
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
	h.logger.Info("sending SetResponse", "peer", pr.Addr, "response", response)
	return response, nil
}
