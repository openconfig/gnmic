/*
Copyright 2017 Google Inc.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// This gNMI server implementation is based on the one found here:
// https://github.com/openconfig/gnmi/blob/c69a5df04b5329d70e3e76afa773669527cfad9b/subscribe/subscribe.go

package gnmi_output

import (
	"errors"
	"io"
	"log"
	"sync"

	"github.com/karimra/gnmic/types"
	"github.com/openconfig/gnmi/cache"
	"github.com/openconfig/gnmi/coalesce"
	"github.com/openconfig/gnmi/ctree"
	"github.com/openconfig/gnmi/match"
	"github.com/openconfig/gnmi/path"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/subscribe"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type streamClient struct {
	target  string
	req     *gnmi.SubscribeRequest
	queue   *coalesce.Queue
	stream  gnmi.GNMI_SubscribeServer
	errChan chan<- error
}

type server struct {
	gnmi.UnimplementedGNMIServer
	//
	l               *log.Logger
	c               *cache.Cache
	m               *match.Match
	subscribeRPCsem *semaphore.Weighted
	unaryRPCsem     *semaphore.Weighted
	//
	mu      *sync.RWMutex
	targets map[string]*types.TargetConfig
}

type matchClient struct {
	queue *coalesce.Queue
	err   error
}

type syncMarker struct{}

type resp struct {
	stream gnmi.GNMI_SubscribeServer
	n      *ctree.Leaf
	dup    uint32
}

func (m *matchClient) Update(n interface{}) {
	if m.err != nil {
		return
	}
	_, m.err = m.queue.Insert(n)
}

func (g *gNMIOutput) newServer() *server {
	return &server{
		l:       g.logger,
		c:       g.c,
		m:       match.New(),
		mu:      new(sync.RWMutex),
		targets: make(map[string]*types.TargetConfig),
	}
}

func (s *server) Update(n *ctree.Leaf) {
	switch v := n.Value().(type) {
	case *gnmi.Notification:
		subscribe.UpdateNotification(s.m, n, v, path.ToStrings(v.Prefix, true))
	default:
		s.l.Printf("unexpected update type: %T", v)
	}
}

func addSubscription(m *match.Match, s *gnmi.SubscriptionList, c *matchClient) func() {
	removes := make([]func(), 0, len(s.GetSubscription()))
	prefix := path.ToStrings(s.GetPrefix(), true)
	for _, p := range s.GetSubscription() {
		if p.GetPath() == nil {
			continue
		}

		path := append(prefix, path.ToStrings(p.GetPath(), false)...)
		removes = append(removes, m.AddQuery(path, c))
	}
	return func() {
		for _, remove := range removes {
			remove()
		}
	}
}

func (s *server) handleSubscriptionRequest(sc *streamClient) {
	var err error
	s.l.Printf("processing subscription to target %q", sc.target)
	defer func() {
		if err != nil {
			s.l.Printf("error processing subscription to target %q: %v", sc.target, err)
			sc.queue.Close()
			sc.errChan <- err
			return
		}
		s.l.Printf("subscription request to target %q processed", sc.target)
	}()

	if !sc.req.GetSubscribe().GetUpdatesOnly() {
		for _, sub := range sc.req.GetSubscribe().GetSubscription() {
			var fp []string
			fp, err = path.CompletePath(sc.req.GetSubscribe().GetPrefix(), sub.GetPath())
			if err != nil {
				return
			}
			err = s.c.Query(sc.target, fp,
				func(_ []string, l *ctree.Leaf, _ interface{}) error {
					if err != nil {
						return err
					}
					_, err = sc.queue.Insert(l)
					return nil
				})
			if err != nil {
				s.l.Printf("target %q failed internal cache query: %v", sc.target, err)
				return
			}
		}
	}
	_, err = sc.queue.Insert(syncMarker{})
}

func (s *server) sendStreamingResults(sc *streamClient) {
	ctx := sc.stream.Context()
	peer, _ := peer.FromContext(ctx)
	s.l.Printf("sending streaming results from target %q to peer %q", sc.target, peer.Addr)
	defer s.subscribeRPCsem.Release(1)
	for {
		item, dup, err := sc.queue.Next(ctx)
		if coalesce.IsClosedQueue(err) {
			sc.errChan <- nil
			return
		}
		if err != nil {
			sc.errChan <- err
			return
		}
		if _, ok := item.(syncMarker); ok {
			err = sc.stream.Send(&gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_SyncResponse{
					SyncResponse: true,
				}})
			if err != nil {
				sc.errChan <- err
				return
			}
			continue
		}

		node, ok := item.(*ctree.Leaf)
		if !ok || node == nil {
			sc.errChan <- status.Errorf(codes.Internal, "invalid cache node: %+v", item)
			return
		}
		err = s.sendSubscribeResponse(&resp{
			stream: sc.stream,
			n:      node,
			dup:    dup,
		}, sc)
		if err != nil {
			s.l.Printf("target %q: failed sending subscribeResponse: %v", sc.target, err)
			sc.errChan <- err
			return
		}
		// TODO: check if target was deleted ? necessary ?
	}
}

func (s *server) handlePolledSubscription(sc *streamClient) {
	s.handleSubscriptionRequest(sc)
	var err error
	for {
		if sc.queue.IsClosed() {
			return
		}
		_, err = sc.stream.Recv()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			s.l.Printf("target %q: failed poll subscription rcv: %v", sc.target, err)
			sc.errChan <- err
			return
		}
		s.l.Printf("target %q: repoll", sc.target)
		s.handleSubscriptionRequest(sc)
		s.l.Printf("target %q: repoll done", sc.target)
	}
}

func (s *server) sendSubscribeResponse(r *resp, sc *streamClient) error {
	notif, err := subscribe.MakeSubscribeResponse(r.n.Value(), r.dup)
	if err != nil {
		return status.Errorf(codes.Unknown, "unknown error: %v", err)
	}
	// No acls
	return r.stream.Send(notif)
}
