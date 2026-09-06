# Kubernetes Lease locker

The `k8s` locker uses Kubernetes Leases for cluster leadership and target ownership.
It authenticates with the Pod's ServiceAccount through the in-cluster client configuration.
It uses client-go leader election for acquisition and renewal, and a shared informer
for ownership queries. Each acquisition has a unique holder identity; annotations
retain the original gNMIc lock key and instance name.

```yaml
clustering:
  locker:
    type: k8s
    namespace: default
    lease-duration: 15s
    renew-deadline: 10s
    retry-period: 2s
    qps: 100
    burst: 200
```

These are the defaults. `renew-deadline` defaults to two thirds of
`lease-duration`. The lease duration must be a positive whole number of seconds.
The renewal deadline must exceed `1.2 × retry-period`, and their sum must be less
than the lease duration to leave time for collection to stop after renewal failure.

`qps` and `burst` configure the client-go request limiter per gNMIc instance.
Healthy renewal normally uses one UPDATE per owned Lease per retry period.
Size the budget for the maximum number of targets per surviving instance, with
headroom for acquisition, retries and discovery. For example, 107 targets and
one cluster leader Lease at a two-second retry period need about 54 writes/s.
The Kubernetes API server must also have capacity for this aggregate write load.

When renewal exceeds its deadline, the locker reports ownership loss through
`KeepLock` so the collector stops the target and retries acquisition. Leases are
released explicitly by `Unlock` or shutdown, using holder identity and Kubernetes
UID/resourceVersion preconditions. Release requests are bounded by `retry-period`;
subscription cancellation and other targets' cleanup proceed independently of
release I/O. The underlying
[client-go election algorithm](https://pkg.go.dev/k8s.io/client-go/tools/leaderelection)
does not provide fencing against arbitrary process pauses or clock-rate skew.

## Lease permissions

Add the following rule to the Role bound to the gNMIc ServiceAccount in its locker
namespace. Service discovery requires its own resource permissions.

```yaml
apiGroups: [coordination.k8s.io]
resources: [leases]
verbs: [get, list, watch, create, update, delete]
```

Startup waits for the initial Lease cache to synchronize. Failure to read the
initial list prevents locker initialization; `watch` permission is required
to keep ownership queries current.
Ownership queries exclude expired Leases and preserve the original key prefix.

## Upgrading the previous Kubernetes locker

The resource name is now `gnmic-` followed by the SHA-256 digest of the original
key. This supports case-sensitive, colon-containing and long target identities
without lossy slash replacement or Kubernetes label restrictions.

All members sharing a cluster must stop before this upgrade. The previous and
new encodings refer to different Lease objects, so a mixed-version rolling
upgrade would create independent ownership domains. After all old members have
stopped, apply the new configuration and RBAC, then start the upgraded members.
Obsolete Lease objects can be removed after confirming their holders have stopped.

Replace the previous `renew-period` and `retry-timer` locker fields with
`renew-deadline` and `retry-period`. The previous fields are rejected to make an
incomplete configuration migration visible. The renewal deadline bounds API
failure handling; the retry period controls the interval between renewal attempts.
