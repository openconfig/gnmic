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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/path"
	"github.com/openconfig/gnmic/pkg/api/server"
	"github.com/openconfig/gnmic/pkg/api/target"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/cache"
)

type streamClient struct {
	target string
	req    *gnmi.SubscribeRequest

	stream  gnmi.GNMI_SubscribeServer
	errChan chan<- error
}

func (a *App) startGnmiServer() error {
	if a.Config.GnmiServer == nil {
		a.c = nil
		return nil
	}

	var err error
	a.c, err = cache.New(a.Config.GnmiServer.Cache, cache.WithLogger(a.Logger))
	if err != nil {
		a.Logger.Printf("failed to initialize gNMI cache: %v", err)
		return err
	}

	s, err := server.New(server.Config{
		Address:              a.Config.GnmiServer.Address,
		MaxUnaryRPC:          a.Config.GnmiServer.MaxUnaryRPC,
		MaxStreamingRPC:      a.Config.GnmiServer.MaxSubscriptions,
		MaxRecvMsgSize:       a.Config.GnmiServer.MaxRecvMsgSize,
		MaxSendMsgSize:       a.Config.GnmiServer.MaxSendMsgSize,
		MaxConcurrentStreams: a.Config.GnmiServer.MaxConcurrentStreams,
		TCPKeepalive:         a.Config.GnmiServer.TCPKeepalive,
		Keepalive:            a.Config.GnmiServer.GRPCKeepalive.Convert(),
		RateLimit:            a.Config.GnmiServer.RateLimit,
		HealthEnabled:        true,
		TLS:                  a.Config.GnmiServer.TLS,
	}, server.WithLogger(a.Logger),
		server.WithGetHandler(a.serverGetHandler),
		server.WithSetHandler(a.serverSetHandler),
		server.WithSubscribeHandler(a.serverSubscribeHandler),
		server.WithRegistry(a.reg),
	)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(a.ctx)

	go a.registerGNMIServer(ctx)
	go func() {
		defer cancel()
		err := s.Start(ctx)
		if err != nil {
			a.Logger.Print(err)
		}
	}()
	return nil
}

