package app

import "time"

const (
	defaultGrpcPort   = "57400"
	msgSize           = 512 * 1024 * 1024
	defaultRetryTimer = 10 * time.Second
)

var encodingNames = []string{
	"json",
	"bytes",
	"proto",
	"ascii",
	"json_ietf",
}

var formatNames = []string{
	"json",
	"protojson",
	"prototext",
	"event",
	"proto",
	"flat",
}

var tlsVersions = []string{"1.3", "1.2", "1.1", "1.0", "1"}
