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
```

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

Set `serviceAccountName: gnmic` in the collector Pod template. Existing installations
must grant EndpointSlice permissions before upgrading gNMIc; core/v1 Endpoints
permissions are no longer required by the locker. Lease permissions and renewal
settings are unchanged by this discovery migration.

Each active target has a Lease that is periodically renewed through the Kubernetes
API. Size the deployment using measured renewal latency and API request capacity,
and verify target ownership and leader recovery during Pod replacement.
