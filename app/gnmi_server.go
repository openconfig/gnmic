package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/hashicorp/consul/api"
	"github.com/karimra/gnmic/cache"
	"github.com/karimra/gnmic/target"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/openconfig/gnmi/proto/gnmi"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type streamClient struct {
	target string
	req    *gnmi.SubscribeRequest

	stream  gnmi.GNMI_SubscribeServer
	errChan chan<- error
}

func (a *App) startGnmiServer() {
	if a.Config.GnmiServer == nil {
		a.c = nil
		return
	}
	var err error
	a.c, err = cache.New(a.Config.GnmiServer.Cache, cache.WithLogger(a.Logger))
	if err != nil {
		a.Logger.Printf("failed to initialize gNMI cache: %v", err)
		return
	}

	a.subscribeRPCsem = semaphore.NewWeighted(a.Config.GnmiServer.MaxSubscriptions)
	a.unaryRPCsem = semaphore.NewWeighted(a.Config.GnmiServer.MaxUnaryRPC)
	//
	var l net.Listener
	network := "tcp"
	addr := a.Config.GnmiServer.Address
	if strings.HasPrefix(a.Config.GnmiServer.Address, "unix://") {
		network = "unix"
		addr = strings.TrimPrefix(addr, "unix://")
	}

	opts, err := a.gRPCServerOpts()
	if err != nil {
		a.Logger.Printf("failed to build gRPC server options: %v", err)
		return
	}
	for {
		l, err = net.Listen(network, addr)
		if err != nil {
			a.Logger.Printf("failed to start gRPC server listener: %v", err)
			time.Sleep(time.Second)
			continue
		}
		break
	}

	a.grpcSrv = grpc.NewServer(opts...)
	gnmi.RegisterGNMIServer(a.grpcSrv, a)
	//
	ctx, cancel := context.WithCancel(a.ctx)
	go func() {
		err = a.grpcSrv.Serve(l)
		if err != nil {
			a.Logger.Printf("gRPC server shutdown: %v", err)
		}
		cancel()
	}()
	go a.registerGNMIServer(ctx)
}

func (a *App) registerGNMIServer(ctx context.Context) {
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
		ID:      a.Config.GnmiServer.ServiceRegistration.Name,
		Name:    a.Config.GnmiServer.ServiceRegistration.Name,
		Address: h,
		Port:    pi,
		Tags:    a.Config.GnmiServer.ServiceRegistration.Tags,
		Checks: api.AgentServiceChecks{
			{
				TTL:                            a.Config.GnmiServer.ServiceRegistration.CheckInterval.String(),
				DeregisterCriticalServiceAfter: a.Config.GnmiServer.ServiceRegistration.DeregisterAfter,
			},
		},
	}
	if a.Config.Clustering != nil {
		service.ID = a.Config.Clustering.InstanceName
		service.Name = a.Config.Clustering.ClusterName + "-gnmi-server"
		if service.Tags == nil {
			service.Tags = make([]string, 0)
		}
		service.Tags = append(service.Tags, fmt.Sprintf("cluster-name=%s", a.Config.Clustering.ClusterName))
		service.Tags = append(service.Tags, fmt.Sprintf("instance-name=%s", a.Config.Clustering.InstanceName))
	}
	//
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

func (a *App) gRPCServerOpts() ([]grpc.ServerOption, error) {
	opts := make([]grpc.ServerOption, 0)
	if a.Config.GnmiServer.EnableMetrics && a.reg != nil {
		grpcMetrics := grpc_prometheus.NewServerMetrics()
		opts = append(opts,
			grpc.StreamInterceptor(grpcMetrics.StreamServerInterceptor()),
			grpc.UnaryInterceptor(grpcMetrics.UnaryServerInterceptor()),
		)
		a.reg.MustRegister(grpcMetrics)
	}

	tlscfg, err := utils.NewTLSConfig(
		a.Config.GnmiServer.CaFile,
		a.Config.GnmiServer.CertFile,
		a.Config.GnmiServer.KeyFile,
		a.Config.GnmiServer.SkipVerify,
		true,
	)
	if err != nil {
		return nil, err
	}
	if tlscfg != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlscfg)))
	}

	return opts, nil
}

func (a *App) selectGNMITargets(target string) (map[string]*types.TargetConfig, error) {
	if target == "" || target == "*" {
		return a.Config.Targets, nil
	}
	targetsNames := strings.Split(target, ",")
	targets := make(map[string]*types.TargetConfig)
	a.configLock.RLock()
	defer a.configLock.RUnlock()
OUTER:
	for i := range targetsNames {
		for n, tc := range a.Config.Targets {
			if utils.GetHost(n) == targetsNames[i] {
				targets[n] = tc
				continue OUTER
			}
		}
		return nil, status.Errorf(codes.NotFound, "target %q is not known", targetsNames[i])
	}
	return targets, nil
}

