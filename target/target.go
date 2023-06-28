// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/jhump/protoreflect/desc"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"github.com/openconfig/gnmic/types"
	"golang.org/x/net/proxy"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/metadata"
)

type TargetError struct {
	SubscriptionName string
	Err              error
}

// SubscribeResponse //
type SubscribeResponse struct {
	SubscriptionName   string
	SubscriptionConfig *types.SubscriptionConfig
	Response           *gnmi.SubscribeResponse
}

// Target represents a gNMI enabled box
type Target struct {
	Config        *types.TargetConfig                  `json:"config,omitempty"`
	Subscriptions map[string]*types.SubscriptionConfig `json:"subscriptions,omitempty"`

	m                  *sync.Mutex
	conn               *grpc.ClientConn
	Client             gnmi.GNMIClient                      `json:"-"`
	SubscribeClients   map[string]gnmi.GNMI_SubscribeClient `json:"-"` // subscription name to subscribeClient
	subscribeCancelFn  map[string]context.CancelFunc
	pollChan           chan string // subscription name to be polled
	subscribeResponses chan *SubscribeResponse
	errors             chan *TargetError
	stopped            bool
	StopChan           chan struct{}      `json:"-"`
	Cfn                context.CancelFunc `json:"-"`
	RootDesc           desc.Descriptor    `json:"-"`
}

// NewTarget //
func NewTarget(c *types.TargetConfig) *Target {
	t := &Target{
		Config:             c,
		Subscriptions:      make(map[string]*types.SubscriptionConfig),
		m:                  new(sync.Mutex),
		SubscribeClients:   make(map[string]gnmi.GNMI_SubscribeClient),
		subscribeCancelFn:  make(map[string]context.CancelFunc),
		pollChan:           make(chan string),
		subscribeResponses: make(chan *SubscribeResponse, c.BufferSize),
		errors:             make(chan *TargetError, c.BufferSize),
		StopChan:           make(chan struct{}),
	}
	return t
}

// CreateGNMIClient //
func (t *Target) CreateGNMIClient(ctx context.Context, opts ...grpc.DialOption) error {
	tOpts, err := t.Config.GrpcDialOptions()
	if err != nil {
		return err
	}
	opts = append(opts, tOpts...)
	opts = append(opts, grpc.WithBlock())
	// create a gRPC connection
	addrs := strings.Split(t.Config.Address, ",")
	numAddrs := len(addrs)
	errC := make(chan error, numAddrs)
	connC := make(chan *grpc.ClientConn)
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for _, addr := range addrs {
		go func(addr string) {
			timeoutCtx, cancel := context.WithTimeout(ctx, t.Config.Timeout)
			defer cancel()
			if t.Config.Proxy != "" {
				if idx := strings.Index(t.Config.Proxy, "://"); idx >= 0 {
					proxyType := t.Config.Proxy[:idx]
					proxyAddress := t.Config.Proxy[idx+3:]
					if proxyType == "socks5" {
						opts = append(opts, grpc.WithContextDialer(
							func(_ context.Context, addr string) (net.Conn, error) {
								dialer, err := proxy.SOCKS5("tcp", proxyAddress, nil,
									&net.Dialer{
										Timeout:   t.Config.Timeout,
										KeepAlive: t.Config.Timeout,
									},
								)
								if err != nil {
									errC <- fmt.Errorf("%s: %v", addr, err)
									return nil, err
								}
								return dialer.Dial("tcp", addr)
							}))
					}
				}
			}
			conn, err := grpc.DialContext(timeoutCtx, addr, opts...)
			if err != nil {
				errC <- fmt.Errorf("%s: %v", addr, err)
				return
			}
			select {
			case connC <- conn:
			case <-done:
				if conn != nil {
					conn.Close()
				}
			}
		}(addr)
	}
	errs := make([]string, 0, numAddrs)
	for {
		select {
		case conn := <-connC:
			close(done)
			t.conn = conn
			t.Client = gnmi.NewGNMIClient(conn)
			return nil
		case err := <-errC:
			errs = append(errs, err.Error())
			if len(errs) == numAddrs {
				return fmt.Errorf("%s", strings.Join(errs, ", "))
			}
		}
	}
}

func (t *Target) callOpts() []grpc.CallOption {
	if t.Config.AuthScheme == "" {
		return nil
	}
	callOpts := make([]grpc.CallOption, 0, 1)

	var auth string
	if t.Config.Username != nil {
		auth = *t.Config.Username
	}
	auth += ":"
	if t.Config.Password != nil {
		auth += *t.Config.Password
	}

	callOpts = append(callOpts,
		grpc.PerRPCCredentials(
			oauth.TokenSource{
				TokenSource: oauth2.StaticTokenSource(
					&oauth2.Token{
						AccessToken: base64.StdEncoding.EncodeToString([]byte(auth)),
						TokenType:   t.Config.AuthScheme,
					},
				),
			},
		))

	return callOpts
}

func (t *Target) appendCredentials(ctx context.Context) context.Context {
	if t.Config.AuthScheme != "" {
		return ctx
	}

	if t.Config.Username != nil && *t.Config.Username != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "username", *t.Config.Username)
	}
	if t.Config.Password != nil && *t.Config.Password != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "password", *t.Config.Password)
	}
	return ctx
}

// Capabilities sends a gnmi.CapabilitiesRequest to the target *t and returns a gnmi.CapabilitiesResponse and an error
func (t *Target) Capabilities(ctx context.Context, ext ...*gnmi_ext.Extension) (*gnmi.CapabilityResponse, error) {
	return t.Client.Capabilities(t.appendCredentials(ctx), &gnmi.CapabilityRequest{Extension: ext}, t.callOpts()...)
}

// Get sends a gnmi.GetRequest to the target *t and returns a gnmi.GetResponse and an error
func (t *Target) Get(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	return t.Client.Get(t.appendCredentials(ctx), req, t.callOpts()...)
}

// Set sends a gnmi.SetRequest to the target *t and returns a gnmi.SetResponse and an error
func (t *Target) Set(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	return t.Client.Set(t.appendCredentials(ctx), req, t.callOpts()...)
}

func (t *Target) StopSubscriptions() {
	t.m.Lock()
	defer t.m.Unlock()
	for _, cfn := range t.subscribeCancelFn {
		cfn()
	}
	if t.Cfn != nil {
		t.Cfn()
	}
	if !t.stopped {
		close(t.StopChan)
	}
	t.stopped = true
}

func (t *Target) Close() error {
	t.StopSubscriptions()
	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}

func (t *Target) ConnState() string {
	if t.conn == nil {
		return ""
	}
	return t.conn.GetState().String()
}
