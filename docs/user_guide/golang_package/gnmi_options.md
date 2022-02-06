
The package `github.com/karimra/gnmic/api` exposes a set of `api.GNMIOption` that can be used with 
`api.NewGetRequest(...api.GNMIOption) GNMIOption`, `api.NewSetRequest(...api.GNMIOption) GNMIOption` or `api.NewSubscribeRequest(...api.GNMIOption) GNMIOption` to create a gNMI Request.

```golang
// Version sets the provided gNMI version string in a gnmi.CapabilityResponse message.
func Version(v string) func(msg proto.Message) error
```

```golang
// SupportedEncoding creates an GNMIOption that sets the provided encodings as supported encodings in a gnmi.CapabilitiesResponse
func SupportedEncoding(encodings ...string) func(msg proto.Message) error
```

```golang
// SupportedModel creates an GNMIOption that sets the provided name, org and version as a supported model in a gnmi.CapabilitiesResponse.
func SupportedModel(name, org, version string) func(msg proto.Message) error
```

```golang
// Extension creates a GNMIOption that applies the supplied gnmi_ext.Extension to the provided
// proto.Message.
func Extension(ext *gnmi_ext.Extension) func(msg proto.Message) error
```

```golang
// Prefix creates a GNMIOption that creates a *gnmi.Path and adds it to the supplied
// proto.Message (as a Path Prefix).
// The proto.Message can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.SubscribeRequest with RequestType Subscribe.
func Prefix(prefix string) func(msg proto.Message) error
```

```golang
// Target creates a GNMIOption that set the gnmi Prefix target to the supplied string value.
// The proto.Message can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.SubscribeRequest with RequestType Subscribe.
func Target(target string) func(msg proto.Message) error
```

```golang
// Path creates a GNMIOption that creates a *gnmi.Path and adds it to the supplied proto.Message
// which can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.Subscription.
func Path(path string) func(msg proto.Message) error
```

```golang
// Encoding creates a GNMIOption that adds the encoding type to the supplied proto.Message
// which can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.SubscribeRequest with RequestType Subscribe.
func Encoding(encoding string) func(msg proto.Message) error
```

```golang
// EncodingJSON creates a GNMIOption that sets the encoding type to JSON in a gnmi.GetRequest or
// gnmi.SubscribeRequest.
func EncodingJSON() func(msg proto.Message) error
```

```golang
// EncodingBytes creates a GNMIOption that sets the encoding type to BYTES in a gnmi.GetRequest or
// gnmi.SubscribeRequest.
func EncodingBytes() func(msg proto.Message) error
```

```golang
// EncodingPROTO creates a GNMIOption that sets the encoding type to PROTO in a gnmi.GetRequest or
// gnmi.SubscribeRequest.
func EncodingPROTO() func(msg proto.Message) error
```

```golang
// EncodingASCII creates a GNMIOption that sets the encoding type to ASCII in a gnmi.GetRequest or
// gnmi.SubscribeRequest.
func EncodingASCII() func(msg proto.Message) error
```

```golang
// EncodingJSON_IETF creates a GNMIOption that sets the encoding type to JSON_IETF in a gnmi.GetRequest or
// gnmi.SubscribeRequest.
func EncodingJSON_IETF() func(msg proto.Message) error
```

```golang
// EncodingCustom creates a GNMIOption that adds the encoding type to the supplied proto.Message
// which can be a *gnmi.GetRequest, *gnmi.SetRequest or a *gnmi.SubscribeRequest with RequestType Subscribe.
// Unlike Encoding, this GNMIOption does not validate if the provided encoding is defined by the gNMI spec.
func EncodingCustom(enc int) func(msg proto.Message) error
```

```golang
// DataType creates a GNMIOption that adds the data type to the supplied proto.Message
// which must be a *gnmi.GetRequest.
func DataType(datat string) func(msg proto.Message) error
```

```golang
// DataTypeALL creates a GNMIOption that sets the gnmi.GetRequest data type to ALL
func DataTypeALL() func(msg proto.Message) error
```

```golang
// DataTypeCONFIG creates a GNMIOption that sets the gnmi.GetRequest data type to CONFIG
func DataTypeCONFIG() func(msg proto.Message) error
```

```golang
// DataTypeSTATE creates a GNMIOption that sets the gnmi.GetRequest data type to STATE
func DataTypeSTATE() func(msg proto.Message) error
```

```golang
// DataTypeOPERATIONAL creates a GNMIOption that sets the gnmi.GetRequest data type to OPERATIONAL
func DataTypeOPERATIONAL() func(msg proto.Message) error
```

```golang
// UseModel creates a GNMIOption that add a gnmi.DataModel to a gnmi.GetRequest or gnmi.SubscribeRequest
// based on the name, org and version strings provided.
func UseModel(name, org, version string) func(msg proto.Message) error
```

```golang
// Update creates a GNMIOption that creates a *gnmi.Update message and adds it to the supplied proto.Message,
// the supplied message must be a *gnmi.SetRequest.
func Update(opts ...GNMIOption) func(msg proto.Message) error
```

```golang
// Replace creates a GNMIOption that creates a *gnmi.Update message and adds it to the supplied proto.Message.
// the supplied message must be a *gnmi.SetRequest.
func Replace(opts ...GNMIOption) func(msg proto.Message) error
```

```golang
// Value creates a GNMIOption that creates a *gnmi.TypedValue and adds it to the supplied proto.Message.
// the supplied message must be a *gnmi.Update.
// If a map is supplied as `data interface{}` it has to be a map[string]interface{}.
func Value(data interface{}, encoding string) func(msg proto.Message) error
```