func (a *App) Get(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	ok := a.unaryRPCsem.TryAcquire(1)
	if !ok {
		return nil, status.Errorf(codes.ResourceExhausted, "max number of Unary RPC reached")
	}
	defer a.unaryRPCsem.Release(1)

	numPaths := len(req.GetPath())
	if numPaths == 0 && req.GetPrefix() == nil {
		return nil, status.Errorf(codes.InvalidArgument, "missing path")
	}

	a.configLock.RLock()
	defer a.configLock.RUnlock()

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

	targets, err := a.selectGNMITargets(targetName)
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
	for name, tc := range targets {
		go func(name string, tc *types.TargetConfig) {
			name = utils.GetHost(name)
			defer wg.Done()
			t := target.NewTarget(tc)
			ctx, cancel := context.WithTimeout(ctx, tc.Timeout)
			defer cancel()
			err := a.CreateGNMIClient(ctx, t)
			if err != nil {
				a.Logger.Printf("target %q err: %v", name, err)
				errChan <- fmt.Errorf("target %q err: %v", name, err)
				return
			}
			defer t.Close()
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
	a.Logger.Printf("sending GetResponse to %q: %+v", pr.Addr, response)
	return response, nil
}

func (a *App) Set(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	ok := a.unaryRPCsem.TryAcquire(1)
	if !ok {
		return nil, status.Errorf(codes.ResourceExhausted, "max number of Unary RPC reached")
	}
	defer a.unaryRPCsem.Release(1)

	numUpdates := len(req.GetUpdate())
	numReplaces := len(req.GetReplace())
	numDeletes := len(req.GetDelete())
	if numUpdates+numReplaces+numDeletes == 0 {
		return nil, status.Errorf(codes.InvalidArgument, "missing update/replace/delete path(s)")
	}

	a.configLock.RLock()
	defer a.configLock.RUnlock()

	targetName := req.GetPrefix().GetTarget()
	pr, _ := peer.FromContext(ctx)
	a.Logger.Printf("received Set request from %q to target %q", pr.Addr, targetName)

	targets, err := a.selectGNMITargets(targetName)
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
	for name, tc := range targets {
		go func(name string, tc *types.TargetConfig) {
			name = utils.GetHost(name)
			defer wg.Done()
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
				a.Logger.Printf("target %q err: %v", name, err)
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
				a.Logger.Printf("target %q err: %v", name, err)
				errChan <- fmt.Errorf("target %q err: %v", name, err)
				return
			}
			for _, upd := range res.GetResponse() {
				upd.Path.Target = name
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
	a.Logger.Printf("sending SetResponse to %q: %+v", pr.Addr, response)
	return response, nil
}

func (a *App) Subscribe(stream gnmi.GNMI_SubscribeServer) error {
	pr, _ := peer.FromContext(stream.Context())
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

	a.Logger.Printf("received a subscribe request mode=%v from %q for target %q", sc.req.GetSubscribe().GetMode(), pr.Addr, sc.target)
	defer a.Logger.Printf("subscription from peer %q terminated", pr.Addr)

	errChan := make(chan error, 3)
	sc.errChan = make(chan error, 3)

	a.Logger.Printf("acquiring subscription spot for target %q", sc.target)
	ok := a.subscribeRPCsem.TryAcquire(1)
	if !ok {
		return status.Errorf(codes.ResourceExhausted, "could not acquire a subscription spot")
	}
	defer a.subscribeRPCsem.Release(1)

	a.Logger.Printf("acquired subscription spot for target %q", sc.target)

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

	for err := range errChan {
		if err != nil {
			return status.Errorf(codes.Internal, "%v", err)
		}
	}
	return nil
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
	var err error
	a.Logger.Printf("processing STREAM subscription from %q to target %q", peer.Addr, sc.target)

	defer func() {
		if err == nil {
			a.Logger.Printf("subscription request from %q to target %q processed", peer.Addr, sc.target)
			return
		}
		if errors.Is(err, context.Canceled) {
			a.Logger.Printf("subscription to target %q canceled", sc.target)
			sc.errChan <- err
			return
		}
		if err != nil {
			a.Logger.Printf("error processing STREAM subscription to target %q: %v", sc.target, err)
			sc.errChan <- err
			return
		}
	}()

	if sc.req.GetSubscribe().GetUpdatesOnly() {
		err = sc.stream.Send(&gnmi.SubscribeResponse{
			Response: &gnmi.SubscribeResponse_SyncResponse{SyncResponse: true},
		})
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
			switch sub.GetMode() {
			case gnmi.SubscriptionMode_ON_CHANGE, gnmi.SubscriptionMode_TARGET_DEFINED:
				ro := &cache.ReadOpts{
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
				a.Logger.Printf("cache subscribe: %+v", ro)
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
				return
			case gnmi.SubscriptionMode_SAMPLE:
				period := time.Duration(sub.GetSampleInterval())
				if period == 0 {
					period = a.Config.GnmiServer.DefaultSampleInterval
				} else if period < a.Config.GnmiServer.MinSampleInterval {
					period = a.Config.GnmiServer.MinSampleInterval
				}
				ro := &cache.ReadOpts{
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
				a.Logger.Printf("cache subscribe: %+v", ro)
				for n := range a.c.Subscribe(sc.stream.Context(), ro) {
					if n.Err != nil {
						err = n.Err
						a.Logger.Printf("cache subscribe failed: %+v: %v", ro, err)
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
				return
			}
		}(sub)
	}

	// wait for ctx to be done
	<-sc.stream.Context().Done()
	err = sc.stream.Context().Err()
	wg.Wait()
}

func (a *App) handlePolledSubscription(sc *streamClient) {
	a.handleONCESubscriptionRequest(sc)
	defer close(sc.errChan)
	var err error
	for {
		_, err = sc.stream.Recv()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			a.Logger.Printf("target %q: failed poll subscription rcv: %v", sc.target, err)
			sc.errChan <- err
			return
		}
		a.Logger.Printf("target %q: repoll", sc.target)
		a.handleONCESubscriptionRequest(sc)
		a.Logger.Printf("target %q: repoll done", sc.target)
	}
}

////

func (a *App) handlegNMIcInternalGet(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	notifications := make([]*gnmi.Notification, 0, len(req.GetPath()))
	a.configLock.RLock()
	defer a.configLock.RUnlock()
	for _, p := range req.GetPath() {
		elems := utils.PathElems(req.GetPrefix(), p)
		ns, err := a.handlegNMIGetPath(elems, req.GetEncoding())
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, ns...)
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
