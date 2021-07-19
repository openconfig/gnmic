package gnmi_output

import (
	"context"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/outputs"
	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func (s *server) Set(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	targetName := req.GetPrefix().GetTarget()
	if targetName == "" || targetName == "*" {
		return nil, status.Errorf(codes.InvalidArgument, "setRequest requires a single target value, received target: %s", targetName)
	}
	peer, _ := peer.FromContext(ctx)
	s.l.Printf("received Set request from %q to target %q", peer.Addr, targetName)

	s.mu.RLock()
	defer s.mu.RUnlock()
	var tc *collector.TargetConfig
	for n, tConfig := range s.targets {
		if outputs.GetHost(n) == targetName {
			tc = tConfig
			break
		}
	}
	if tc == nil {
		return nil, status.Errorf(codes.NotFound, "unknown target %q", targetName)
	}
	t := collector.NewTarget(tc)
	err := t.CreateGNMIClient(ctx)
	if err != nil {
		return nil, err
	}
	if req.GetPrefix() == nil {
		req.Prefix = new(gnmi.Path)
	}
	if req.GetPrefix().GetTarget() == "" || req.GetPrefix().GetTarget() == "*" {
		req.Prefix.Target = targetName
	}
	res, err := t.Set(ctx, req)
	if err != nil {
		return nil, err
	}
	if res.GetPrefix() == nil {
		res.Prefix = new(gnmi.Path)
	}
	if res.GetPrefix().GetTarget() == "" {
		res.Prefix.Target = targetName
	}
	return res, nil
}
