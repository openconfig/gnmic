package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/karimra/gnmic/api"
	"google.golang.org/protobuf/encoding/prototext"
)

func main() {
	// create a target
	tg, err := api.NewTarget(
		api.Name("srl1"),
		api.Address("srl1:57400"),
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
	// create a gNMI subscribeRequest
	subReq, err := api.NewSubscribeRequest(
		api.SubscriptionListMode("stream"),
		api.Subscription(
			api.Path("system/name"),
			api.SubscriptionMode("sample"),
			api.SampleInterval(10*time.Second),
		))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(prototext.Format(subReq))
	// start the subscription
	go tg.Subscribe(ctx, subReq, "sub1")
	// start a goroutine that will stop the subscription after x seconds
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(42 * time.Second):
			tg.StopSubscription("sub1")
		}
	}()
	subRspChan, subErrChan := tg.ReadSubscriptions()
	for {
		select {
		case rsp := <-subRspChan:
			fmt.Println(prototext.Format(rsp.Response))
		case tgErr := <-subErrChan:
			log.Fatalf("subscription %q stopped: %v", tgErr.SubscriptionName, tgErr.Err)
		}
	}
}
