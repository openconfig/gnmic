module github.com/openconfig/gnmic/pkg/types

go 1.21.1

replace github.com/openconfig/gnmic/pkg/utils => ../utils

require (
	github.com/openconfig/gnmic/pkg/utils v0.33.0
	golang.org/x/oauth2 v0.13.0
	google.golang.org/grpc v1.59.0
)

require (
	cloud.google.com/go/compute v1.23.0 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)
