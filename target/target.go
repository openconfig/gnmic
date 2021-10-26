package target

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/jhump/protoreflect/desc"
	"github.com/karimra/gnmic/types"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/oauth"
	"google.golang.org/grpc/encoding/gzip"
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
	Client             gnmi.GNMIClient                      `json:"-"`
	SubscribeClients   map[string]gnmi.GNMI_SubscribeClient `json:"-"` // subscription name to subscribeClient
	subscribeCancelFn  map[string]context.CancelFunc
	pollChan           chan string // subscription name to be polled
	subscribeResponses chan *SubscribeResponse
	errors             chan *TargetError
	stopped            bool
	StopChan           chan struct{}      `json:"-"`
	Cfn                context.CancelFunc `json:"-"`

	RootDesc desc.Descriptor
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
		errors:             make(chan *TargetError),
		StopChan:           make(chan struct{}),
	}
	return t
}

// CreateGNMIClient //
func (t *Target) CreateGNMIClient(ctx context.Context, opts ...grpc.DialOption) error {
	tOpts := make([]grpc.DialOption, 0, len(opts)+1)
	tOpts = append(tOpts, opts...)

	if *t.Config.Insecure {
		tOpts = append(tOpts, grpc.WithInsecure())
	} else {
		tlsConfig, err := t.Config.NewTLSConfig()
		if err != nil {
			return err
		}
		tOpts = append(tOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
		if t.Config.Token != nil && *t.Config.Token != "" {
			tOpts = append(tOpts,
				grpc.WithPerRPCCredentials(
					oauth.NewOauthAccess(&oauth2.Token{
						AccessToken: *t.Config.Token,
					})))
		}
	}
	if *t.Config.Gzip {
		tOpts = append(tOpts, grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)))
	}
	//
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
			conn, err := grpc.DialContext(timeoutCtx, addr, tOpts...)
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

// Capabilities sends a gnmi.CapabilitiesRequest to the target *t and returns a gnmi.CapabilitiesResponse and an error
func (t *Target) Capabilities(ctx context.Context, ext ...*gnmi_ext.Extension) (*gnmi.CapabilityResponse, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "username", *t.Config.Username, "password", *t.Config.Password)
	response, err := t.Client.Capabilities(ctx, &gnmi.CapabilityRequest{Extension: ext})
	if err != nil {
		return nil, fmt.Errorf("%q CapabilitiesRequest failed: %v", t.Config.Address, err)
	}
	return response, nil
}

// Get sends a gnmi.GetRequest to the target *t and returns a gnmi.GetResponse and an error
func (t *Target) Get(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "username", *t.Config.Username, "password", *t.Config.Password)
	response, err := t.Client.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%q GetRequest failed: %v", t.Config.Address, err)
	}
	return response, nil
}

// Set sends a gnmi.SetRequest to the target *t and returns a gnmi.SetResponse and an error
func (t *Target) Set(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "username", *t.Config.Username, "password", *t.Config.Password)
	response, err := t.Client.Set(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("%q SetRequest failed: %v", t.Config.Address, err)
	}
	return response, nil
}

func (t *Target) Stop() {
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
