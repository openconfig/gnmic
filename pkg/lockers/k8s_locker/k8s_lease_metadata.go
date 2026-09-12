// SPDX-License-Identifier: Apache-2.0

package k8s_locker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"maps"
	"strings"

	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	coordinationclient "k8s.io/client-go/kubernetes/typed/coordination/v1"
)

func legacyLeaseName(key string) string {
	return strings.ReplaceAll(key, "/", "-")
}

func leaseName(key string) string {
	legacy := legacyLeaseName(key)
	if len(validation.IsDNS1123Subdomain(legacy)) == 0 && len(validation.IsQualifiedName(legacy)) == 0 {
		return legacy
	}
	digest := sha256.Sum256([]byte(key))
	return "gnmic-" + hex.EncodeToString(digest[:])
}

func leaseLabels(key string, value []byte) map[string]string {
	labels := map[string]string{"app": "gnmic"}
	legacy := legacyLeaseName(key)
	if leaseName(key) == legacy && len(validation.IsValidLabelValue(string(value))) == 0 {
		labels[legacy] = string(value)
	}
	return labels
}

func leaseValue(lease *coordinationv1.Lease) (string, bool) {
	if value, ok := lease.Annotations[origValueName]; ok {
		return value, true
	}
	key, ok := lease.Annotations[origKeyName]
	if !ok {
		return "", false
	}
	value, ok := lease.Labels[legacyLeaseName(key)]
	return value, ok
}

// LeaseLock exposes labels but not annotations; decorate its typed client to retain the original key and value atomically.
type annotatedLeaseGetter struct {
	coordinationclient.LeasesGetter
	annotations map[string]string
}

func (g annotatedLeaseGetter) Leases(namespace string) coordinationclient.LeaseInterface {
	return annotatedLeases{LeaseInterface: g.LeasesGetter.Leases(namespace), annotations: g.annotations}
}

type annotatedLeases struct {
	coordinationclient.LeaseInterface
	annotations map[string]string
}

func (l annotatedLeases) annotated(lease *coordinationv1.Lease) *coordinationv1.Lease {
	copy := lease.DeepCopy()
	if copy.Annotations == nil {
		copy.Annotations = make(map[string]string)
	}
	maps.Copy(copy.Annotations, l.annotations)
	return copy
}

func (l annotatedLeases) Create(ctx context.Context, lease *coordinationv1.Lease, opts metav1.CreateOptions) (*coordinationv1.Lease, error) {
	return l.LeaseInterface.Create(ctx, l.annotated(lease), opts)
}

func (l annotatedLeases) Update(ctx context.Context, lease *coordinationv1.Lease, opts metav1.UpdateOptions) (*coordinationv1.Lease, error) {
	return l.LeaseInterface.Update(ctx, l.annotated(lease), opts)
}
