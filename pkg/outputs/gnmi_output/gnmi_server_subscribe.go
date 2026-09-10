// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package gnmi_output

import (
	"errors"
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openconfig/gnmi/coalesce"
	"github.com/openconfig/gnmi/proto/gnmi"
)

func (s *server) Subscribe(stream gnmi.GNMI_SubscribeServer) error {
	sc := &streamClient{
		stream: stream,
	}
	var err error
	sc.req, err = stream.Recv()
	switch {
	case err == io.EOF:
		return nil
	case err != nil:
		return err
	case sc.req.GetSubscribe() == nil:
		return status.Errorf(codes.InvalidArgument, "the subscribe request must contain a subscription definition")
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
	if !s.c.HasTarget(sc.target) {
		return status.Errorf(codes.NotFound, "target %q not found", sc.target)
	}
	peerAddr := peerAddr(stream.Context())
	s.l.Info("received subscribe request", "mode", sc.req.GetSubscribe().GetMode(), "peer", peerAddr, "target", sc.target)
	defer s.l.Info("subscription terminated", "peer", peerAddr)

	// validate the request before acquiring a subscription spot:
	// the spot is only released by sendStreamingResults, which must
	// then be started on every path past the acquisition.
	mode := sc.req.GetSubscribe().GetMode()
	switch mode {
	case gnmi.SubscriptionList_ONCE, gnmi.SubscriptionList_POLL, gnmi.SubscriptionList_STREAM:
	default:
		return status.Errorf(codes.InvalidArgument, "unrecognized subscription mode: %v", mode)
	}

	sc.queue = coalesce.NewQueue()
	errChan := make(chan error, 3)
	sc.errChan = errChan

	s.l.Debug("acquiring subscription spot", "target", sc.target)
	ok := s.subscribeRPCsem.TryAcquire(1)
	if !ok {
		return status.Errorf(codes.ResourceExhausted, "could not acquire a subscription spot")
	}
	s.l.Debug("acquired subscription spot", "target", sc.target)

	switch mode {
	case gnmi.SubscriptionList_ONCE:
		go func() {
			s.handleSubscriptionRequest(sc)
			sc.queue.Close()
		}()
	case gnmi.SubscriptionList_POLL:
		go s.handlePolledSubscription(sc)
	case gnmi.SubscriptionList_STREAM:
		if sc.req.GetSubscribe().GetUpdatesOnly() {
			sc.queue.Insert(syncMarker{})
		}
		remove := addSubscription(s.m, sc.req.GetSubscribe(), &matchClient{queue: sc.queue})
		defer remove()
		if !sc.req.GetSubscribe().GetUpdatesOnly() {
			go s.handleSubscriptionRequest(sc)
		}
	}
	// send all nodes added to queue.
	// sendStreamingResults owns the subscription spot and the stream:
	// the RPC is over once it returns, whether because the queue was
	// closed (ONCE), the client went away or a send failed.
	s.sendStreamingResults(sc)

	// collect the errors reported by the subscription goroutines.
	// errChan is buffered for one error per goroutine so none of them
	// blocks on send, and no error is reported after this point.
	var errs []error
drain:
	for {
		select {
		case err := <-errChan:
			if err != nil {
				errs = append(errs, err)
			}
		default:
			break drain
		}
	}
	return errors.Join(errs...)
}
