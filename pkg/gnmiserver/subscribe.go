// SPDX-License-Identifier: Apache-2.0

package gnmiserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"sync"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	"github.com/openconfig/gnmic/pkg/cache"
	"github.com/openconfig/gnmic/pkg/logging"
)

type streamClient struct {
	target string
	req    *gnmi.SubscribeRequest

	stream  gnmi.GNMI_SubscribeServer
	errChan chan<- error
}

// Subscribe handles a gNMI Subscribe RPC, serving it from the cache.
func (h *Handlers) Subscribe(req *gnmi.SubscribeRequest, stream gnmi.GNMI_SubscribeServer) error {
	if h.cache == nil {
		return status.Errorf(codes.Unimplemented, "no cache configured, cannot serve Subscribe RPCs")
	}
	pr, _ := peer.FromContext(stream.Context())
	sc := &streamClient{
		stream: stream,
		req:    req,
	}
	sc.target = sc.req.GetSubscribe().GetPrefix().GetTarget()
	if sc.target == "" {
		sc.target = "*"
		sub := sc.req.GetSubscribe()
		if sub.GetPrefix() == nil {
			sub.Prefix = &gnmi.Path{Target: "*"}
		} else {
			sub.Prefix.Target = "*"
		}
	}

	h.logger.Info("received subscribe request",
		"mode", sc.req.GetSubscribe().GetMode(),
		"peer", pr.Addr,
		"target", sc.target,
	)
	defer func() {
		h.logger.Info("subscription from peer terminated", "peer", pr.Addr)
	}()

	// closing of this channel is handled by respective goroutines that are going to send error on this channel
	errChan := make(chan error, len(sc.req.GetSubscribe().GetSubscription()))
	sc.errChan = errChan // send-only

	switch sc.req.GetSubscribe().GetMode() {
	case gnmi.SubscriptionList_ONCE:
		go func() {
			h.handleONCESubscriptionRequest(sc)
			errChan <- sc.stream.Send(&gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_SyncResponse{SyncResponse: true},
			})
			close(errChan)
		}()
	case gnmi.SubscriptionList_POLL:
		go h.handlePolledSubscription(sc)
	case gnmi.SubscriptionList_STREAM:
		go h.handleStreamSubscriptionRequest(sc)
	default:
		return status.Errorf(codes.InvalidArgument, "unrecognized subscription mode: %v", sc.req.GetSubscribe().GetMode())
	}

	// flushing the errChan
	defer func() {
		h.logger.Info("flushing subscription errChan")
		for range errChan {
		}
	}()

	// returning first non-nil error and flushing rest in defer
	for err := range errChan {
		if err != nil {
			return status.Errorf(codes.Internal, "%v", err)
		}
	}

	return nil
}

func (h *Handlers) handleONCESubscriptionRequest(sc *streamClient) {
	var err error
	h.logger.Info("processing subscription to target", "target", sc.target)
	paths := make([]*gnmi.Path, 0)

	switch req := sc.req.GetRequest().(type) {
	case *gnmi.SubscribeRequest_Subscribe:
		pr := req.Subscribe.GetPrefix()
		for _, sub := range req.Subscribe.GetSubscription() {
			paths = append(paths, mergedPath(pr, sub))
		}
	}
	//
	ro := &cache.ReadOpts{
		Target:      sc.target,
		Paths:       paths,
		Mode:        "once",
		UpdatesOnly: sc.req.GetSubscribe().GetUpdatesOnly(),
	}

	defer func() {
		if err != nil {
			logging.LogErrUnlessCanceled(h.logger, err, "error processing subscription to target", "target", sc.target)
			sc.errChan <- err
			return
		}
		h.logger.Info("subscription request to target processed", "target", sc.target)
	}()

	for n := range h.cache.Subscribe(sc.stream.Context(), ro) {
		if n.Err != nil {
			err = n.Err
			return
		}
		err = sc.stream.Send(&gnmi.SubscribeResponse{
			Response: &gnmi.SubscribeResponse_Update{
				Update: n.Notification,
			},
		})
		if err != nil {
			return
		}
	}
}

