// SPDX-License-Identifier: Apache-2.0

package gnmiserver

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"
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

// Get handles a gNMI Get RPC.
//
// Requests with paths under the `gnmic` origin are served by the internal Get
// handler. Every other path is answered from the cache: the notifications
// stored for the selected target(s) under the requested path(s) are returned
// as they were received from the targets, with their timestamps and encoding.
// The request `type` and `encoding` fields are accepted and ignored. A path
// with no cached data yields an empty response, not an error.
//
// The target is taken from Prefix.Target: empty or `*` selects every target,
// a comma separated list selects several. Relaying a Get to the targets is
// the job of `gnmic proxy`.
func (h *Handlers) Get(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	numPaths := len(req.GetPath())
	if numPaths == 0 && req.GetPrefix() == nil {
		return nil, status.Errorf(codes.InvalidArgument, "missing path")
	}

	origins := make(map[string]struct{})
	for _, p := range req.GetPath() {
		origins[p.GetOrigin()] = struct{}{}
	}
	if _, ok := origins["gnmic"]; ok {
		if len(origins) > 1 {
			return nil, status.Errorf(codes.InvalidArgument, "combining `gnmic` origin with other origin values is not supported")
		}
		if h.internalGet == nil {
			return nil, status.Errorf(codes.Unimplemented, "`gnmic` origin is not supported")
		}
		return h.internalGet(ctx, req)
	}
	if h.cache == nil {
		return nil, status.Errorf(codes.Unimplemented, "no cache configured, cannot serve Get RPCs")
	}

	targetName := req.GetPrefix().GetTarget()
	pr, _ := peer.FromContext(ctx)
	h.logger.Info("received Get request", "peer", pr.Addr, "target", targetName)

	targets := []string{"*"}
	if targetName != "" && targetName != "*" {
		targets = strings.Split(targetName, ",")
	}
	// a request with a prefix and no path reads the prefix itself.
	paths := req.GetPath()
	if len(paths) == 0 {
		paths = []*gnmi.Path{{}}
	}

	response := &gnmi.GetResponse{
		Notification: make([]*gnmi.Notification, 0, len(paths)*len(targets)),
	}
	for _, p := range paths {
		fp := mergedGetPath(req.GetPrefix(), p)
		for _, tn := range targets {
			if err := ctx.Err(); err != nil {
				return nil, status.FromContextError(err).Err()
			}
			notifs, err := h.cache.Read("*", tn, fp)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "cache read failed: %v", err)
			}
			// deterministic order across subscription caches
			subs := make([]string, 0, len(notifs))
			for sub := range notifs {
				subs = append(subs, sub)
			}
			sort.Strings(subs)
			for _, sub := range subs {
				response.Notification = append(response.Notification, notifs[sub]...)
			}
		}
	}
	if h.cfg.Debug {
		h.logger.Debug("sending GetResponse", "peer", pr.Addr, "response", response)
	}
	return response, nil
}

// mergedGetPath joins the request prefix and one request path into the path
// used to read the cache. The origin comes from the path, or from the prefix
// when the path has none. A new Elem slice is allocated so the request is
// never mutated.
func mergedGetPath(prefix, p *gnmi.Path) *gnmi.Path {
	origin := p.GetOrigin()
	if origin == "" {
		origin = prefix.GetOrigin()
	}
	return &gnmi.Path{
		Origin: origin,
		Elem:   slices.Concat(prefix.GetElem(), p.GetElem()),
	}
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
				if upd.Path == nil {
					upd.Path = new(gnmi.Path)
				}
				if upd.Path.Target == "" {
					upd.Path.Target = name
				}
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
