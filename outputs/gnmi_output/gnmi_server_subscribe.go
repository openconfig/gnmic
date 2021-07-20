package gnmi_output

import (
	"fmt"
	"io"
	"strings"

	"github.com/openconfig/gnmi/coalesce"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
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
	peer, _ := peer.FromContext(stream.Context())
	s.l.Printf("received a subscribe request mode=%v from %q for target %q", sc.req.GetSubscribe().GetMode(), peer.Addr, sc.target)
	defer s.l.Printf("subscription from peer %q terminated", peer.Addr)

	sc.queue = coalesce.NewQueue()
	errChan := make(chan error, 3)
	sc.errChan = errChan

	s.l.Printf("acquiring subscription spot for target %q", sc.target)
	ok := s.subscribeRPCsem.TryAcquire(1)
	if !ok {
		return status.Errorf(codes.ResourceExhausted, "could not acquire a subscription spot")
	}
	s.l.Printf("acquired subscription spot for target %q", sc.target)

	switch sc.req.GetSubscribe().GetMode() {
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
	default:
		return status.Errorf(codes.InvalidArgument, "unrecognized subscription mode: %v", sc.req.GetSubscribe().GetMode())
	}
	// send all nodes added to queue
	go s.sendStreamingResults(sc)

	var errs = make([]error, 0)
	for err := range errChan {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		sb := strings.Builder{}
		sb.WriteString("multiple errors occurred:\n")
		for _, err := range errs {
			sb.WriteString(fmt.Sprintf("- %v\n", err))
		}
		return fmt.Errorf("%v", sb)
	}
	return nil
}
