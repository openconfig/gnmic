
The package `github.com/karimra/gnmic/api` exposes a set of `api.GNMIOption` that can be used with 
`api.NewGetRequest(...api.GNMIOption)`, `api.NewSetRequest(...api.GNMIOption)` or `api.NewSubscribeRequest(...api.GNMIOption)` to create a gNMI Request.

```golang
// Extention creates a GNMIOption that applies the suplied gnmi_ext.Extension to the provided
// proto.Message.
api.Extension(ext *gnmi_ext.Extension)
```

```golang
// Prefix creates a GNMIOption that creates a *gnmi.Path and adds it to the supplied 
// proto.Message (as a Path Prefix) which can be a 
// *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.SubscribeRequest with RequestType Subscribe.
api.Prefix(prefix string)
```

```golang
// Target creates a GNMIOption that set the gnmi Prefix target to the supplied string value.
// The proto.Message can be a *gnmi.GetRequest, *gnmi.SetRequest or 
// a *gnmi.SubscribeRequest with RequestType Subscribe.
api.Target(target string)
```

```golang
// Path creates a GNMIOption that creates a *gnmi.Path and adds it to the supplied proto.Message
// which can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.Subscription.
api.Path(path string)
```

```golang
// Encoding creates a GNMIOption that adds the encoding type to the supplied proto.Message
// which can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.SubscribeRequest with RequestType Subscribe.
api.Encoding(encoding string)
```

```golang
// DataType creates a GNMIOption that adds the data type to the supplied proto.Message
// which must be a *gnmi.GetRequest.
api.DataType(datat string)
```

```golang
// Update creates a GNMIOption that creates a *gnmi.Update message and adds it to the supplied proto.Message,
// the supplied message must be a *gnmi.SetRequest.
api.Update(opts ...GNMIOption)
```

```golang
// Replace creates a GNMIOption that creates a *gnmi.Update message and adds it to the supplied proto.Message.
// the supplied message must be a *gnmi.SetRequest.
api.Replace(opts ...GNMIOption)
```

```golang
// Value creates a GNMIOption that creates a *gnmi.TypedValue and adds it to the supplied proto.Message.
// the supplied message must be a *gnmi.Update.
api.Value(data, encoding string)
```

```golang
// Delete creates a GNMIOption that creates a *gnmi.Path and adds it to the supplied proto.Message.
// the supplied message must be a *gnmi.SetRequest. The *gnmi.Path is added the .Delete list.
api.Delete(path string)
```

```golang
// SubscriptionListMode creates a GNMIOption that sets the SubscribeRequest Mode.
// The variable mode must be one of "once", "poll" or "stream".
// The supplied proto.Message must be a *gnmi.SubscribeRequest with RequestType Subscribe.
api.SubscriptionListMode(mode string)
```

```golang
// Qos creates a GNMIOption that sets the QosMarking field in a 
// *gnmi.SubscribeRequest with RequestType Subscribe.
api.Qos(qos uint32)
```

```golang
// UseAliases creates a GNMIOption that sets the UsesAliases field in a 
// *gnmi.SubscribeRequest with RequestType Subscribe.
api.UseAliases(b bool)
```

```golang
// AllowAggregation creates a GNMIOption that sets the AllowAggregation field in a 
// *gnmi.SubscribeRequest with RequestType Subscribe.
api.AllowAggregation(b bool)
```

```golang
// UpdatesOnly creates a GNMIOption that sets the UpdatesOnly field in a 
// *gnmi.SubscribeRequest with RequestType Subscribe.
api.UpdatesOnly(b bool)
```

```golang
// UpdatesOnly creates a GNMIOption that creates a *gnmi.Subscription based on 
// the supplied GNMIOption(s) and adds it the supplied proto.Mesage which must be 
// of type *gnmi.SubscribeRequest with RequestType Subscribe.
api.Subscription(opts ...GNMIOption)
```

```golang
// SubscriptionMode creates a GNMIOption that sets the Subscription mode in a 
// proto.Message of type *gnmi.Subscription.
api.SubscriptionMode(mode string)
```

```golang
// SampleInterval creates a GNMIOption that sets the SampleInterval in a 
// proto.Message of type *gnmi.Subscription.
api.SampleInterval(d time.Duration)
```

```golang
// HeartbeatInterval creates a GNMIOption that sets the HeartbeatInterval in a 
// proto.Message of type *gnmi.Subscription.
api.HeartbeatInterval(d time.Duration)
```

```golang
// SuppressRedundant creates a GNMIOption that sets the SuppressRedundant in a 
// proto.Message of type *gnmi.Subscription.
api.SuppressRedundant(s bool)
```
