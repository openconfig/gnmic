package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"strings"
	"sync"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/outputs"
	"github.com/karimra/gnmic/utils"
	"github.com/openconfig/gnmi/coalesce"
	"github.com/openconfig/gnmi/ctree"
	"github.com/openconfig/gnmi/match"
	"github.com/openconfig/gnmi/path"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/subscribe"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type streamClient struct {
	target  string
	req     *gnmi.SubscribeRequest
	queue   *coalesce.Queue
	stream  gnmi.GNMI_SubscribeServer
	errChan chan<- error
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

func (a *App) startGnmiServer() {
	if a.Config.GnmiServer == nil {
		a.c = nil
		return
	}
	a.match = match.New()

	a.subscribeRPCsem = semaphore.NewWeighted(a.Config.GnmiServer.MaxSubscriptions)
	a.unaryRPCsem = semaphore.NewWeighted(a.Config.GnmiServer.MaxUnaryRPC)
	a.c.SetClient(a.Update)
	//
	var l net.Listener
	var err error
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
LISTENER:
	l, err = net.Listen(network, addr)
	if err != nil {
		a.Logger.Printf("failed to start gRPC server listener: %v", err)
		time.Sleep(time.Second)
		goto LISTENER
	}

	a.grpcSrv = grpc.NewServer(opts...)
	gnmi.RegisterGNMIServer(a.grpcSrv, a)
	go a.grpcSrv.Serve(l)
}

func (a *App) gRPCServerOpts() ([]grpc.ServerOption, error) {
	opts := make([]grpc.ServerOption, 0)
	if a.Config.GnmiServer.EnableMetrics {
		opts = append(opts, grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor))
	}
	if a.Config.GnmiServer.SkipVerify || a.Config.GnmiServer.CaFile != "" || (a.Config.GnmiServer.CertFile != "" && a.Config.GnmiServer.KeyFile != "") {
		tlscfg := &tls.Config{
			Renegotiation:      tls.RenegotiateNever,
			InsecureSkipVerify: a.Config.GnmiServer.SkipVerify,
		}
		if a.Config.GnmiServer.CertFile != "" && a.Config.GnmiServer.KeyFile != "" {
			certificate, err := tls.LoadX509KeyPair(a.Config.GnmiServer.CertFile, a.Config.GnmiServer.KeyFile)
			if err != nil {
				return nil, err
			}
			tlscfg.Certificates = []tls.Certificate{certificate}
			// tlscfg.BuildNameToCertificate()
		} else {
			cert, err := utils.SelfSignedCerts()
			if err != nil {
				return nil, err
			}
			tlscfg.Certificates = []tls.Certificate{cert}
		}
		if a.Config.GnmiServer.CaFile != "" {
			certPool := x509.NewCertPool()
			caFile, err := ioutil.ReadFile(a.Config.GnmiServer.CaFile)
			if err != nil {
				return nil, err
			}
			if ok := certPool.AppendCertsFromPEM(caFile); !ok {
				return nil, errors.New("failed to append certificate")
			}
			tlscfg.RootCAs = certPool
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlscfg)))
	}
	return opts, nil
}

func (a *App) selectGNMITargets(target string) (map[string]*collector.TargetConfig, error) {
	if target == "" || target == "*" {
		return a.Config.GetTargets()
	}
	targetsNames := strings.Split(target, ",")
	targets := make(map[string]*collector.TargetConfig)
	a.m.RLock()
	defer a.m.RUnlock()
OUTER:
	for i := range targetsNames {
		for n, tc := range a.Config.Targets {
			if outputs.GetHost(n) == targetsNames[i] {
				targets[n] = tc
				continue OUTER
			}
		}
		return nil, status.Errorf(codes.NotFound, "target %q is not known", targetsNames[i])
	}
	return targets, nil
}

