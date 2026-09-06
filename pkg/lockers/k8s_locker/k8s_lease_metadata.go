// SPDX-License-Identifier: Apache-2.0

package k8s_locker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"maps"

	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coordinationclient "k8s.io/client-go/kubernetes/typed/coordination/v1"
)

func leaseName(key string) string {
	digest := sha256.Sum256([]byte(key))
	return "gnmic-" + hex.EncodeToString(digest[:])
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