func (a *App) registerGNMIServer(ctx context.Context, defaultTags ...string) {
	if a.Config.GnmiServer.ServiceRegistration == nil {
		return
	}
	var err error
	clientConfig := &api.Config{
		Address:    a.Config.GnmiServer.ServiceRegistration.Address,
		Scheme:     "http",
		Datacenter: a.Config.GnmiServer.ServiceRegistration.Datacenter,
		Token:      a.Config.GnmiServer.ServiceRegistration.Token,
	}
	if a.Config.GnmiServer.ServiceRegistration.Username != "" && a.Config.GnmiServer.ServiceRegistration.Password != "" {
		clientConfig.HttpAuth = &api.HttpBasicAuth{
			Username: a.Config.GnmiServer.ServiceRegistration.Username,
			Password: a.Config.GnmiServer.ServiceRegistration.Password,
		}
	}
INITCONSUL:
	consulClient, err := api.NewClient(clientConfig)
	if err != nil {
		a.Logger.Printf("failed to connect to consul: %v", err)
		time.Sleep(1 * time.Second)
		goto INITCONSUL
	}
	self, err := consulClient.Agent().Self()
	if err != nil {
		a.Logger.Printf("failed to connect to consul: %v", err)
		time.Sleep(1 * time.Second)
		goto INITCONSUL
	}
	if cfg, ok := self["Config"]; ok {
		b, _ := json.Marshal(cfg)
		a.Logger.Printf("consul agent config: %s", string(b))
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	h, p, err := net.SplitHostPort(a.Config.GnmiServer.Address)
	if err != nil {
		a.Logger.Printf("failed to split host and port from gNMI server address %q: %v", a.Config.GnmiServer.Address, err)
		return
	}
	pi, _ := strconv.Atoi(p)
	service := &api.AgentServiceRegistration{
		ID:      a.Config.InstanceName,
		Name:    a.Config.GnmiServer.ServiceRegistration.Name,
		Address: h,
		Port:    pi,
		Tags:    append(defaultTags, a.Config.GnmiServer.ServiceRegistration.Tags...),
		Checks: api.AgentServiceChecks{
			{
				TTL:                            a.Config.GnmiServer.ServiceRegistration.CheckInterval.String(),
				DeregisterCriticalServiceAfter: a.Config.GnmiServer.ServiceRegistration.DeregisterAfter,
			},
		},
	}
	if a.Config.Clustering != nil {
		if a.Config.Clustering.InstanceName != "" {
			service.ID = a.Config.Clustering.InstanceName
		}
		service.Name = a.Config.Clustering.ClusterName + "-gnmi-server"
		if service.Tags == nil {
			service.Tags = make([]string, 0)
		}
		service.Tags = append(service.Tags, fmt.Sprintf("cluster-name=%s", a.Config.Clustering.ClusterName))
	}
	if service.ID == "" {
		service.ID = service.Name
	}
	service.Tags = append(service.Tags, fmt.Sprintf("instance-name=%s", service.ID))
	ttlCheckID := "service:" + service.ID
	b, _ := json.Marshal(service)
	a.Logger.Printf("registering service: %s", string(b))
	err = consulClient.Agent().ServiceRegister(service)
	if err != nil {
		a.Logger.Printf("failed to register service in consul: %v", err)
		return
	}

	err = consulClient.Agent().UpdateTTL(ttlCheckID, "", api.HealthPassing)
	if err != nil {
		a.Logger.Printf("failed to pass TTL check: %v", err)
	}
	ticker := time.NewTicker(a.Config.GnmiServer.ServiceRegistration.CheckInterval / 2)
	for {
		select {
		case <-ticker.C:
			err = consulClient.Agent().UpdateTTL(ttlCheckID, "", api.HealthPassing)
			if err != nil {
				a.Logger.Printf("failed to pass TTL check: %v", err)
			}
		case <-ctx.Done():
			consulClient.Agent().UpdateTTL(ttlCheckID, ctx.Err().Error(), api.HealthCritical)
			ticker.Stop()
			goto INITCONSUL
		}
	}
}

func (a *App) handleONCESubscriptionRequest(sc *streamClient) {
	var err error
	a.Logger.Printf("processing subscription to target %q", sc.target)
	paths := make([]*gnmi.Path, 0)

	switch req := sc.req.GetRequest().(type) {
	case *gnmi.SubscribeRequest_Subscribe:
		pr := req.Subscribe.GetPrefix()
		for _, sub := range req.Subscribe.GetSubscription() {
			paths = append(paths,
				&gnmi.Path{
					Origin: pr.GetOrigin(),
					Target: pr.GetTarget(),
					Elem:   append(pr.GetElem(), sub.GetPath().GetElem()...),
				})
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
			a.Logger.Printf("error processing subscription to target %q: %v", sc.target, err)
			sc.errChan <- err
			return
		}
		a.Logger.Printf("subscription request to target %q processed", sc.target)
	}()

	for n := range a.c.Subscribe(sc.stream.Context(), ro) {
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

func (a *App) handleStreamSubscriptionRequest(sc *streamClient) {
	peer, _ := peer.FromContext(sc.stream.Context())

	errChan := make(chan error)
	defer close(errChan)

	// this context is required to signal this goroutine and `handleSampledQuery` goroutine that error has happened in cache
	ctx, cancel := context.WithCancel(sc.stream.Context())
	a.Logger.Printf("processing STREAM subscription from %q to target %q", peer.Addr, sc.target)

	go func() {
		defer close(sc.errChan)

		for err := range errChan {
			if err == nil {
				a.Logger.Printf("subscription request from %q to target %q processed", peer.Addr, sc.target)
			} else if errors.Is(err, context.Canceled) {
				a.Logger.Printf("subscription to target %q canceled", sc.target)
				sc.errChan <- err
				cancel()
			} else {
				a.Logger.Printf("error processing STREAM subscription to target %q: %v", sc.target, err)
				sc.errChan <- err
				cancel()
			}
		}
	}()

	if sc.req.GetSubscribe().GetUpdatesOnly() {
		err := sc.stream.Send(&gnmi.SubscribeResponse{
			Response: &gnmi.SubscribeResponse_SyncResponse{SyncResponse: true},
		})

		if err != nil {
			errChan <- err
			return
		}
	}
	var pr *gnmi.Path
	switch req := sc.req.GetRequest().(type) {
	case *gnmi.SubscribeRequest_Subscribe:
		pr = req.Subscribe.GetPrefix()
	}

	subs := sc.req.GetSubscribe().GetSubscription()
	wg := new(sync.WaitGroup)
	wg.Add(len(subs))

	for i, sub := range subs {
		a.Logger.Printf("handling subscriptionList item[%d]: target %q, %q", i, sc.target, sub.String())

		go func(sub *gnmi.Subscription) {
			defer wg.Done()
			var ro *cache.ReadOpts

			switch sub.GetMode() {
			case gnmi.SubscriptionMode_ON_CHANGE, gnmi.SubscriptionMode_TARGET_DEFINED:
				ro = &cache.ReadOpts{
					Target: sc.target,
					Paths: []*gnmi.Path{
						{
							Origin: pr.GetOrigin(),
							Target: pr.GetTarget(),
							Elem:   append(pr.GetElem(), sub.GetPath().GetElem()...),
						},
					},
					Mode:              cache.ReadMode_StreamOnChange,
					HeartbeatInterval: time.Duration(sub.GetHeartbeatInterval()),
					SuppressRedundant: sub.GetSuppressRedundant(),
					UpdatesOnly:       sc.req.GetSubscribe().GetUpdatesOnly(),
				}
			case gnmi.SubscriptionMode_SAMPLE:
				period := time.Duration(sub.GetSampleInterval())
				if period == 0 {
					period = a.Config.GnmiServer.DefaultSampleInterval
				} else if period < a.Config.GnmiServer.MinSampleInterval {
					period = a.Config.GnmiServer.MinSampleInterval
				}
				ro = &cache.ReadOpts{
					Target: sc.target,
					Paths: []*gnmi.Path{
						{
							Origin: pr.GetOrigin(),
							Target: pr.GetTarget(),
							Elem:   append(pr.GetElem(), sub.GetPath().GetElem()...),
						}},
					Mode:              cache.ReadMode_StreamSample,
					SampleInterval:    period,
					HeartbeatInterval: time.Duration(sub.GetHeartbeatInterval()),
					SuppressRedundant: sub.GetSuppressRedundant(),
					UpdatesOnly:       sc.req.GetSubscribe().GetUpdatesOnly(),
				}
			}

			a.Logger.Printf("cache subscribe: %+v", ro)

			for n := range a.c.Subscribe(ctx, ro) {
				// `errChan <- n.Err` should trigger the gnmi-server side cleanup
				// only wait would be for the cache to close the channel
				if n.Err != nil {
					errChan <- n.Err
					a.Logger.Printf("cache subscribe failed: %+v: %v", ro, n.Err)

					// reader should only stop once the channel is closed by sender or otherwise
					// it coould block the senders who doesn't know that error has happened

					continue
				}

				err := sc.stream.Send(&gnmi.SubscribeResponse{
					Response: &gnmi.SubscribeResponse_Update{
						Update: n.Notification,
					},
				})

				if err != nil {
					errChan <- n.Err
				}
			}
		}(sub)
	}

	// wait for ctx to be done
	<-ctx.Done()
	errChan <- ctx.Err()
	wg.Wait()
}

func (a *App) handlePolledSubscription(sc *streamClient) {
	defer close(sc.errChan)
	a.handleONCESubscriptionRequest(sc)
	sc.errChan <- sc.stream.Send(&gnmi.SubscribeResponse{Response: &gnmi.SubscribeResponse_SyncResponse{
		SyncResponse: true,
	}})
	// var err error
	for {
		req, err := sc.stream.Recv()
		if errors.Is(err, io.EOF) {
			sc.errChan <- err
			return
		}
		switch req := req.Request.(type) {
		case *gnmi.SubscribeRequest_Poll:
		default:
			err = fmt.Errorf("unexpected request type: expecting a Poll request, rcvd: %v", req)
			a.Logger.Print(err)
			sc.errChan <- err
			return
		}
		if err != nil {
			a.Logger.Printf("target %q: failed poll subscription rcv: %v", sc.target, err)
			sc.errChan <- err
			return
		}
		a.Logger.Printf("target %q: repoll", sc.target)
		a.handleONCESubscriptionRequest(sc)
		sc.errChan <- sc.stream.Send(&gnmi.SubscribeResponse{Response: &gnmi.SubscribeResponse_SyncResponse{
			SyncResponse: true,
		}})
		a.Logger.Printf("target %q: repoll done", sc.target)
	}
}

////

func (a *App) handlegNMIcInternalGet(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	notifications := make([]*gnmi.Notification, 0, len(req.GetPath()))
	a.configLock.RLock()
	defer a.configLock.RUnlock()

	for _, p := range req.GetPath() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			elems := path.PathElems(req.GetPrefix(), p)
			ns, err := a.handlegNMIGetPath(elems, req.GetEncoding())
			if err != nil {
				return nil, err
			}
			notifications = append(notifications, ns...)
		}
	}
	return &gnmi.GetResponse{Notification: notifications}, nil
}

func (a *App) handlegNMIGetPath(elems []*gnmi.PathElem, enc gnmi.Encoding) ([]*gnmi.Notification, error) {
	notifications := make([]*gnmi.Notification, 0, len(elems))
	for _, e := range elems {
		switch e.Name {
		// case "":
		case "targets":
			if e.Key != nil {
				if _, ok := e.Key["name"]; ok {
					for _, tc := range a.Config.Targets {
						if tc.Name == e.Key["name"] {
							notifications = append(notifications, targetConfigToNotification(tc, enc))
							break
						}
					}
				}
				break
			}
			// no keys
			for _, tc := range a.Config.Targets {
				notifications = append(notifications, targetConfigToNotification(tc, enc))
			}
		case "subscriptions":
			if e.Key != nil {
				if _, ok := e.Key["name"]; ok {
					for _, sub := range a.Config.Subscriptions {
						if sub.Name == e.Key["name"] {
							notifications = append(notifications, subscriptionConfigToNotification(sub, enc))
							break
						}
					}
				}
				break
			}
			// no keys
			for _, sub := range a.Config.Subscriptions {
				notifications = append(notifications, subscriptionConfigToNotification(sub, enc))
			}
		// case "outputs":
		// case "inputs":
		// case "processors":
		// case "clustering":
		// case "gnmi-server":
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unknown path element %q", e.Name)
		}
	}
	return notifications, nil
}

func targetConfigToNotification(tc *types.TargetConfig, e gnmi.Encoding) *gnmi.Notification {
	switch e {
	case gnmi.Encoding_JSON, gnmi.Encoding_JSON_IETF:
		b, _ := json.Marshal(tc)
		n := &gnmi.Notification{
			Timestamp: time.Now().UnixNano(),
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Origin: "gnmic",
						Elem: []*gnmi.PathElem{
							{
								Name: "target",
								Key:  map[string]string{"name": tc.Name},
							},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{JsonVal: b},
					},
				},
			},
		}
		return n
	case gnmi.Encoding_BYTES:
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
						Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(tc.Address)},
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
					Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(*tc.Username)},
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
					Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(fmt.Sprint(*tc.Insecure))},
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
					Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(fmt.Sprint(*tc.SkipVerify))},
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
				Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(tc.Timeout.String())},
			},
		})
		if tc.TLSCA != nil && *tc.TLSCA != "" {
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "tls-ca"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte((tc.TLSCAString()))},
				},
			})
		}
		if tc.TLSCert != nil && *tc.TLSCert != "" {
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "tls-cert"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(tc.TLSCertString())},
				},
			})
		}
		if tc.TLSKey != nil && *tc.TLSKey != "" {
			n.Update = append(n.Update, &gnmi.Update{
				Path: &gnmi.Path{
					Elem: []*gnmi.PathElem{
						{Name: "tls-key"},
					},
				},
				Val: &gnmi.TypedValue{
					Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(tc.TLSKeyString())},
				},
			})
		}
		if len(tc.Outputs) > 0 {
			typedVals := make([]*gnmi.TypedValue, 0, len(tc.Subscriptions))
			for _, out := range tc.Outputs {
				typedVals = append(typedVals, &gnmi.TypedValue{
					Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(out)},
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
					Value: &gnmi.TypedValue_BytesVal{BytesVal: []byte(sub)},
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
	case gnmi.Encoding_ASCII:
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
		if tc.TLSCA != nil && *tc.TLSCA != "" {
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
		if tc.TLSCert != nil && *tc.TLSCert != "" {
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
		if tc.TLSKey != nil && *tc.TLSKey != "" {
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
	return nil
}

func subscriptionConfigToNotification(sub *types.SubscriptionConfig, e gnmi.Encoding) *gnmi.Notification {
	switch e {
	case gnmi.Encoding_JSON, gnmi.Encoding_JSON_IETF:
		b, _ := json.Marshal(sub)
		n := &gnmi.Notification{
			Timestamp: time.Now().UnixNano(),
			Update: []*gnmi.Update{
				{
					Path: &gnmi.Path{
						Origin: "gnmic",
						Elem: []*gnmi.PathElem{
							{
								Name: "subscriptions",
								Key:  map[string]string{"name": sub.Name},
							},
						},
					},
					Val: &gnmi.TypedValue{
						Value: &gnmi.TypedValue_JsonVal{JsonVal: b},
					},
				},
			},
		}
		return n
	case gnmi.Encoding_BYTES:
	case gnmi.Encoding_ASCII:
	}
	return nil
}

func (a *App) serverGetHandler(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
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
		return a.handlegNMIcInternalGet(ctx, req)
	}

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
	if a.Config.Debug {
		a.Logger.Printf("sending GetResponse to %q: %+v", pr.Addr, response)
	}
	return response, nil
}

func (a *App) serverSetHandler(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
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

func (a *App) serverSubscribeHandler(req *gnmi.SubscribeRequest, stream gnmi.GNMI_SubscribeServer) error {
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

	a.Logger.Printf("received a subscribe request mode=%v from %q for target %q", sc.req.GetSubscribe().GetMode(), pr.Addr, sc.target)
	defer a.Logger.Printf("subscription from peer %q terminated", pr.Addr)

	// closing of this channel is handled by respective goroutines that are going to send error on this channel
	errChan := make(chan error, len(sc.req.GetSubscribe().GetSubscription()))
	sc.errChan = errChan // send-only

	switch sc.req.GetSubscribe().GetMode() {
	case gnmi.SubscriptionList_ONCE:
		go func() {
			a.handleONCESubscriptionRequest(sc)
			errChan <- sc.stream.Send(&gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_SyncResponse{SyncResponse: true},
			})
			close(errChan)
		}()

	case gnmi.SubscriptionList_POLL:
		go a.handlePolledSubscription(sc)
	case gnmi.SubscriptionList_STREAM:
		go a.handleStreamSubscriptionRequest(sc)
	default:
		return status.Errorf(codes.InvalidArgument, "unrecognized subscription mode: %v", sc.req.GetSubscribe().GetMode())
	}

	// flushing the errChan
	defer func() {
		a.Logger.Printf("flushing subscription errChan")
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
