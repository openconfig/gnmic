// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package app

import "time"

const (
	defaultGrpcPort   = "57400"
	msgSize           = 512 * 1024 * 1024
	defaultRetryTimer = 10 * time.Second

	formatJSON      = "json"
	formatPROTOJSON = "protojson"
	formatPROTOTEXT = "prototext"
	formatEvent     = "event"
	formatPROTO     = "proto"
	formatFLAT      = "flat"
)

var encodingNames = []string{
	"json",
	"bytes",
	"proto",
	"ascii",
	"json_ietf",
}

var formatNames = []string{
	formatJSON,
	formatPROTOJSON,
	formatPROTOTEXT,
	formatEvent,
	formatPROTO,
	formatFLAT,
}

var tlsVersions = []string{"1.3", "1.2", "1.1", "1.0", "1"}
