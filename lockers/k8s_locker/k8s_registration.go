package k8s_locker

import (
	"context"
	"time"

	"github.com/karimra/gnmic/lockers"
)

const defaultWatchTimeout = 1 * time.Minute

func (k *k8sLocker) Register(ctx context.Context, s *lockers.ServiceRegistration) error {
	return nil
}

func (k *k8sLocker) Deregister(s string) error {
	return nil
}

func (k *k8sLocker) WatchServices(ctx context.Context, serviceName string, tags []string, sChan chan<- []*lockers.Service, watchTimeout time.Duration) error {
	return nil
}

func (k *k8sLocker) GetServices(ctx context.Context, serviceName string, tags []string) ([]*lockers.Service, error) {
	return nil, nil
}

func (k *k8sLocker) IsLocked(ctx context.Context, key string) (bool, error) {
	return false, nil
}

func (k *k8sLocker) List(ctx context.Context, prefix string) (map[string]string, error) {
	return nil, nil
}
