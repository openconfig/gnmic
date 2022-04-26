package k8s_locker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/karimra/gnmic/lockers"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
	ksrv, err := k.clientset.CoreV1().Services(k.Cfg.Namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if len(ksrv.Spec.Ports) == 0 {
		return nil, fmt.Errorf("missing ports in service %s", serviceName)
	}
	portNum := ksrv.Spec.Ports[0].Port

	set := labels.Set(ksrv.Spec.Selector)
	pods, err := k.clientset.CoreV1().Pods(k.Cfg.Namespace).List(ctx, metav1.ListOptions{LabelSelector: set.AsSelector().String()})
	if err != nil {
		return nil, err
	}

	services := make([]*lockers.Service, 0, len(pods.Items))
	for _, pod := range pods.Items {
		if pod.Status.PodIP == "" {
			continue
		}
		ls := &lockers.Service{
			ID:      fmt.Sprintf("%s-api", pod.Name),
			Address: fmt.Sprintf("%s:%d", pod.Status.PodIP, portNum),
			Tags: []string{
				fmt.Sprintf("instance-name=%s", pod.Name),
			},
		}
		services = append(services, ls)
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