func (h *Handlers) handleStreamSubscriptionRequest(sc *streamClient) {
	pr, _ := peer.FromContext(sc.stream.Context())

	errChan := make(chan error)
	defer close(errChan)

	// this context is required to signal this goroutine and `handleSampledQuery` goroutine that error has happened in cache
	ctx, cancel := context.WithCancel(sc.stream.Context())
	h.logger.Info("processing STREAM subscription", "peer", pr.Addr, "target", sc.target)

	go func() {
		defer close(sc.errChan)

		for err := range errChan {
			if err == nil {
				h.logger.Info("subscription request processed", "peer", pr.Addr, "target", sc.target)
			} else if errors.Is(err, context.Canceled) {
				h.logger.Info("subscription to target canceled", "target", sc.target)
				sc.errChan <- err
				cancel()
			} else {
				logging.LogErrUnlessCanceled(h.logger, err, "error processing STREAM subscription to target", "target", sc.target)
				sc.errChan <- err
				cancel()
			}
		}
	}()

	var pr2 *gnmi.Path
	switch req := sc.req.GetRequest().(type) {
	case *gnmi.SubscribeRequest_Subscribe:
		pr2 = req.Subscribe.GetPrefix()
	}

	subs := sc.req.GetSubscribe().GetSubscription()
	updatesOnly := sc.req.GetSubscribe().GetUpdatesOnly()

	// build the cache read options for each subscription list item.
	ros := make([]*cache.ReadOpts, 0, len(subs))
	for _, sub := range subs {
		ro := h.streamReadOpts(sc.target, pr2, sub)
		ro.UpdatesOnly = updatesOnly
		ros = append(ros, ro)
	}

	// initial sync phase: unless updates_only is set, send the
	// current state of the matching paths, then a sync_response.
	// When updates_only is set, only the sync_response is sent.
	if !updatesOnly {
		for _, ro := range ros {
			streamMode := ro.Mode
			ro.Mode = cache.ReadMode_Once
			for n := range h.cache.Subscribe(ctx, ro) {
				if n.Err != nil {
					errChan <- n.Err
					return
				}
				err := sc.stream.Send(&gnmi.SubscribeResponse{
					Response: &gnmi.SubscribeResponse_Update{
						Update: n.Notification,
					},
				})
				if err != nil {
					errChan <- err
					return
				}
			}
			// restore the streaming mode and skip the cache's own
			// initial read, the state was just sent above.
			ro.Mode = streamMode
			ro.UpdatesOnly = true
		}
	}
	err := sc.stream.Send(&gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_SyncResponse{SyncResponse: true},
	})
	if err != nil {
		errChan <- err
		return
	}

	wg := new(sync.WaitGroup)
	wg.Add(len(ros))

	for i, ro := range ros {
		h.logger.Info("handling subscription list item", "index", i, "target", sc.target, "opts", ro)

		go func(ro *cache.ReadOpts) {
			defer wg.Done()

			for n := range h.cache.Subscribe(ctx, ro) {
				// `errChan <- n.Err` should trigger the gnmi-server side cleanup
				// only wait would be for the cache to close the channel
				if n.Err != nil {
					errChan <- n.Err
					logging.LogErrUnlessCanceled(h.logger, n.Err, "cache subscribe failed", "opts", ro)

					// reader should only stop once the channel is closed by sender or otherwise
					// it could block the senders who don't know that error has happened
					continue
				}

				err := sc.stream.Send(&gnmi.SubscribeResponse{
					Response: &gnmi.SubscribeResponse_Update{
						Update: n.Notification,
					},
				})

				if err != nil {
					errChan <- err
				}
			}
		}(ro)
	}

	// wait for ctx to be done
	<-ctx.Done()
	errChan <- ctx.Err()
	wg.Wait()
}

// streamReadOpts builds the cache read options for one STREAM subscription
// list item, applying the configured sample and heartbeat interval bounds.
// Unknown subscription modes are treated as TARGET_DEFINED (on-change).
func (h *Handlers) streamReadOpts(target string, prefix *gnmi.Path, sub *gnmi.Subscription) *cache.ReadOpts {
	paths := []*gnmi.Path{mergedPath(prefix, sub)}
	heartbeat := time.Duration(sub.GetHeartbeatInterval())
	if heartbeat > 0 && heartbeat < h.cfg.MinHeartbeatInterval {
		heartbeat = h.cfg.MinHeartbeatInterval
	}

	switch sub.GetMode() {
	case gnmi.SubscriptionMode_SAMPLE:
		period := time.Duration(sub.GetSampleInterval())
		if period == 0 {
			period = h.cfg.DefaultSampleInterval
		} else if period < h.cfg.MinSampleInterval {
			period = h.cfg.MinSampleInterval
		}
		return &cache.ReadOpts{
			Target:            target,
			Paths:             paths,
			Mode:              cache.ReadMode_StreamSample,
			SampleInterval:    period,
			HeartbeatInterval: heartbeat,
			SuppressRedundant: sub.GetSuppressRedundant(),
		}
	default: // ON_CHANGE, TARGET_DEFINED
		return &cache.ReadOpts{
			Target:            target,
			Paths:             paths,
			Mode:              cache.ReadMode_StreamOnChange,
			HeartbeatInterval: heartbeat,
		}
	}
}

// mergedPath joins the subscription list prefix with one subscription path.
// The result is a new Elem slice so the request prefix is not mutated.
func mergedPath(prefix *gnmi.Path, sub *gnmi.Subscription) *gnmi.Path {
	return &gnmi.Path{
		Origin: prefix.GetOrigin(),
		Target: prefix.GetTarget(),
		Elem:   slices.Concat(prefix.GetElem(), sub.GetPath().GetElem()),
	}
}

func (h *Handlers) handlePolledSubscription(sc *streamClient) {
	defer close(sc.errChan)
	h.handleONCESubscriptionRequest(sc)
	sc.errChan <- sc.stream.Send(&gnmi.SubscribeResponse{Response: &gnmi.SubscribeResponse_SyncResponse{
		SyncResponse: true,
	}})
	for {
		req, err := sc.stream.Recv()
		if errors.Is(err, io.EOF) {
			sc.errChan <- err
			return
		}
		if err != nil {
			logging.LogErrUnlessCanceled(h.logger, err, "failed poll subscription receive", "target", sc.target)
			sc.errChan <- err
			return
		}
		switch req := req.GetRequest().(type) {
		case *gnmi.SubscribeRequest_Poll:
		default:
			err = fmt.Errorf("unexpected request type: expecting a Poll request, rcvd: %v", req)
			logging.LogErrUnlessCanceled(h.logger, err, "unexpected poll subscription request")
			sc.errChan <- err
			return
		}
		h.logger.Info("repoll", "target", sc.target)
		h.handleONCESubscriptionRequest(sc)
		sc.errChan <- sc.stream.Send(&gnmi.SubscribeResponse{Response: &gnmi.SubscribeResponse_SyncResponse{
			SyncResponse: true,
		}})
		h.logger.Info("repoll done", "target", sc.target)
	}
}
