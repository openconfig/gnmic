package collector

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/dynamic"
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

const (
	defaultRetryTimer = 10 * time.Second
)

type TargetError struct {
	SubscriptionName string
	Err              error
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
	stopChan           chan struct{}
	cfn                context.CancelFunc

	rootDesc desc.Descriptor
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
		stopChan:           make(chan struct{}),
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
		tlsConfig, err := t.Config.NewTLS()
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
		return nil, fmt.Errorf("failed sending capabilities request: %v", err)
	}
	return response, nil
}

// Get sends a gnmi.GetRequest to the target *t and returns a gnmi.GetResponse and an error
func (t *Target) Get(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "username", *t.Config.Username, "password", *t.Config.Password)
	response, err := t.Client.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed sending GetRequest to '%s': %v", t.Config.Address, err)
	}
	return response, nil
}

// Set sends a gnmi.SetRequest to the target *t and returns a gnmi.SetResponse and an error
func (t *Target) Set(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "username", *t.Config.Username, "password", *t.Config.Password)
	response, err := t.Client.Set(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed sending SetRequest to '%s': %v", t.Config.Address, err)
	}
	return response, nil
}

// Subscribe sends a gnmi.SubscribeRequest to the target *t, responses and error are sent to the target channels
func (t *Target) Subscribe(ctx context.Context, req *gnmi.SubscribeRequest, subscriptionName string) {
SUBSC:
	nctx, cancel := context.WithCancel(ctx)
	defer cancel()
	nctx = metadata.AppendToOutgoingContext(nctx, "username", *t.Config.Username, "password", *t.Config.Password)
	subscribeClient, err := t.Client.Subscribe(nctx)
	if err != nil {
		t.errors <- &TargetError{
			SubscriptionName: subscriptionName,
			Err:              fmt.Errorf("failed to create a subscribe client, target='%s', retry in %d. err=%v", t.Config.Name, t.Config.RetryTimer, err),
		}
		cancel()
		time.Sleep(t.Config.RetryTimer)
		goto SUBSC
	}
	t.m.Lock()
	t.SubscribeClients[subscriptionName] = subscribeClient
	t.subscribeCancelFn[subscriptionName] = cancel
	subConfig := t.Subscriptions[subscriptionName]
	t.m.Unlock()
	err = subscribeClient.Send(req)
	if err != nil {
		t.errors <- &TargetError{
			SubscriptionName: subscriptionName,
			Err:              fmt.Errorf("target '%s' send error, retry in %d. err=%v", t.Config.Name, t.Config.RetryTimer, err),
		}
		cancel()
		time.Sleep(t.Config.RetryTimer)
		goto SUBSC
	}

	switch req.GetSubscribe().Mode {
	case gnmi.SubscriptionList_STREAM:
		for {
			if nctx.Err() != nil {
				return
			}
			response, err := subscribeClient.Recv()
			if err != nil {
				t.errors <- &TargetError{
					SubscriptionName: subscriptionName,
					Err:              err,
				}
				t.errors <- &TargetError{
					SubscriptionName: subscriptionName,
					Err:              fmt.Errorf("retrying in %s", t.Config.RetryTimer),
				}
				cancel()
				time.Sleep(t.Config.RetryTimer)
				goto SUBSC
			}
			t.subscribeResponses <- &SubscribeResponse{
				SubscriptionName:   subscriptionName,
				SubscriptionConfig: subConfig,
				Response:           response,
			}
		}
	case gnmi.SubscriptionList_ONCE:
		for {
			response, err := subscribeClient.Recv()
			if err != nil {
				t.errors <- &TargetError{
					SubscriptionName: subscriptionName,
					Err:              err,
				}
				if errors.Is(err, io.EOF) {
					return
				}
				t.errors <- &TargetError{
					SubscriptionName: subscriptionName,
					Err:              fmt.Errorf("retrying in %d", t.Config.RetryTimer),
				}
				cancel()
				time.Sleep(t.Config.RetryTimer)
				goto SUBSC
			}
			t.subscribeResponses <- &SubscribeResponse{
				SubscriptionName:   subscriptionName,
				SubscriptionConfig: subConfig,
				Response:           response,
			}
			switch response.Response.(type) {
			case *gnmi.SubscribeResponse_SyncResponse:
				return
			}
		}
	case gnmi.SubscriptionList_POLL:
		for {
			select {
			case subName := <-t.pollChan:
				err = t.SubscribeClients[subName].Send(&gnmi.SubscribeRequest{
					Request: &gnmi.SubscribeRequest_Poll{
						Poll: &gnmi.Poll{},
					},
				})
				if err != nil {
					t.errors <- &TargetError{
						SubscriptionName: subscriptionName,
						Err:              fmt.Errorf("failed to send PollRequest: %v", err),
					}
					continue
				}
				response, err := subscribeClient.Recv()
				if err != nil {
					t.errors <- &TargetError{
						SubscriptionName: subscriptionName,
						Err:              err,
					}
					continue
				}
				t.subscribeResponses <- &SubscribeResponse{
					SubscriptionName:   subscriptionName,
					SubscriptionConfig: subConfig,
					Response:           response,
				}
			case <-nctx.Done():
				return
			}
		}
	}
}

func (t *Target) SubscribeOnce(ctx context.Context, req *gnmi.SubscribeRequest, subscriptionName string) (chan *gnmi.SubscribeResponse, chan error) {
	responseCh := make(chan *gnmi.SubscribeResponse)
	errCh := make(chan error)
	go func() {
		nctx, cancel := context.WithCancel(ctx)
		defer cancel()

		nctx = metadata.AppendToOutgoingContext(nctx, "username", *t.Config.Username, "password", *t.Config.Password)
		subscribeClient, err := t.Client.Subscribe(nctx)
		if err != nil {
			errCh <- err
			return
		}
		err = subscribeClient.Send(req)
		if err != nil {
			errCh <- err
			return
		}
		for {
			response, err := subscribeClient.Recv()
			if err != nil {
				errCh <- err
				return
			}
			responseCh <- response
		}
	}()

	return responseCh, errCh
}

func (t *Target) Stop() {
	t.m.Lock()
	defer t.m.Unlock()
	for _, cfn := range t.subscribeCancelFn {
		cfn()
	}
	if t.cfn != nil {
		t.cfn()
	}
	if !t.stopped {
		close(t.stopChan)
	}
	t.stopped = true
}

func (t *Target) ReadSubscriptions() (chan *SubscribeResponse, chan *TargetError) {
	return t.subscribeResponses, t.errors
}

func (t *Target) numberOfOnceSubscriptions() int {
	num := 0
	for _, sub := range t.Subscriptions {
		if strings.ToUpper(sub.Mode) == "ONCE" {
			num++
		}
	}
	return num
}

func (t *Target) decodeProtoBytes(resp *gnmi.SubscribeResponse) error {
	if t.rootDesc == nil {
		return nil
	}
	switch resp := resp.Response.(type) {
	case *gnmi.SubscribeResponse_Update:
		for _, update := range resp.Update.Update {
			switch update.Val.Value.(type) {
			case *gnmi.TypedValue_ProtoBytes:
				m := dynamic.NewMessage(t.rootDesc.GetFile().FindMessage("Nokia.SROS.root"))
				err := m.Unmarshal(update.Val.GetProtoBytes())
				if err != nil {
					return err
				}
				jsondata, err := m.MarshalJSON()
				if err != nil {
					return err
				}
				update.Val.Value = &gnmi.TypedValue_JsonVal{JsonVal: jsondata}
			}
		}
	}
	return nil
}
