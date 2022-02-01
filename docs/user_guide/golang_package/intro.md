`gnmic` (`github.com/karimra/gnmic/api`) can be imported as a dependency in your Golang programs.

It acts as a wrapper around the `openconfig/gnmi` package providing a user friendly API to create a target and easily craft gNMI requests.

## Creating gNMI requests

### Get Request

```golang
func NewGetRequest(opts ...GNMIOption) (*gnmi.GetRequest, error)
```

The below 2 snippets create a Get Request with 2 paths, `json_ietf` encoding and data type `STATE`

Using `github.com/karimra/gnmic/api`

```golang
getReq, err := api.NewGetRequest(
    api.Encoding("json_ietf"),
    api.DataType("state"),    
    api.Path("interface/statistics"),    
    api.Path("interface/subinterface/statistics"),
)
// check error
```

Using `github.com/openconfig/gnmi`

```golang
getReq := &gnmi.GetRequest{
        Path: []*gnmi.Path{
            {
                Elem: []*gnmi.PathElem{
                    {Name: "interface"},
                    {Name: "statistics"},
                },
            },
            {
                Elem: []*gnmi.PathElem{
                    {Name: "interface"},
                    {Name: "subinterface"},
                    {Name: "statistics"},
                },
            },
        },
        Type:     gnmi.GetRequest_STATE,
        Encoding: gnmi.Encoding_JSON_IETF,
    }
```

### Set Request

```golang
func NewSetRequest(opts ...GNMIOption) (*gnmi.SetRequest, error)
```

The below 2 snippets create a Set Request with one two updates, one replace and one delete messages:

Using `github.com/karimra/gnmic/api`

```golang
setReq, err := api.NewSetRequest(
    api.Update(
        api.Path("/system/name/host-name"),
        api.Value("srl2", "json_ietf"),
    ),
    api.Update(
        api.Path("/system/gnmi-server/unix-socket/admin-state"),
        api.Value("enable", "json_ietf"),
    ),
    api.Replace(
        api.Path("/network-instance[name=default]/admin-state"),
        api.Value("enable", "json_ietf"),
    ),
    api.Delete("/interface[name=ethernet-1/1]/admin-state"),
)
// check error
```

Using `github.com/openconfig/gnmi`

```golang
setReq := &gnmi.SetRequest{
    Update: []*gnmi.Update{
        {
            Path: &gnmi.Path{
                Elem: []*gnmi.PathElem{
                    {Name: "system"},
                    {Name: "name"},
                    {Name: "host-name"},
                },
            },
            Val: &gnmi.TypedValue{
                Value: &gnmi.TypedValue_JsonIetfVal{
                    JsonIetfVal: []byte("\"srl2\""),
                },
            },
        },
        {
            Path: &gnmi.Path{
                Elem: []*gnmi.PathElem{
                    {Name: "system"},
                    {Name: "gnmi-server"},
                    {Name: "unix-socket"},
                    {Name: "admin-state"},
                },
            },
            Val: &gnmi.TypedValue{
                Value: &gnmi.TypedValue_JsonIetfVal{
                    JsonIetfVal: []byte("\"enable\""),
                },
            },
        },
    },
    Replace: []*gnmi.Update{
        {
            Path: &gnmi.Path{
                Elem: []*gnmi.PathElem{
                    {
                        Name: "network-instance",
                        Key: map[string]string{
                            "name": "default",
                        },
                    },
                    {
                        Name: "admin-state",
                    },
                },
            },
            Val: &gnmi.TypedValue{
                Value: &gnmi.TypedValue_JsonIetfVal{
                    JsonIetfVal: []byte("\"enable\""),
                },
            },
        },
    },
    Delete: []*gnmi.Path{
        {
            Elem: []*gnmi.PathElem{
                {
                    Name: "interface",
                    Key: map[string]string{
                        "name": "ethernet-1/1",
                    },
                },
                {
                    Name: "admin-state",
                },
            },
        },
    },
}
```

### Subscribe Request

Create a Subscribe Request

```golang
func NewSubscribeRequest(opts ...GNMIOption) (*gnmi.SubscribeRequest, error)
```

Create a Subscribe Poll Request

```golang
func NewSubscribePollRequest(opts ...GNMIOption) *gnmi.SubscribeRequest
```

The below 2 snippets create a `stream` subscribe request with 2 paths, `json_ietf` encoding and a sample interval of 10 seconds:

Using `github.com/karimra/gnmic/api`

```golang
subReq, err := api.NewSubscribeRequest(
    api.Encoding("json_ietf"),
    api.SubscriptionListMode("stream"),
    api.Subscription(
        api.Path("interface/statistics"),
        api.SubscriptionMode("sample"),
        api.SampleInterval("10s"),
    ),
    api.Subscription(
        api.Path("interface/subinterface/statistics"),
        api.SubscriptionMode("sample"),
        api.SampleInterval("10s"),
    ),
)
// check error
```

Using `github.com/openconfig/gnmi`

```golang
subReq := &gnmi.SubscribeRequest_Subscribe{
    Subscribe: &gnmi.SubscriptionList{
        Subscription: []*gnmi.Subscription{
            {
                Path: &gnmi.Path{
                    Elem: []*gnmi.PathElem{
                        {Name: "interface"},
                        {Name: "statistics"},
                    },
                },
                Mode:           gnmi.SubscriptionMode_SAMPLE,
                SampleInterval: uint64(10 * time.Second),
            },
            {
                Path: &gnmi.Path{
                    Elem: []*gnmi.PathElem{
                        {Name: "interface"},
                        {Name: "subinterface"},
                        {Name: "statistics"},
                    },
                },
                Mode:           gnmi.SubscriptionMode_SAMPLE,
                SampleInterval: uint64(10 * time.Second),
            },
        },
        Mode:     gnmi.SubscriptionList_STREAM,
        Encoding: gnmi.Encoding_JSON_IETF,
    },
}
```

## Creating Targets

A target can be created using `func NewTarget(opts ...TargetOption) (*target.Target, error)`.

The full list of `api.TargetOption` can be found [here](target_options.md)

```golang
tg, err := api.NewTarget(
    api.Name("srl1"),
    api.Address("10.0.0.1:57400"),
    api.Username("admin"),
    api.Password("admin"),
    api.SkipVerify(true),
)
// check error
```

Once a Target is created, Multiple functions are available to run the desired RPCs, check the examples [here](examples/capabilities.md)
