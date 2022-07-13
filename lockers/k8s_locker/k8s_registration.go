package k8s_locker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/karimra/gnmic/lockers"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultWatchTimeout = 10 * time.Second

func (k *k8sLocker) Register(ctx context.Context, s *lockers.ServiceRegistration) error {
	return nil
}

func (k *k8sLocker) Deregister(s string) error {
	return nil
}

func (k *k8sLocker) WatchServices(ctx context.Context, serviceName string, tags []string, sChan chan<- []*lockers.Service, _ time.Duration) error {
	// if watchTimeout <= 0 {
	watchTimeout := defaultWatchTimeout
	// }
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			services, err := k.GetServices(ctx, serviceName, tags)
			if err != nil {
				return err
			}
			sChan <- services
			time.Sleep(watchTimeout)
		}
	}
}

func (k *k8sLocker) GetServices(ctx context.Context, serviceName string, tags []string) ([]*lockers.Service, error) {
	ep, err := k.clientset.CoreV1().Endpoints(k.Cfg.Namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// the service should only have a single port number assigned, so
	// all subsets should have the port number we're looking for
	if len(ep.Subsets) <= 0 {
		return nil, fmt.Errorf("no subsets found in endpoint for service %s", serviceName)
	}
	if len(ep.Subsets[0].Ports) <= 0 {
		return nil, fmt.Errorf("no ports found for service %s", serviceName)
	}
	port := ep.Subsets[0].Ports[0].Port

	services := make([]*lockers.Service, 0, len(ep.Subsets[0].Addresses))
	for _, subset := range ep.Subsets {
		for _, addr := range subset.Addresses {
			targetName := addr.IP
			if addr.TargetRef != nil {
				targetName = addr.TargetRef.Name
			}
			ls := &lockers.Service{
				ID:      fmt.Sprintf("%s-api", targetName),
				Address: fmt.Sprintf("%s:%d", addr.IP, port),
				Tags: []string{
					fmt.Sprintf("instance-name=%s", targetName),
				},
			}
			services = append(services, ls)
		}
	}

	return services, nil
}

func (k *k8sLocker) IsLocked(ctx context.Context, key string) (bool, error) {
	key = strings.ReplaceAll(key, "/", "-")
	ol, err := k.clientset.CoordinationV1().Leases(k.Cfg.Namespace).Get(ctx, key, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if ol == nil {
		return false, nil
	}
	if ol.Spec.RenewTime == nil {
		return false, nil
	}
	now := metav1.NowMicro()
	expectedRenewTime := ol.Spec.RenewTime.Add(time.Duration(*ol.Spec.LeaseDurationSeconds) * time.Second)
	return expectedRenewTime.After(now.Time), nil
}

func (k *k8sLocker) List(ctx context.Context, prefix string) (map[string]string, error) {
	ll, err := k.clientset.CoordinationV1().Leases(k.Cfg.Namespace).List(ctx,
		metav1.ListOptions{
			LabelSelector: "app=gnmic",
		})
	if err != nil {
		return nil, err
	}

	prefix = strings.ReplaceAll(prefix, "/", "-")
	rs := make(map[string]string, len(ll.Items))
	for _, l := range ll.Items {
		for key, v := range l.Labels {
			if key == "app" {
				continue
			}
			if strings.HasPrefix(key, prefix) {
				okey, ok := l.Annotations[origKeyName]
				if ok {
					rs[okey] = v
					continue
				}
			}
		}
	}
	return rs, nil
}