func (a *App) Update(n *ctree.Leaf) {
	switch v := n.Value().(type) {
	case *gnmi.Notification:
		subscribe.UpdateNotification(a.match, n, v, path.ToStrings(v.Prefix, true))
	default:
		a.Logger.Printf("unexpected update type: %T", v)
	}
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

	a.m.RLock()
	defer a.m.RUnlock()

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

	target := req.GetPrefix().GetTarget()
	peer, _ := peer.FromContext(ctx)
	a.Logger.Printf("received Get request from %q to target %q", peer.Addr, target)

	targets, err := a.selectGNMITargets(target)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not find targets: %v", err)
	}
	numTargets := len(targets)
	if numTargets == 0 {
		return nil, status.Errorf(codes.NotFound, "unknown target %q", target)
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
		go func(name string, tc *collector.TargetConfig) {
			// name = outputs.GetHost(name)
			defer wg.Done()
			t := collector.NewTarget(tc)
			ctx, cancel := context.WithTimeout(ctx, tc.Timeout)
			defer cancel()
			err := t.CreateGNMIClient(ctx)
			if err != nil {
				a.Logger.Printf("target %q err: %v", name, err)
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
	a.Logger.Printf("sending GetResponse to %q: %+v", peer.Addr, response)
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

	a.m.RLock()
	defer a.m.RUnlock()

	target := req.GetPrefix().GetTarget()
	peer, _ := peer.FromContext(ctx)
	a.Logger.Printf("received Set request from %q to target %q", peer.Addr, target)

	targets, err := a.selectGNMITargets(target)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "could not find targets: %v", err)
	}
	numTargets := len(targets)
	if numTargets == 0 {
		return nil, status.Errorf(codes.NotFound, "unknown target(s) %q", target)
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
		go func(name string, tc *collector.TargetConfig) {
			name = outputs.GetHost(name)
			defer wg.Done()
			t := collector.NewTarget(tc)
			err := t.CreateGNMIClient(ctx)
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
	a.Logger.Printf("sending SetResponse to %q: %+v", peer.Addr, response)
	return response, nil
}

func (a *App) Subscribe(stream gnmi.GNMI_SubscribeServer) error {
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
	if !a.c.HasTarget(sc.target) {
		return status.Errorf(codes.NotFound, "target %q not found", sc.target)
	}
	peer, _ := peer.FromContext(stream.Context())
	a.Logger.Printf("received a subscribe request mode=%v from %q for target %q", sc.req.GetSubscribe().GetMode(), peer.Addr, sc.target)
	defer a.Logger.Printf("subscription from peer %q terminated", peer.Addr)

	sc.queue = coalesce.NewQueue()
	errChan := make(chan error, 3)
	sc.errChan = errChan

	a.Logger.Printf("acquiring subscription spot for target %q", sc.target)
	ok := a.subscribeRPCsem.TryAcquire(1)
	if !ok {
		return status.Errorf(codes.ResourceExhausted, "could not acquire a subscription spot")
	}
	a.Logger.Printf("acquired subscription spot for target %q", sc.target)

	switch sc.req.GetSubscribe().GetMode() {
	case gnmi.SubscriptionList_ONCE:
		go func() {
			a.handleSubscriptionRequest(sc)
			sc.queue.Close()
		}()
	case gnmi.SubscriptionList_POLL:
		go a.handlePolledSubscription(sc)
	case gnmi.SubscriptionList_STREAM:
		if sc.req.GetSubscribe().GetUpdatesOnly() {
			sc.queue.Insert(syncMarker{})
		}
		remove := addSubscription(a.match, sc.req.GetSubscribe(), &matchClient{queue: sc.queue})
		defer remove()
		if !sc.req.GetSubscribe().GetUpdatesOnly() {
			go a.handleSubscriptionRequest(sc)
		}
	default:
		return status.Errorf(codes.InvalidArgument, "unrecognized subscription mode: %v", sc.req.GetSubscribe().GetMode())
	}
	// send all nodes added to queue
	go a.sendStreamingResults(sc)

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

func addSubscription(m *match.Match, s *gnmi.SubscriptionList, c *matchClient) func() {
	var removes []func()
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

func (a *App) handleSubscriptionRequest(sc *streamClient) {
	var err error
	a.Logger.Printf("processing subscription to target %q", sc.target)
	defer func() {
		if err != nil {
			a.Logger.Printf("error processing subscription to target %q: %v", sc.target, err)
			sc.queue.Close()
			sc.errChan <- err
			return
		}
		a.Logger.Printf("subscription request to target %q processed", sc.target)
	}()

	if !sc.req.GetSubscribe().GetUpdatesOnly() {
		for _, sub := range sc.req.GetSubscribe().GetSubscription() {
			var fp []string
			fp, err = path.CompletePath(sc.req.GetSubscribe().GetPrefix(), sub.GetPath())
			if err != nil {
				return
			}
			err = a.c.Query(sc.target, fp,
				func(_ []string, l *ctree.Leaf, _ interface{}) error {
					if err != nil {
						return err
					}
					_, err = sc.queue.Insert(l)
					return nil
				})
			if err != nil {
				a.Logger.Printf("target %q failed internal cache query: %v", sc.target, err)
				return
			}
		}
	}
	_, err = sc.queue.Insert(syncMarker{})
}

func (a *App) sendStreamingResults(sc *streamClient) {
	ctx := sc.stream.Context()
	peer, _ := peer.FromContext(ctx)
	a.Logger.Printf("sending streaming results from target %q to peer %q", sc.target, peer.Addr)
	defer a.subscribeRPCsem.Release(1)
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
		err = a.sendSubscribeResponse(&resp{
			stream: sc.stream,
			n:      node,
			dup:    dup,
		}, sc)
		if err != nil {
			a.Logger.Printf("target %q: failed sending subscribeResponse: %v", sc.target, err)
			sc.errChan <- err
			return
		}
		// TODO: check if target was deleted ? necessary ?
	}
}

func (a *App) handlePolledSubscription(sc *streamClient) {
	a.handleSubscriptionRequest(sc)
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
			a.Logger.Printf("target %q: failed poll subscription rcv: %v", sc.target, err)
			sc.errChan <- err
			return
		}
		a.Logger.Printf("target %q: repoll", sc.target)
		a.handleSubscriptionRequest(sc)
		a.Logger.Printf("target %q: repoll done", sc.target)
	}
}

func (a *App) sendSubscribeResponse(r *resp, sc *streamClient) error {
	notif, err := subscribe.MakeSubscribeResponse(r.n.Value(), r.dup)
	if err != nil {
		return status.Errorf(codes.Unknown, "unknown error: %v", err)
	}
	// No acls
	return r.stream.Send(notif)
}

////

func (a *App) handlegNMIcInternalGet(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	notifications := make([]*gnmi.Notification, 0, len(req.GetPath()))
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
		case "":
		case "targets":
			tcs, err := a.Config.GetTargets()
			if err != nil {
				return nil, err
			}
			if e.Key != nil {
				if _, ok := e.Key["name"]; ok {
					for _, tc := range tcs {
						if tc.Name == e.Key["name"] {
							notifications = append(notifications, targetConfigToNotification(tc, enc))
							break
						}
					}
				}
				break
			}
			for _, tc := range tcs {
				notifications = append(notifications, targetConfigToNotification(tc, enc))
			}
		case "subscriptions":
			subs, err := a.Config.GetSubscriptions(nil)
			if err != nil {
				return nil, err
			}
			for _, sub := range subs {
				notifications = append(notifications, subscriptionConfigToNotification(sub))
			}
		case "processors":
		case "clustering":
		case "gnmi-server":
		}
	}
	return notifications, nil
}

func targetConfigToNotification(tc *collector.TargetConfig, e gnmi.Encoding) *gnmi.Notification {
	switch e {
	case gnmi.Encoding_JSON:
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
		if tc.TLSCA != nil && tc.TLSCAString() != "NA" {
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
		if tc.TLSCert != nil && tc.TLSCertString() != "NA" {
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
	case gnmi.Encoding_JSON_IETF:
	}
	return nil
}

func subscriptionConfigToNotification(sub *collector.SubscriptionConfig) *gnmi.Notification {
	return nil
}
