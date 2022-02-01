
The below snippet shows how to create a target, send a Get Request and print the response.

```golang
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/karimra/gnmic/api"
    "google.golang.org/protobuf/encoding/prototext"
)

func main() {
    // create a target
    tg, err := api.NewTarget(
        api.Name("srl1"),
        api.Address("10.0.0.1:57400"),
        api.Username("admin"),
        api.Password("admin"),
        api.SkipVerify(true),
    )
    if err != nil {
        log.Fatal(err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // create a gNMI client
    err = tg.CreateGNMIClient(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer tg.Close()

    // create a GetRequest
    getReq, err := api.NewGetRequest(
        api.Path("/system/name"),
        api.Encoding("json_ietf"))
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(prototext.Format(getReq))
    
    // send the created gNMI GetRequest to the created target
    getResp, err := tg.Get(ctx, getReq)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(prototext.Format(getResp))
}
```
