package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/karimra/gnmic/types"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"google.golang.org/grpc"
)

func (a *App) ClientCapabilities(ctx context.Context, tc *types.TargetConfig, ext ...*gnmi_ext.Extension) (*gnmi.CapabilityResponse, error) {
	// acquire writer lock
	a.operLock.Lock()
	t, err := a.initTarget(tc)
	a.operLock.Unlock()
	if err != nil {
		return nil, err
	}
	// acquire reader lock
	a.operLock.RLock()
	defer a.operLock.RUnlock()

	err = a.CreateGNMIClient(ctx, t)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, t.Config.Timeout)
	defer cancel()
	capResponse, err := t.Capabilities(ctx, ext...)
	if err != nil {
		return nil, fmt.Errorf("%q CapabilitiesRequest failed: %v", t.Config.Address, err)
	}
	return capResponse, nil

}

func (a *App) ClientGet(ctx context.Context, tc *types.TargetConfig, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	a.operLock.Lock()
	t, err := a.initTarget(tc)
	a.operLock.Unlock()
	if err != nil {
		return nil, err
	}
	// acquire reader lock
	a.operLock.RLock()
	defer a.operLock.RUnlock()

	err = a.CreateGNMIClient(ctx, t)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, t.Config.Timeout)
	defer cancel()
	getResponse, err := t.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%q GetRequest failed: %v", t.Config.Address, err)
	}
	return getResponse, nil
}

func (a *App) ClientSet(ctx context.Context, tc *types.TargetConfig, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	a.operLock.Lock()
	t, err := a.initTarget(tc)
	a.operLock.Unlock()
	if err != nil {
		return nil, err
	}
	// acquire reader lock
	a.operLock.RLock()
	defer a.operLock.RUnlock()

	if t.Client == nil {
		targetDialOpts := a.dialOpts
		if a.Config.UseTunnelServer {
			targetDialOpts = append(targetDialOpts,
				grpc.WithContextDialer(a.tunDialerFn(ctx, tc.Name)),
			)
			// overwrite target address
			t.Config.Address = t.Config.Name
		}
		if err := t.CreateGNMIClient(ctx, targetDialOpts...); err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, fmt.Errorf("failed to create a gRPC client for target %q, timeout (%s) reached", t.Config.Name, t.Config.Timeout)
			}
			return nil, fmt.Errorf("failed to create a gRPC client for target %q : %v", t.Config.Name, err)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, t.Config.Timeout)
	defer cancel()
	setResponse, err := t.Set(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("target %q SetRequest failed: %v", t.Config.Name, err)
	}
	return setResponse, nil
}
