package utils

import (
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/store"
)

func GetConfigMaps(s store.Store[any]) (map[string]*types.TargetConfig, map[string]map[string]any, map[string]map[string]any, error) {
	tgm, err := s.List("targets")
	if err != nil {
		return nil, nil, nil, err
	}
	tcs := make(map[string]*types.TargetConfig)
	for n, t := range tgm {
		if tc, ok := t.(*types.TargetConfig); ok {
			tcs[n] = tc
		}
	}
	egm, err := s.List("processors")
	if err != nil {
		return nil, nil, nil, err
	}
	eps := make(map[string]map[string]any)
	for n, e := range egm {
		if ep, ok := e.(map[string]any); ok {
			eps[n] = ep
		}
	}
	agm, err := s.List("actions")
	if err != nil {
		return nil, nil, nil, err
	}
	acts := make(map[string]map[string]any)
	for n, a := range agm {
		if act, ok := a.(map[string]any); ok {
			acts[n] = act
		}
	}
	return tcs, eps, acts, nil
}
