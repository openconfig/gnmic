
The below snippet shows how to create a target, send a Set Request and print the reponse.

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
    
    err = tg.CreateGNMIClient(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer tg.Close()

    // create a gNMI SetRequest
    setReq, err := api.NewSetRequest(
        api.Update(
            api.Path("/system/name/host-name"),
            api.Value("srl2", "json_ietf")),
    )
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(prototext.Format(setReq))

    // send the created gNMI SetRequest to the created target
    setResp, err := tg.Set(ctx, setReq)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(prototext.Format(setResp))
}
```
