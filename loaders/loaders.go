package loaders

import (
	"context"
	"log"

	"github.com/karimra/gnmic/types"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"
)

type TargetLoader interface {
	Init(context.Context, map[string]interface{}, *log.Logger, ...Option) error
	RunOnce(ctx context.Context) (map[string]*types.TargetConfig, error)
	Start(context.Context) chan *TargetOperation
	RegisterMetrics(*prometheus.Registry)
	WithActions(map[string]map[string]interface{})
	WithTargetsDefaults(func(tc *types.TargetConfig) error)
}

type Initializer func() TargetLoader

var Loaders = map[string]Initializer{}

var LoadersTypes = []string{
	"file",
	"consul",
	"docker",
	"http",
}

func Register(name string, initFn Initializer) {
	Loaders[name] = initFn
}

type TargetOperation struct {
	Add []*types.TargetConfig
	Del []string
}

func DecodeConfig(src, dst interface{}) error {
	decoder, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{
			DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
			Result:     dst,
		},
	)
	if err != nil {
		return err
	}
	return decoder.Decode(src)
}

func Diff(m1, m2 map[string]*types.TargetConfig) *TargetOperation {
	result := &TargetOperation{
		Add: make([]*types.TargetConfig, 0),
		Del: make([]string, 0),
	}
	if len(m1) == 0 {
		for _, t := range m2 {
			result.Add = append(result.Add, t)
		}
		return result
	}
	if len(m2) == 0 {
		for name := range m1 {
			result.Del = append(result.Del, name)
		}
		return result
	}
	for n, t := range m2 {
		if _, ok := m1[n]; !ok {
			result.Add = append(result.Add, t)
		}
	}
	for n := range m1 {
		if _, ok := m2[n]; !ok {
			result.Del = append(result.Del, n)
		}
	}
	return result
}
