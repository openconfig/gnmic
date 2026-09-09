// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia's intellectual property are granted for any other purpose.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package gnmiserver

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/openconfig/gnmic/pkg/api/path"
	"github.com/openconfig/gnmic/pkg/api/target"
	"github.com/openconfig/gnmic/pkg/api/types"
	apiutils "github.com/openconfig/gnmic/pkg/api/utils"
	targets_manager "github.com/openconfig/gnmic/pkg/collector/managers/targets"
	"github.com/openconfig/gnmic/pkg/utils"
)

// selectTargets resolves the request Prefix.Target value to a set of gNMI
// targets. An empty value or `*` selects all the targets managed by this
// instance. A comma separated list of names selects the matching targets,
// by name or by host (name without port number).
//
// Targets managed by this instance with an active gNMI client are used
// directly. Other configured targets are dialed on demand; the returned
// cleanup function closes these temporary connections.
func (s *Server) selectTargets(ctx context.Context, tn string) (map[string]*target.Target, func(), error) {
	targets := make(map[string]*target.Target)
	temp := make([]*target.Target, 0)
	cleanup := func() {
		for _, t := range temp {
			_ = t.Close()
		}
	}

	// collect targets managed by this instance that have an active client.
	managed := make(map[string]*target.Target)
	s.targetsManager.ForEach(func(mt *targets_manager.ManagedTarget) {
		mt.RLock()
		defer mt.RUnlock()
		if mt.T != nil && mt.T.Client != nil {
			managed[mt.Name] = mt.T
		}
	})

	if tn == "" || tn == "*" {
		// all configured targets: managed ones are used as is,
		// the rest are dialed on demand.
		cfgs, err := s.store.Config.List("targets")
		if err != nil {
			cleanup()
			return nil, func() {}, err
		}
		for name, v := range cfgs {
			if t, ok := managed[name]; ok {
				targets[name] = t
				continue
			}
			tc, ok := v.(*types.TargetConfig)
			if !ok {
				continue
			}
			t, err := s.createTemporaryTarget(ctx, tc)
			if err != nil {
				cleanup()
				return nil, func() {}, err
			}
			temp = append(temp, t)
			targets[name] = t
		}
		return targets, cleanup, nil
	}

	targetsNames := strings.Split(tn, ",")
	for _, name := range targetsNames {
		found := false
		for n, t := range managed {
			if n == name || apiutils.GetHost(n) == name {
				targets[n] = t
				found = true
			}
		}
		if found {
			continue
		}
		// not managed by this instance (or not connected):
		// look it up in the configuration and dial on demand.
		cfgs, err := s.store.Config.List("targets")
		if err != nil {
			cleanup()
			return nil, func() {}, err
		}
		for n, v := range cfgs {
			if n != name && apiutils.GetHost(n) != name {
				continue
			}
			tc, ok := v.(*types.TargetConfig)
			if !ok {
				continue
			}
			t, err := s.createTemporaryTarget(ctx, tc)
			if err != nil {
				cleanup()
				return nil, func() {}, err
			}
			temp = append(temp, t)
			targets[n] = t
			break
		}
	}
	return targets, cleanup, nil
}

// createTemporaryTarget creates a gNMI target with an active client from a
// target configuration. It is used to relay unary RPCs to targets that are
// not currently connected by this instance. The caller must Close() the
// returned target.
func (s *Server) createTemporaryTarget(ctx context.Context, tc *types.TargetConfig) (*target.Target, error) {
	if tc.TunnelTargetType != "" {
		return nil, fmt.Errorf("target %q: cannot dial a tunnel target on demand", tc.Name)
	}
	t := target.NewTarget(tc)
	err := t.CreateGNMIClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("target %q: %v", tc.Name, err)
	}
	return t, nil
}

// handleInternalGet handles Get requests with origin `gnmic`,
// serving the collector's own configuration from the store.
func (s *Server) handleInternalGet(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	notifications := make([]*gnmi.Notification, 0, len(req.GetPath()))

	for _, p := range req.GetPath() {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			elems := path.PathElems(req.GetPrefix(), p)
			ns, err := s.handleInternalGetPath(elems, req.GetEncoding())
			if err != nil {
				return nil, err
			}
			notifications = append(notifications, ns...)
		}
	}
	return &gnmi.GetResponse{Notification: notifications}, nil
}

func (s *Server) handleInternalGetPath(elems []*gnmi.PathElem, enc gnmi.Encoding) ([]*gnmi.Notification, error) {
	notifications := make([]*gnmi.Notification, 0, len(elems))
	for _, e := range elems {
		switch e.Name {
		case "targets":
			tcs, err := s.listTargetConfigs()
			if err != nil {
				return nil, status.Errorf(codes.Internal, "%v", err)
			}
			name, withKey := e.Key["name"]
			for _, tc := range tcs {
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
			subs, err := s.listSubscriptionConfigs()
			if err != nil {
				return nil, status.Errorf(codes.Internal, "%v", err)
			}
			name, withKey := e.Key["name"]
			for _, sub := range subs {
				if withKey && sub.Name != name {
					continue
				}
				n := utils.SubscriptionConfigToNotification(sub, enc)
				if n == nil {
					return nil, status.Errorf(codes.Unimplemented, "encoding %s is not supported for %q", enc, e.Name)
				}
				notifications = append(notifications, n)
			}
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unknown path element %q", e.Name)
		}
	}
	return notifications, nil
}

func (s *Server) listTargetConfigs() ([]*types.TargetConfig, error) {
	vs, err := s.store.Config.List("targets")
	if err != nil {
		return nil, err
	}
	tcs := make([]*types.TargetConfig, 0, len(vs))
	for name, v := range vs {
		var tc types.TargetConfig
		switch v := v.(type) {
		case *types.TargetConfig:
			tc = *v
		case types.TargetConfig:
			tc = v
		default:
			continue
		}
		// configs loaded from file are stored without
		// the name field set, default to the store key.
		if tc.Name == "" {
			tc.Name = name
		}
		tcs = append(tcs, &tc)
	}
	sort.Slice(tcs, func(i, j int) bool { return tcs[i].Name < tcs[j].Name })
	return tcs, nil
}

func (s *Server) listSubscriptionConfigs() ([]*types.SubscriptionConfig, error) {
	vs, err := s.store.Config.List("subscriptions")
	if err != nil {
		return nil, err
	}
	subs := make([]*types.SubscriptionConfig, 0, len(vs))
	for name, v := range vs {
		var sub types.SubscriptionConfig
		switch v := v.(type) {
		case *types.SubscriptionConfig:
			sub = *v
		case types.SubscriptionConfig:
			sub = v
		default:
			continue
		}
		// configs loaded from file are stored without
		// the name field set, default to the store key.
		if sub.Name == "" {
			sub.Name = name
		}
		subs = append(subs, &sub)
	}
	sort.Slice(subs, func(i, j int) bool { return subs[i].Name < subs[j].Name })
	return subs, nil
}