```golang
// Delete creates a GNMIOption that creates a *gnmi.Path and adds it to the supplied proto.Message.
// the supplied message must be a *gnmi.SetRequest. The *gnmi.Path is added the .Delete list.
func Delete(path string) func(msg proto.Message) error
```

```golang
// SubscriptionListMode creates a GNMIOption that sets the SubscribeRequest Mode.
// The variable mode must be one of "once", "poll" or "stream".
// The supplied proto.Message must be a *gnmi.SubscribeRequest with RequestType Subscribe.
func SubscriptionListMode(mode string) func(msg proto.Message) error
```

```golang
// SubscriptionListModeSTREAM creates a GNMIOption that sets the Subscription List Mode to STREAM
func SubscriptionListModeSTREAM() func(msg proto.Message) error
```

```golang
// SubscriptionListModeONCE creates a GNMIOption that sets the Subscription List Mode to ONCE
func SubscriptionListModeONCE() func(msg proto.Message) error
```

```golang
// SubscriptionListModePOLL creates a GNMIOption that sets the Subscription List Mode to POLL
func SubscriptionListModePOLL() func(msg proto.Message) error
```

```golang
// Qos creates a GNMIOption that sets the QosMarking field in a *gnmi.SubscribeRequest with RequestType Subscribe.
func Qos(qos uint32) func(msg proto.Message) error
```

```golang
// UseAliases creates a GNMIOption that sets the UsesAliases field in a *gnmi.SubscribeRequest with RequestType Subscribe.
func UseAliases(b bool) func(msg proto.Message) error
```

```golang
// AllowAggregation creates a GNMIOption that sets the AllowAggregation field in a *gnmi.SubscribeRequest with RequestType Subscribe.
func AllowAggregation(b bool) func(msg proto.Message) error
```

```golang
// UpdatesOnly creates a GNMIOption that sets the UpdatesOnly field in a *gnmi.SubscribeRequest with RequestType Subscribe.
func UpdatesOnly(b bool) func(msg proto.Message) error
```

```golang
// UpdatesOnly creates a GNMIOption that creates a *gnmi.Subscription based on the supplied GNMIOption(s) and adds it the
// supplied proto.Message which must be of type *gnmi.SubscribeRequest with RequestType Subscribe.
func Subscription(opts ...GNMIOption) func(msg proto.Message) error
```

```golang
// SubscriptionMode creates a GNMIOption that sets the Subscription mode in a proto.Message of type *gnmi.Subscription.
func SubscriptionMode(mode string) func(msg proto.Message) error
```

```golang
// SubscriptionModeTARGET_DEFINED creates a GNMIOption that sets the subscription mode to TARGET_DEFINED
func SubscriptionModeTARGET_DEFINED() func(msg proto.Message) error
```

```golang
// SubscriptionModeON_CHANGE creates a GNMIOption that sets the subscription mode to ON_CHANGE
func SubscriptionModeON_CHANGE() func(msg proto.Message) error
```

```golang
// SubscriptionModeSAMPLE creates a GNMIOption that sets the subscription mode to SAMPLE
func SubscriptionModeSAMPLE() func(msg proto.Message) error
```

```golang
// SampleInterval creates a GNMIOption that sets the SampleInterval in a proto.Message of type *gnmi.Subscription.
func SampleInterval(d time.Duration) func(msg proto.Message) error
```

```golang
// HeartbeatInterval creates a GNMIOption that sets the HeartbeatInterval in a proto.Message of type *gnmi.Subscription.
func HeartbeatInterval(d time.Duration) func(msg proto.Message) error
```

```golang
// SuppressRedundant creates a GNMIOption that sets the SuppressRedundant in a proto.Message of type *gnmi.Subscription.
func SuppressRedundant(s bool) func(msg proto.Message) error
```

```golang
// Notification creates a GNMIOption that builds a gnmi.Notification from the supplied GNMIOptions and adds it
// to the supplied proto.Message
func Notification(opts ...GNMIOption) func(msg proto.Message) error
```

```golang
// Timestamp sets the supplied timestamp in a gnmi.Notification message
func Timestamp(t int64) func(msg proto.Message) error
```

```golang
// TimestampNow is the same as Timestamp(time.Now().UnixNano())
func TimestampNow() func(msg proto.Message) error
```

```golang
// Alias sets the supplied alias value in a gnmi.Notification message
func Alias(alias string) func(msg proto.Message) error
```

```golang
// Atomic sets the .Atomic field in a gnmi.Notification message
func Atomic(b bool) func(msg proto.Message) error
```

```golang
// UpdateResult creates a GNMIOption that creates a gnmi.UpdateResult and adds it to
// a proto.Message of type gnmi.SetResponse.
func UpdateResult(opts ...GNMIOption) func(msg proto.Message) error
```

```golang
// Operation creates a GNMIOption that sets the gnmi.UpdateResult_Operation
// value in a gnmi.UpdateResult.
func Operation(oper string) func(msg proto.Message) error
```

```golang
// OperationINVALID creates a GNMIOption that sets the gnmi.SetResponse Operation to INVALID
func OperationINVALID() func(msg proto.Message) error
```

```golang
// OperationDELETE creates a GNMIOption that sets the gnmi.SetResponse Operation to DELETE
func OperationDELETE() func(msg proto.Message) error
```

```golang
// OperationREPLACE creates a GNMIOption that sets the gnmi.SetResponse Operation to REPLACE
func OperationREPLACE() func(msg proto.Message) error
```

```golang
// OperationUPDATE creates a GNMIOption that sets the gnmi.SetResponse Operation to UPDATE
func OperationUPDATE() func(msg proto.Message) error
```
