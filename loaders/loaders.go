package loaders

import (
	"context"

	"github.com/karimra/gnmic/collector"
	"github.com/mitchellh/mapstructure"
)

type TargetLoader interface {
	Init(context.Context, map[string]interface{}) error
	Start(context.Context) chan *TargetOperation
}

type Initializer func() TargetLoader

var Loaders = map[string]Initializer{}

var LoadersTypes = []string{
	"file",
	"consul",
}

func Register(name string, initFn Initializer) {
	Loaders[name] = initFn
}

type TargetOperation struct {
	Add []*collector.TargetConfig
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

func Diff(m1, m2 map[string]*collector.TargetConfig) *TargetOperation {
	result := &TargetOperation{
		Add: make([]*collector.TargetConfig, 0),
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
