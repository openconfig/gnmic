// SPDX-License-Identifier: Apache-2.0

package k8s_locker

import (
	"context"
	"fmt"
	"strings"
	"time"

	coordinationv1 "k8s.io/api/coordination/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	coordinationlisters "k8s.io/client-go/listers/coordination/v1"
	"k8s.io/client-go/tools/cache"
)

const leaseKeyDirIndex = "original-key-dir"

// leaseKeyDirs indexes a Lease under each prefix of its original key that ends before a "/".
func leaseKeyDirs(obj any) ([]string, error) {
	lease, ok := obj.(*coordinationv1.Lease)
	if !ok {
		return nil, fmt.Errorf("expected Kubernetes Lease, got %T", obj)
	}
	key := lease.Annotations[origKeyName]
	var dirs []string
	for i := range len(key) {
		if key[i] == '/' {
			dirs = append(dirs, key[:i])
		}
	}
	return dirs, nil
}

func (k *k8sLocker) startLeaseCache(ctx context.Context) error {
	client := k.clientset.CoordinationV1().Leases(k.Cfg.Namespace)
	source := &cache.ListWatch{
		ListWithContextFunc: func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
			opts.LabelSelector = "app=gnmic"
			return client.List(ctx, opts)
		},
		WatchFuncWithContext: func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
			opts.LabelSelector = "app=gnmic"
			return client.Watch(ctx, opts)
		},
	}
	informer := cache.NewSharedIndexInformer(cache.ToListWatcherWithWatchListSemantics(source, k.clientset), &coordinationv1.Lease{}, 0, cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
		leaseKeyDirIndex:     leaseKeyDirs,
	})
	k.leases = coordinationlisters.NewLeaseLister(informer.GetIndexer()).Leases(k.Cfg.Namespace)
	k.leaseIndex = informer.GetIndexer()
	ctx, k.stopCache = context.WithCancel(ctx)
	k.cacheDone = make(chan struct{})
	go func() {
		defer close(k.cacheDone)
		informer.Run(ctx.Done())
	}()
	initial, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if !cache.WaitForCacheSync(initial.Done(), informer.HasSynced) {
		k.stopCache()
		<-k.cacheDone
		return fmt.Errorf("initial Kubernetes Lease list failed: %w", initial.Err())
	}
	return nil
}

func validLease(lease *coordinationv1.Lease, now time.Time) bool {
	spec := lease.Spec
	return spec.HolderIdentity != nil && *spec.HolderIdentity != "" &&
		spec.RenewTime != nil && spec.LeaseDurationSeconds != nil && *spec.LeaseDurationSeconds > 0 &&
		spec.RenewTime.Add(time.Duration(*spec.LeaseDurationSeconds)*time.Second).After(now)
}

func (k *k8sLocker) IsLocked(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	lease, err := k.leases.Get(leaseName(key))
	if errors.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return validLease(lease, time.Now()), nil
}

func (k *k8sLocker) List(ctx context.Context, prefix string) (map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	objects, err := k.leaseCandidates(prefix)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	now := time.Now()
	for _, object := range objects {
		lease := object.(*coordinationv1.Lease)
		key, present := lease.Annotations[origKeyName]
		if !present || !strings.HasPrefix(key, prefix) {
			continue
		}
		if value, ok := leaseValue(lease); ok && validLease(lease, now) {
			result[key] = value
		}
	}
	return result, nil
}

// leaseCandidates returns the cached Leases that may match prefix. Every key that starts
// with prefix is indexed under the part of prefix before its last "/"; a prefix without
// a "/" scans the cache.
func (k *k8sLocker) leaseCandidates(prefix string) ([]any, error) {
	dir := strings.LastIndexByte(prefix, '/')
	if dir < 0 {
		return k.leaseIndex.List(), nil
	}
	return k.leaseIndex.ByIndex(leaseKeyDirIndex, prefix[:dir])
}
