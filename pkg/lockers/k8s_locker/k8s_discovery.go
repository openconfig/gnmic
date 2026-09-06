// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package k8s_locker

import (
	"context"
	"net"
	"reflect"
	"sort"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	discoverylisters "k8s.io/client-go/listers/discovery/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/openconfig/gnmic/pkg/lockers"
)

const defaultWatchTimeout = 10 * time.Second

func serviceSelector(serviceName string) string {
	return labels.Set{discoveryv1.LabelServiceName: serviceName}.String()
}

func (k *k8sLocker) GetServices(ctx context.Context, serviceName string, _ []string) ([]*lockers.Service, error) {
	list, err := k.clientset.DiscoveryV1().EndpointSlices(k.Cfg.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: serviceSelector(serviceName),
	})
	if err != nil {
		return nil, err
	}
	slices := make([]*discoveryv1.EndpointSlice, len(list.Items))
	for i := range list.Items {
		slices[i] = &list.Items[i]
	}
	return endpointSliceServices(slices), nil
}

func (k *k8sLocker) WatchServices(ctx context.Context, serviceName string, _ []string, sChan chan<- []*lockers.Service, watchTimeout time.Duration) error {
	if watchTimeout <= 0 {
		watchTimeout = defaultWatchTimeout
	}
	timeoutSeconds := max(int64(watchTimeout.Seconds()), 1)
	client := k.clientset.DiscoveryV1().EndpointSlices(k.Cfg.Namespace)
	source := &cache.ListWatch{
		ListWithContextFunc: func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
			opts.LabelSelector = serviceSelector(serviceName)
			return client.List(ctx, opts)
		},
		WatchFuncWithContext: func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
			opts.LabelSelector = serviceSelector(serviceName)
			opts.TimeoutSeconds = &timeoutSeconds
			return client.Watch(ctx, opts)
		},
	}
	informer := cache.NewSharedIndexInformer(cache.ToListWatcherWithWatchListSemantics(source, k.clientset), &discoveryv1.EndpointSlice{}, 0, cache.Indexers{})
	changes := make(chan struct{}, 1)
	notify := func() {
		select {
		case changes <- struct{}{}:
		default:
		}
	}
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(interface{}) { notify() },
		UpdateFunc: func(interface{}, interface{}) { notify() },
		DeleteFunc: func(interface{}) { notify() },
	})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		informer.Run(ctx.Done())
	}()
	defer func() {
		cancel()
		<-stopped
	}()
	if !cache.WaitForCacheSync(ctx.Done(), informer.HasSynced) {
		return ctx.Err()
	}
	lister := discoverylisters.NewEndpointSliceLister(informer.GetIndexer())
	var previous []*lockers.Service
	initial := true
	for {
		slices, err := lister.List(labels.Everything())
		if err != nil {
			return err
		}
		services := endpointSliceServices(slices)
		if initial || !reflect.DeepEqual(previous, services) {
			select {
			case sChan <- services:
				previous, initial = services, false
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changes:
		}
	}
}

func endpointSliceServices(slices []*discoveryv1.EndpointSlice) []*lockers.Service {
	peers := make(map[string]*lockers.Service)
	for _, slice := range slices {
		port := int32(0)
		for _, p := range slice.Ports {
			if p.Port != nil && *p.Port > 0 && (p.Protocol == nil || *p.Protocol == corev1.ProtocolTCP) {
				port = *p.Port
				break
			}
		}
		if port == 0 {
			continue
		}
		for _, endpoint := range slice.Endpoints {
			c := endpoint.Conditions
			if c.Ready != nil && !*c.Ready || c.Serving != nil && !*c.Serving || c.Terminating != nil && *c.Terminating {
				continue
			}
			for _, address := range endpoint.Addresses {
				if address == "" {
					continue
				}
				name := address
				if endpoint.TargetRef != nil && endpoint.TargetRef.Name != "" {
					name = endpoint.TargetRef.Name
				}
				peer := &lockers.Service{
					ID:      name + "-api",
					Address: net.JoinHostPort(address, strconv.Itoa(int(port))),
					Tags:    []string{"instance-name=" + name},
				}
				// A Pod can occur in overlapping or dual-stack slices; keep one stable API address.
				if previous, ok := peers[peer.ID]; !ok || peer.Address < previous.Address {
					peers[peer.ID] = peer
				}
			}
		}
	}
	services := make([]*lockers.Service, 0, len(peers))
	for _, peer := range peers {
		services = append(services, peer)
	}
	sort.Slice(services, func(i, j int) bool { return services[i].ID < services[j].ID })
	return services
}
