package gnmi_output

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/outputs"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func (s *server) Set(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	ok := s.unaryRPCsem.TryAcquire(1)
	if !ok {
		return nil, status.Errorf(codes.ResourceExhausted, "max number of Unary RPC reached")
	}
	defer s.unaryRPCsem.Release(1)

	if len(req.GetUpdate())+len(req.GetReplace())+len(req.GetDelete()) == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "missing update/replace/delete path(s)")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	targets := make(map[string]*collector.TargetConfig)
	target := req.GetPrefix().GetTarget()
	peer, _ := peer.FromContext(ctx)
	s.l.Printf("received Set request from %q to target %q", peer.Addr, target)

	if target == "" || target == "*" {
		targets = s.targets
	} else {
		for n, tc := range s.targets {
			if outputs.GetHost(n) == target {
				targets[target] = tc
				break
			}
		}
	}

	numTargets := len(targets)
	if numTargets == 0 {
		return nil, status.Errorf(codes.NotFound, "unknown target %q", target)
	}
	results := make(chan *gnmi.UpdateResult)
	errChan := make(chan error, numTargets)

	response := &gnmi.SetResponse{
		Response: make([]*gnmi.UpdateResult, 0, numTargets), // assume a single update per target
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
	for name, tc := range targets {
		go func(name string, tc *collector.TargetConfig) {
			name = outputs.GetHost(name)
			defer wg.Done()
			t := collector.NewTarget(tc)
			err := t.CreateGNMIClient(ctx)
			if err != nil {
				s.l.Printf("target %q err: %v", name, err)
				errChan <- fmt.Errorf("target %q err: %v", name, err)
				return
			}
			creq := proto.Clone(req).(*gnmi.SetRequest)
			if creq.GetPrefix() == nil {
				creq.Prefix = new(gnmi.Path)
			}
			if creq.GetPrefix().GetTarget() == "" || creq.GetPrefix().GetTarget() == "*" {
				creq.Prefix.Target = name
			}
			res, err := t.Set(ctx, creq)
			if err != nil {
				s.l.Printf("target %q err: %v", name, err)
				errChan <- fmt.Errorf("target %q err: %v", name, err)
				return
			}
			for _, upd := range res.GetResponse() {
				upd.Path.Origin = name
				results <- upd
			}
		}(name, tc)
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
	s.l.Printf("sending SetResponse to %q: %+v", peer.Addr, response)
	return response, nil
}
