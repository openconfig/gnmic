// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/protobuf/encoding/prototext"

	"github.com/openconfig/gnmic/pkg/api"
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
