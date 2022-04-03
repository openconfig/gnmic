package app

import (
	"context"
	"fmt"

	"github.com/karimra/gnmic/types"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
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
	err = a.CreateGNMIClient(ctx, t)
	a.operLock.RUnlock()
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
	err = a.CreateGNMIClient(ctx, t)
	a.operLock.RUnlock()
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
	err = a.CreateGNMIClient(ctx, t)
	a.operLock.RUnlock()
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, t.Config.Timeout)
	defer cancel()
	setResponse, err := t.Set(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("target %q SetRequest failed: %v", t.Config.Name, err)
	}
	return setResponse, nil
}
