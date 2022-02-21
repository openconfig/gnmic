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
	"google.golang.org/grpc"
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
		errors:             make(chan *TargetError),
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

// Capabilities sends a gnmi.CapabilitiesRequest to the target *t and returns a gnmi.CapabilitiesResponse and an error
func (t *Target) Capabilities(ctx context.Context, ext ...*gnmi_ext.Extension) (*gnmi.CapabilityResponse, error) {
	if t.Config.Username != nil {
		ctx = metadata.AppendToOutgoingContext(ctx, "username", *t.Config.Username)
	}
	if t.Config.Password != nil {
		ctx = metadata.AppendToOutgoingContext(ctx, "password", *t.Config.Password)
	}
	return t.Client.Capabilities(ctx, &gnmi.CapabilityRequest{Extension: ext})
}

// Get sends a gnmi.GetRequest to the target *t and returns a gnmi.GetResponse and an error
func (t *Target) Get(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	if t.Config.Username != nil {
		ctx = metadata.AppendToOutgoingContext(ctx, "username", *t.Config.Username)
	}
	if t.Config.Password != nil {
		ctx = metadata.AppendToOutgoingContext(ctx, "password", *t.Config.Password)
	}
	return t.Client.Get(ctx, req)
}

// Set sends a gnmi.SetRequest to the target *t and returns a gnmi.SetResponse and an error
func (t *Target) Set(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	if t.Config.Username != nil {
		ctx = metadata.AppendToOutgoingContext(ctx, "username", *t.Config.Username)
	}
	if t.Config.Password != nil {
		ctx = metadata.AppendToOutgoingContext(ctx, "password", *t.Config.Password)
	}
	return t.Client.Set(ctx, req)
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
