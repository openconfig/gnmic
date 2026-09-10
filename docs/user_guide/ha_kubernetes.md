# Kubernetes locker

The `k8s` locker uses Kubernetes Leases for leader election and target ownership,
and EndpointSlices for peer discovery. gNMIc runs inside the cluster and uses its
Pod's ServiceAccount. The locker does not require a separate Redis or Consul service.

Configure the namespace containing both the gNMIc Pods and their API Service:

```yaml
api-server:
  address: :7890
clustering:
  cluster-name: telemetry
  instance-name: ${POD_NAME}
  locker:
    type: k8s
    namespace: telemetry
    lease-duration: 15s
    renew-deadline: 10s
    retry-period: 2s
    qps: 100
    burst: 200
```

These are the Lease lifecycle and Kubernetes client defaults. `renew-deadline`
defaults to two thirds of `lease-duration`. The Lease duration must be a positive
whole number of seconds. The renewal deadline must exceed `1.2 × retry-period`,
and their sum must be less than the Lease duration so collection can stop before
ownership expires.

Set `POD_NAME` from the Pod's `metadata.name` using the downward API. The instance
name must match the Pod name so that discovered peers match the instance names
stored with their Leases.
The API Service name is `<cluster-name>-gnmic-api`; its selector must match the
collector Pods. Expose a single TCP API port on this Service:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: telemetry-gnmic-api
  namespace: telemetry
spec:
  selector:
    app.kubernetes.io/name: gnmic
    app.kubernetes.io/instance: telemetry
  ports:
    - name: api
      port: 7890
      targetPort: api
      protocol: TCP
```

Use an API readiness probe so Kubernetes only advertises running API servers.
Discovery combines all `discovery.k8s.io/v1` EndpointSlices labeled
`kubernetes.io/service-name=telemetry-gnmic-api`. It excludes endpoints explicitly
marked not ready, not serving, or terminating. Unspecified readiness and serving
conditions are accepted. Empty results remove the previously discovered peers.
Duplicate Pod endpoints across slices produce one stable API address, including
when both IPv4 and IPv6 addresses are present.

## Lease lifecycle

The locker uses client-go leader election for acquisition and renewal, and a shared
Lease informer for ownership queries. Each acquisition has a unique holder identity;
annotations retain the original gNMIc lock key and instance name. `qps` and `burst`
set the client-go request budget per gNMIc instance. Healthy renewal normally uses
one Lease update per owned target during each retry period.

When renewal exceeds its deadline, `KeepLock` reports ownership loss so the collector
stops the target and retries acquisition. `Unlock` and shutdown release only Leases
owned by the current session, using UID and resource-version preconditions. Each
shutdown wait and API request has an independent timeout, and releases run with
bounded concurrency.

The client-go election algorithm does not fence arbitrary process pauses or clock-rate
skew. Size the Lease timings and request budget using measured API latency and the
maximum number of targets owned by a surviving instance.

Grant the ServiceAccount access to EndpointSlices and Leases in that namespace:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: gnmic
  namespace: telemetry
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: gnmic-locker
  namespace: telemetry
rules:
  - apiGroups: [discovery.k8s.io]
    resources: [endpointslices]
    verbs: [get, list, watch]
  - apiGroups: [coordination.k8s.io]
    resources: [leases]
    verbs: [get, list, watch, create, update, delete]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: gnmic-locker
  namespace: telemetry
subjects:
  - kind: ServiceAccount
    name: gnmic
    namespace: telemetry
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: gnmic-locker
```

Set `serviceAccountName: gnmic` in the collector Pod template. Core/v1 Endpoints
permissions are no longer required by the locker.

## Upgrade

Use this sequence for a rolling update:

1. Grant EndpointSlice and Lease `watch` permissions before updating gNMIc.
2. Keep the existing lock keys and the `renew-period` and `retry-timer` field names.
   Ensure those values satisfy the Lease timing constraints above, then roll out the
   new gNMIc version normally.
3. After every replica is running the new version, rename `renew-period` to
   `renew-deadline` and `retry-timer` to `retry-period`.

The deprecated fields remain accepted as aliases during the rollout. When an alias
and its replacement are both set, their values must match. Existing lock keys that
the previous locker could represent retain their slash-to-hyphen Lease names and
legacy labels, so old and new replicas coordinate through the same objects. Keys
that are not valid as both a Kubernetes Lease name and label key use a
`gnmic-<sha256>` name. Do not introduce these previously unsupported keys until every
replica is upgraded because older replicas cannot observe them.

Retaining legacy names also retains their slash-to-hyphen collision behavior. Verify
that existing lock keys remain unique after that replacement. Digest names avoid this
limitation for keys that the previous locker could not represent.
