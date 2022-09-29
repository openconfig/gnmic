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

	"github.com/openconfig/gnmic/api"
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
	// send a gNMI capabilities request to the created target
	capResp, err := tg.Capabilities(ctx)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(prototext.Format(capResp))
}
