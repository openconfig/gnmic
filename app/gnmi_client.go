package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
)

func (a *App) ClientCapabilities(ctx context.Context, tName string, ext ...*gnmi_ext.Extension) (*gnmi.CapabilityResponse, error) {
	// a.operLock.RLock()
	// defer a.operLock.RUnlock()
	if _, ok := a.Targets[tName]; !ok {
		err := a.initTarget(tName)
		if err != nil {
			return nil, err
		}
	}
	if t, ok := a.Targets[tName]; ok {
		if t.Client == nil {
			if err := t.CreateGNMIClient(ctx, a.dialOpts...); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return nil, fmt.Errorf("failed to create a gRPC client for target %q, timeout (%s) reached", t.Config.Name, t.Config.Timeout)
				}
				return nil, fmt.Errorf("failed to create a gRPC client for target %q : %v", t.Config.Name, err)
			}
		}
		ctx, cancel := context.WithTimeout(ctx, t.Config.Timeout)
		defer cancel()
		capResponse, err := t.Capabilities(ctx, ext...)
		if err != nil {
			return nil, fmt.Errorf("%q CapabilitiesRequest failed: %v", t.Config.Address, err)
		}
		return capResponse, nil
	}
	return nil, fmt.Errorf("unknown target name: %q", tName)
}

func (a *App) ClientGet(ctx context.Context, tName string, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	// a.operLock.RLock()
	// defer a.operLock.RUnlock()
	if _, ok := a.Targets[tName]; !ok {
		err := a.initTarget(tName)
		if err != nil {
			return nil, err
		}
	}
	if t, ok := a.Targets[tName]; ok {
		if t.Client == nil {
			if err := t.CreateGNMIClient(ctx, a.dialOpts...); err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					return nil, fmt.Errorf("failed to create a gRPC client for target %q, timeout (%s) reached", t.Config.Name, t.Config.Timeout)
				}
				return nil, fmt.Errorf("failed to create a gRPC client for target %q : %v", t.Config.Name, err)
			}
		}
		ctx, cancel := context.WithTimeout(ctx, t.Config.Timeout)
		defer cancel()
		getResponse, err := t.Get(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("%q GetRequest failed: %v", t.Config.Address, err)
		}
		return getResponse, nil
	}
	return nil, fmt.Errorf("unknown target name: %q", tName)
}

func (a *App) ClientSet(ctx context.Context, tName string, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	// a.operLock.RLock()
	// defer a.operLock.RUnlock()
	if _, ok := a.Targets[tName]; !ok {
		err := a.initTarget(tName)
		if err != nil {
			return nil, err
		}
	}
	if t, ok := a.Targets[tName]; ok {
		if t.Client == nil {
			if err := t.CreateGNMIClient(ctx, a.dialOpts...); err != nil {
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
	return nil, fmt.Errorf("unknown target name: %q", tName)
}
