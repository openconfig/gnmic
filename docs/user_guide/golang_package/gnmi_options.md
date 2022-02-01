
The package `github.com/karimra/gnmic/api` exposes a set of `api.GNMIOption` that can be used with 
`api.NewGetRequest(...api.GNMIOption) GNMIOption`, `api.NewSetRequest(...api.GNMIOption) GNMIOption` or `api.NewSubscribeRequest(...api.GNMIOption) GNMIOption` to create a gNMI Request.

```golang
// Extention creates a GNMIOption that applies the suplied gnmi_ext.Extension to the provided
// proto.Message.
func Extension(ext *gnmi_ext.Extension) GNMIOption
```

```golang
// Prefix creates a GNMIOption that creates a *gnmi.Path and adds it to the supplied 
// proto.Message (as a Path Prefix) GNMIOption which can be a 
// *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.SubscribeRequest with RequestType Subscribe.
func Prefix(prefix string) GNMIOption
```

```golang
// Target creates a GNMIOption that set the gnmi Prefix target to the supplied string value.
// The proto.Message can be a *gnmi.GetRequest, *gnmi.SetRequest or 
// a *gnmi.SubscribeRequest with RequestType Subscribe.
func Target(target string) GNMIOption
```

```golang
// Path creates a GNMIOption that creates a *gnmi.Path and adds it to the supplied proto.Message
// which can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.Subscription.
func Path(path string) GNMIOption
```

```golang
// Encoding creates a GNMIOption that adds the encoding type to the supplied proto.Message
// which can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.SubscribeRequest with RequestType Subscribe.
func Encoding(encoding string) GNMIOption
```

```golang
// DataType creates a GNMIOption that adds the data type to the supplied proto.Message
// which must be a *gnmi.GetRequest.
func DataType(datat string) GNMIOption
```

```golang
// Update creates a GNMIOption that creates a *gnmi.Update message and adds it to the supplied proto.Message,
// the supplied message must be a *gnmi.SetRequest.
func Update(opts ...GNMIOption) GNMIOption
```

```golang
// Replace creates a GNMIOption that creates a *gnmi.Update message and adds it to the supplied proto.Message.
// the supplied message must be a *gnmi.SetRequest.
func Replace(opts ...GNMIOption) GNMIOption
```

```golang
// Value creates a GNMIOption that creates a *gnmi.TypedValue and adds it to the supplied proto.Message.
// the supplied message must be a *gnmi.Update.
func Value(data, encoding string) GNMIOption
```

```golang
// Delete creates a GNMIOption that creates a *gnmi.Path and adds it to the supplied proto.Message.
// the supplied message must be a *gnmi.SetRequest. The *gnmi.Path is added the .Delete list.
func Delete(path string) GNMIOption
```

```golang
// SubscriptionListMode creates a GNMIOption that sets the SubscribeRequest Mode.
// The variable mode must be one of "once", "poll" or "stream".
// The supplied proto.Message must be a *gnmi.SubscribeRequest with RequestType Subscribe.
func SubscriptionListMode(mode string) GNMIOption
```

```golang
// Qos creates a GNMIOption that sets the QosMarking field in a 
// *gnmi.SubscribeRequest with RequestType Subscribe.
func Qos(qos uint32) GNMIOption
```

```golang
// UseAliases creates a GNMIOption that sets the UsesAliases field in a 
// *gnmi.SubscribeRequest with RequestType Subscribe.
func UseAliases(b bool) GNMIOption
```

```golang
// AllowAggregation creates a GNMIOption that sets the AllowAggregation field in a 
// *gnmi.SubscribeRequest with RequestType Subscribe.
func AllowAggregation(b bool) GNMIOption
```

```golang
// UpdatesOnly creates a GNMIOption that sets the UpdatesOnly field in a 
// *gnmi.SubscribeRequest with RequestType Subscribe.
func UpdatesOnly(b bool) GNMIOption
```

```golang
// UpdatesOnly creates a GNMIOption that creates a *gnmi.Subscription based on 
// the supplied GNMIOption(s) GNMIOption and adds it the supplied proto.Mesage which must be 
// of type *gnmi.SubscribeRequest with RequestType Subscribe.
func Subscription(opts ...GNMIOption) GNMIOption
```

```golang
// SubscriptionMode creates a GNMIOption that sets the Subscription mode in a 
// proto.Message of type *gnmi.Subscription.
func SubscriptionMode(mode string) GNMIOption
```

```golang
// SampleInterval creates a GNMIOption that sets the SampleInterval in a 
// proto.Message of type *gnmi.Subscription.
func SampleInterval(d time.Duration) GNMIOption
```

```golang
// HeartbeatInterval creates a GNMIOption that sets the HeartbeatInterval in a 
// proto.Message of type *gnmi.Subscription.
func HeartbeatInterval(d time.Duration) GNMIOption
```

```golang
// SuppressRedundant creates a GNMIOption that sets the SuppressRedundant in a 
// proto.Message of type *gnmi.Subscription.
func SuppressRedundant(s bool) GNMIOption
```
