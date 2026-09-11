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
	"fmt"
	"net"
	"slices"
	"strconv"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openconfig/gnmic/pkg/api/path"
	"github.com/openconfig/gnmic/pkg/api/server"
	"github.com/openconfig/gnmic/pkg/api/target"
	"github.com/openconfig/gnmic/pkg/cache"
	"github.com/openconfig/gnmic/pkg/gnmiserver"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/openconfig/gnmic/pkg/utils"
)

func (a *App) startGnmiServer() error {
	if a.Config.GnmiServer == nil {
		a.c = nil
		return nil
	}

	var err error
	a.c, err = cache.New(a.Config.GnmiServer.Cache, cache.WithLogger(a.Logger))
	if err != nil {
		logging.LogErrUnlessCanceled(a.Logger, err, "failed to initialize gNMI cache")
		return err
	}

	h := gnmiserver.New(
		gnmiserver.Config{
			DefaultSampleInterval: a.Config.GnmiServer.DefaultSampleInterval,
			MinSampleInterval:     a.Config.GnmiServer.MinSampleInterval,
			MinHeartbeatInterval:  a.Config.GnmiServer.MinHeartbeatInterval,
			Debug:                 a.Config.Debug,
		},
		a.c,
		a.Logger,
		func(ctx context.Context, tn string) (map[string]*target.Target, func(), error) {
			// selectTargets adopts on demand targets into a.Targets,
			// no cleanup is needed.
			targets, err := a.selectTargets(ctx, tn)
			return targets, func() {}, err
		},
		a.handlegNMIcInternalGet,
	)
	opts := []server.Option{
		server.WithLogger(a.Logger),
		server.WithCapabilitiesHandler(h.Capabilities),
		server.WithGetHandler(h.Get),
		server.WithSubscribeHandler(h.Subscribe),
		server.WithRegistry(a.reg),
	}
	// when the server is read-only, the Set handler is not registered
	// and Set RPCs are rejected with an `Unimplemented` status.
	if a.Config.GnmiServer.IsReadOnly() {
		a.Logger.Info("gNMI server is read-only, Set RPCs are disabled")
	} else {
		opts = append(opts, server.WithSetHandler(h.Set))
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
		Timeout:              a.Config.GnmiServer.Timeout,
		HealthEnabled:        true,
		TLS:                  a.Config.GnmiServer.TLS,
	}, opts...)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(a.ctx)

	go a.registerGNMIServer(ctx)
	go func() {
		defer cancel()
		err := s.Start(ctx)
		if err != nil {
			logging.LogErrUnlessCanceled(a.Logger, err, "gNMI server exited")
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
		logging.LogErrUnlessCanceled(a.Logger, err, "failed to connect to consul")
		time.Sleep(1 * time.Second)
		goto INITCONSUL
	}
	self, err := consulClient.Agent().Self()
	if err != nil {
		logging.LogErrUnlessCanceled(a.Logger, err, "failed to connect to consul")
		time.Sleep(1 * time.Second)
		goto INITCONSUL
	}
	if cfg, ok := self["Config"]; ok {
		b, _ := json.Marshal(cfg)
		a.Logger.Info("consul agent config", "config", string(b))
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	h, p, err := net.SplitHostPort(a.Config.GnmiServer.Address)
	if err != nil {
		logging.LogErrUnlessCanceled(a.Logger, err, "failed to split host and port from gNMI server address", "address", a.Config.GnmiServer.Address)
		return
	}
	pi, _ := strconv.Atoi(p)
	service := &api.AgentServiceRegistration{
		ID:      a.Config.InstanceName,
		Name:    a.Config.GnmiServer.ServiceRegistration.Name,
		Address: h,
		Port:    pi,
		Tags:    slices.Concat(defaultTags, a.Config.GnmiServer.ServiceRegistration.Tags),
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
	a.Logger.Info("registering service", "service", string(b))
	err = consulClient.Agent().ServiceRegister(service)
	if err != nil {
		logging.LogErrUnlessCanceled(a.Logger, err, "failed to register service in consul")
		return
	}

	err = consulClient.Agent().UpdateTTL(ttlCheckID, "", api.HealthPassing)
	if err != nil {
		logging.LogErrUnlessCanceled(a.Logger, err, "failed to update TTL check to Passing")
	}
	ticker := time.NewTicker(a.Config.GnmiServer.ServiceRegistration.CheckInterval / 2)
	for {
		select {
		case <-ticker.C:
			err = consulClient.Agent().UpdateTTL(ttlCheckID, "", api.HealthPassing)
			if err != nil {
				logging.LogErrUnlessCanceled(a.Logger, err, "failed to update TTL check to Passing")
			}
		case <-ctx.Done():
			err = consulClient.Agent().UpdateTTL(ttlCheckID, ctx.Err().Error(), api.HealthCritical)
			if err != nil {
				logging.LogErrUnlessCanceled(a.Logger, err, "failed to update TTL check to Critical")
			}
			ticker.Stop()
			goto INITCONSUL
		}
	}
}

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
			name, withKey := e.Key["name"]
			for _, tc := range a.Config.Targets {
				if withKey && tc.Name != name {
					continue
				}
				n := utils.TargetConfigToNotification(tc, enc)
				if n == nil {
					return nil, status.Errorf(codes.Unimplemented, "encoding %s is not supported for %q", enc, e.Name)
				}
				notifications = append(notifications, n)
			}
		case "subscriptions":
			name, withKey := e.Key["name"]
			for _, sub := range a.Config.Subscriptions {
				if withKey && sub.Name != name {
					continue
				}
				n := utils.SubscriptionConfigToNotification(sub, enc)
				if n == nil {
					return nil, status.Errorf(codes.Unimplemented, "encoding %s is not supported for %q", enc, e.Name)
				}
				notifications = append(notifications, n)
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
