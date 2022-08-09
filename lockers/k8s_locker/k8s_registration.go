package k8s_locker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/karimra/gnmic/lockers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
)

const defaultWatchTimeout = 10 * time.Second

func (k *k8sLocker) Register(ctx context.Context, s *lockers.ServiceRegistration) error {
	return nil
}

func (k *k8sLocker) Deregister(s string) error {
	return nil
}

func (k *k8sLocker) WatchServices(ctx context.Context, serviceName string, tags []string, sChan chan<- []*lockers.Service, watchTimeout time.Duration) error {
	if watchTimeout <= 0 {
		watchTimeout = defaultWatchTimeout
	}
	resourceVersion := ""
	var err error
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			resourceVersion, err = k.watch(ctx, serviceName, tags, sChan, watchTimeout, resourceVersion)
			if err != nil {
				k.logger.Printf("watch ended with error: %s", err)
				time.Sleep(k.Cfg.RetryTimer)
			} else if k.Cfg.Debug {
				k.logger.Print("watch timed out")
			}
		}
	}
}

func (k *k8sLocker) watch(ctx context.Context, serviceName string, tags []string, sChan chan<- []*lockers.Service, watchTimeout time.Duration, resourceVersion string) (string, error) {
	timeoutSeconds := int64(watchTimeout.Seconds())
	listopts := metav1.ListOptions{
		FieldSelector:   fields.OneTermEqualSelector(metav1.ObjectNameField, serviceName).String(),
		ResourceVersion: resourceVersion,
		TimeoutSeconds:  &timeoutSeconds,
	}
	if k.Cfg.Debug {
		if resourceVersion == "" {
			k.logger.Print("starting watch beginning with unspecified resource version")
		} else {
			k.logger.Printf("starting watch beginning with resource version %s", resourceVersion)
		}
	}
	watched, err := k.clientset.CoreV1().Endpoints(k.Cfg.Namespace).Watch(ctx, listopts)
	if err != nil {
		return "", err
	}
	defer watched.Stop()

	watchChan := watched.ResultChan()

	for {
		event := <- watchChan
		switch event.Type {
		case watch.Modified, watch.Added:
			endpoints, ok := event.Object.(*corev1.Endpoints)
			if !ok {
				// this ought not to happen, but we should probably
				// start from scratch next time in case it does
				return "", fmt.Errorf("error converting watch result to an endpoint")
			}
			resourceVersion = endpoints.ResourceVersion
			if k.Cfg.Debug {
				k.logger.Printf("received watch event %s for resource version %s", event.Type, resourceVersion)
			}
			svcs, err := parseEndpoint(endpoints)
			if err != nil {
				return "", err
			}
			sChan <- svcs
		case "":
			// reached the timeout. return the version we last saw so
			// we can resume watching
			return resourceVersion, nil
		default:
			// something else happened, including maybe the object we
			// were watching being deleted. we'll need to start the
			// next watch from scratch, so don't return the resource
			// version
			return "", fmt.Errorf("unexpected watch event: %s", event.Type)
		}
	}
}

func parseEndpoint(endpoint *corev1.Endpoints) ([]*lockers.Service, error) {
	// the service should only have a single port number assigned, so
	// all subsets should have the port number we're looking for
	if len(endpoint.Subsets) <= 0 {
		return nil, fmt.Errorf("no subsets found in endpoint for service %s", endpoint.Name)
	}
	if len(endpoint.Subsets[0].Ports) <= 0 {
		return nil, fmt.Errorf("no ports found for service %s", endpoint.Name)
	}
	port := endpoint.Subsets[0].Ports[0].Port

	services := make([]*lockers.Service, 0, len(endpoint.Subsets[0].Addresses))
	for _, subset := range endpoint.Subsets {
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

func (k *k8sLocker) GetServices(ctx context.Context, serviceName string, tags []string) ([]*lockers.Service, error) {
	ep, err := k.clientset.CoreV1().Endpoints(k.Cfg.Namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return parseEndpoint(ep)
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
