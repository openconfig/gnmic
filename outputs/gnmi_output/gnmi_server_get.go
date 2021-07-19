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

func (s *server) Get(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	ok := s.unaryRPCsem.TryAcquire(1)
	if !ok {
		return nil, status.Errorf(codes.ResourceExhausted, "max number of Get RPC reached")
	}
	defer s.unaryRPCsem.Release(1)

	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(req.GetPath()) == 1 && req.GetPath()[0].GetOrigin() == "gnmic" {
		if req.GetPath()[0].Elem[0].Name == "target" {
			notifs := make([]*gnmi.Notification, 0, len(s.targets))
			for _, tc := range s.targets {
				notifs = append(notifs, targetConfigToNotification(tc))
			}
			return &gnmi.GetResponse{Notification: notifs}, nil
		}
	}

	targets := make(map[string]*collector.TargetConfig)
	target := req.GetPrefix().GetTarget()
	peer, _ := peer.FromContext(ctx)
	s.l.Printf("received Get request from %q to target %q", peer.Addr, target)

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
	results := make(chan *gnmi.Notification)
	errChan := make(chan error, numTargets)

	response := &gnmi.GetResponse{
		Notification: make([]*gnmi.Notification, 0, numTargets), // assume a single notification per target
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
	for name, tc := range targets {
		go func(name string, tc *collector.TargetConfig) {
			defer wg.Done()
			t := collector.NewTarget(tc)
			defer t.Stop()
			ctx, cancel := context.WithTimeout(ctx, tc.Timeout)
			defer cancel()
			err := t.CreateGNMIClient(ctx)
			if err != nil {
				s.l.Printf("target %q err: %v", name, err)
				errChan <- fmt.Errorf("target %q err: %v", name, err)
				return
			}
			creq := proto.Clone(req).(*gnmi.GetRequest)
			if creq.GetPrefix() == nil {
				creq.Prefix = new(gnmi.Path)
			}
			if creq.GetPrefix().GetTarget() == "" || creq.GetPrefix().GetTarget() == "*" {
				creq.Prefix.Target = name
			}
			res, err := t.Get(ctx, creq)
			if err != nil {
				s.l.Printf("target %q err: %v", name, err)
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
	s.l.Printf("sending GetResponse to %q: %+v", peer.Addr, response)
	return response, nil
}

func targetConfigToNotification(tc *collector.TargetConfig) *gnmi.Notification {
	n := &gnmi.Notification{
		Timestamp: time.Now().UnixNano(),
		Prefix: &gnmi.Path{
			Origin: "gnmic",
			Elem: []*gnmi.PathElem{
				{
					Name: "target",
					Key:  map[string]string{"name": tc.Name},
				},
			},
		},
		Update: []*gnmi.Update{
			{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "address"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_AsciiVal{AsciiVal: tc.Address},
				},
			},
		},
	}
	if tc.Username != nil {
		n.Update = append(n.Update, &gnmi.Update{
			Path: &gnmi.Path{
				Elem: []*gnmi.PathElem{
					{Name: "username"},
				},
			},
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_AsciiVal{AsciiVal: *tc.Username},
			},
		})
	}
	if tc.Insecure != nil {
		n.Update = append(n.Update, &gnmi.Update{
			Path: &gnmi.Path{
				Elem: []*gnmi.PathElem{
					{Name: "insecure"},
				},
			},
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_AsciiVal{AsciiVal: fmt.Sprint(*tc.Insecure)},
			},
		})
	}
	if tc.SkipVerify != nil {
		n.Update = append(n.Update, &gnmi.Update{
			Path: &gnmi.Path{
				Elem: []*gnmi.PathElem{
					{Name: "skip-verify"},
				},
			},
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_AsciiVal{AsciiVal: fmt.Sprint(*tc.SkipVerify)},
			},
		})
	}
	n.Update = append(n.Update, &gnmi.Update{
		Path: &gnmi.Path{
			Elem: []*gnmi.PathElem{
				{Name: "timeout"},
			},
		},
		Val: &gnmi.TypedValue{
			Value: &gnmi.TypedValue_AsciiVal{AsciiVal: tc.Timeout.String()},
		},
	})
	if tc.TLSCA != nil {
		n.Update = append(n.Update, &gnmi.Update{
			Path: &gnmi.Path{
				Elem: []*gnmi.PathElem{
					{Name: "tls-ca"},
				},
			},
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_AsciiVal{AsciiVal: tc.TLSCAString()},
			},
		})
	}
	if tc.TLSCert != nil {
		n.Update = append(n.Update, &gnmi.Update{
			Path: &gnmi.Path{
				Elem: []*gnmi.PathElem{
					{Name: "tls-cert"},
				},
			},
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_AsciiVal{AsciiVal: tc.TLSCertString()},
			},
		})
	}
	if tc.TLSKey != nil && tc.TLSKeyString() != "NA" {
		n.Update = append(n.Update, &gnmi.Update{
			Path: &gnmi.Path{
				Elem: []*gnmi.PathElem{
					{Name: "tls-key"},
				},
			},
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_AsciiVal{AsciiVal: tc.TLSKeyString()},
			},
		})
	}
	if len(tc.Outputs) > 0 {
		typedVals := make([]*gnmi.TypedValue, 0, len(tc.Subscriptions))
		for _, out := range tc.Outputs {
			typedVals = append(typedVals, &gnmi.TypedValue{
				Value: &gnmi.TypedValue_AsciiVal{AsciiVal: out},
			})
		}
		n.Update = append(n.Update, &gnmi.Update{
			Path: &gnmi.Path{
				Elem: []*gnmi.PathElem{
					{Name: "outputs"},
				},
			},
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_LeaflistVal{
					LeaflistVal: &gnmi.ScalarArray{
						Element: typedVals,
					},
				},
			},
		})
	}
	if len(tc.Subscriptions) > 0 {
		typedVals := make([]*gnmi.TypedValue, 0, len(tc.Subscriptions))
		for _, sub := range tc.Subscriptions {
			typedVals = append(typedVals, &gnmi.TypedValue{
				Value: &gnmi.TypedValue_AsciiVal{AsciiVal: sub},
			})
		}
		n.Update = append(n.Update, &gnmi.Update{
			Path: &gnmi.Path{
				Elem: []*gnmi.PathElem{
					{Name: "subscriptions"},
				},
			},
			Val: &gnmi.TypedValue{
				Value: &gnmi.TypedValue_LeaflistVal{
					LeaflistVal: &gnmi.ScalarArray{
						Element: typedVals,
					},
				},
			},
		})
	}
	return n
}
