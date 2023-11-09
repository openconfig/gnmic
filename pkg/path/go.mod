module github.com/openconfig/gnmic/pkg/path

go 1.21.1

replace github.com/openconfig/gnmic/pkg/testutils => ../testutils

require (
	github.com/google/go-cmp v0.6.0
	github.com/openconfig/gnmi v0.10.0
	github.com/openconfig/gnmic/pkg/testutils v0.33.0
)

require (
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/openconfig/grpctunnel v0.1.0 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/grpc v1.59.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)
