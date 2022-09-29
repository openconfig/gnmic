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
	// create a gNMI client
	err = tg.CreateGNMIClient(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer tg.Close()
	// create a GetREquest
	getReq, err := api.NewGetRequest(
		api.Path("/system/name"),
		api.Encoding("json_ietf"),
	)
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
