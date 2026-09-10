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
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/openconfig/gnmic/pkg/lockers"
)

func (k *k8sLocker) Register(ctx context.Context, s *lockers.ServiceRegistration) error {
	return nil
}

func (k *k8sLocker) Deregister(s string) error {
	return nil
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
