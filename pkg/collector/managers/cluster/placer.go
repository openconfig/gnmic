package cluster_manager

import (
	"context"
	"log/slog"

	"github.com/openconfig/gnmic/pkg/lockers"
)

type target struct {
	Name   string
	Labels map[string]string
}

type Placer interface {
	// places targets on members based on the labels
	// returns a map of `targetName -> member`
	Place(ctx context.Context, targets map[string]target, members map[string]*Member) (map[string]*Member, error)
}

type placer struct {
	locker lockers.Locker
	logger *slog.Logger
}

func NewPlacer(locker lockers.Locker, logger *slog.Logger) Placer {
	return &placer{locker: locker, logger: logger}
}

func (p *placer) Place(ctx context.Context, targets map[string]target, members map[string]*Member) (map[string]*Member, error) {
	return nil, nil
}
