package collector

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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
	Config        *TargetConfig
	Subscriptions map[string]*SubscriptionConfig

	m                  *sync.Mutex
	Client             gnmi.GNMIClient
	SubscribeClients   map[string]gnmi.GNMI_SubscribeClient // subscription name to subscribeClient
	PollChan           chan string                          // subscription name to be polled
	SubscribeResponses chan *SubscribeResponse
	Errors             chan *TargetError
}

// TargetConfig //
type TargetConfig struct {
	Name          string        `mapstructure:"name,omitempty"`
	Address       string        `mapstructure:"address,omitempty"`
	Username      *string       `mapstructure:"username,omitempty"`
	Password      *string       `mapstructure:"password,omitempty"`
	Timeout       time.Duration `mapstructure:"timeout,omitempty"`
	Insecure      *bool         `mapstructure:"insecure,omitempty"`
	TLSCA         *string       `mapstructure:"tls-ca,omitempty"`
	TLSCert       *string       `mapstructure:"tls-cert,omitempty"`
	TLSKey        *string       `mapstructure:"tls-key,omitempty"`
	SkipVerify    *bool         `mapstructure:"skip-verify,omitempty"`
	Subscriptions []string      `mapstructure:"subscriptions,omitempty"`
	Outputs       []string      `mapstructure:"outputs,omitempty"`
	BufferSize    uint          `mapstructure:"buffer-size,omitempty"`
	RetryTimer    time.Duration `mapstructure:"retry,omitempty"`
}

func (tc *TargetConfig) String() string {
	b, err := json.Marshal(tc)
	if err != nil {
		return ""
	}
	return string(b)
}

// NewTarget //
func NewTarget(c *TargetConfig) *Target {
	t := &Target{
		Config:             c,
		Subscriptions:      make(map[string]*SubscriptionConfig),
		m:                  new(sync.Mutex),
		SubscribeClients:   make(map[string]gnmi.GNMI_SubscribeClient),
		PollChan:           make(chan string),
		SubscribeResponses: make(chan *SubscribeResponse, c.BufferSize),
		Errors:             make(chan *TargetError),
	}
	return t
}

// NewTLS //
func (tc *TargetConfig) newTLS() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		Renegotiation:      tls.RenegotiateNever,
		InsecureSkipVerify: *tc.SkipVerify,
	}
	err := loadCerts(tlsConfig, tc)
	if err != nil {
		return nil, err
	}
	return tlsConfig, nil
}

// CreateGNMIClient //
func (t *Target) CreateGNMIClient(ctx context.Context, opts ...grpc.DialOption) error {
	if opts == nil {
		opts = []grpc.DialOption{}
	}
	if *t.Config.Insecure {
		opts = append(opts, grpc.WithInsecure())
	} else {
		tlsConfig, err := t.Config.newTLS()
		if err != nil {
			return err
		}
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, t.Config.Timeout)
	defer cancel()
	conn, err := grpc.DialContext(timeoutCtx, t.Config.Address, opts...)
	if err != nil {
		return err
	}
	t.Client = gnmi.NewGNMIClient(conn)
	return nil
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
		t.Errors <- &TargetError{
			SubscriptionName: subscriptionName,
			Err:              fmt.Errorf("failed to create a subscribe client, target='%s', retry in %d. err=%v", t.Config.Name, t.Config.RetryTimer, err),
		}
		cancel()
		time.Sleep(t.Config.RetryTimer)
		goto SUBSC
	}
	t.m.Lock()
	t.SubscribeClients[subscriptionName] = subscribeClient
	t.m.Unlock()
	err = subscribeClient.Send(req)
	if err != nil {
		t.Errors <- &TargetError{
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
			response, err := subscribeClient.Recv()
			if err != nil {
				t.Errors <- &TargetError{
					SubscriptionName: subscriptionName,
					Err:              err,
				}
				t.Errors <- &TargetError{
					SubscriptionName: subscriptionName,
					Err:              fmt.Errorf("retrying in %d", t.Config.RetryTimer),
				}
				cancel()
				time.Sleep(t.Config.RetryTimer)
				goto SUBSC
			}
			t.SubscribeResponses <- &SubscribeResponse{
				SubscriptionName: subscriptionName,
				Response:         response,
			}
		}
	case gnmi.SubscriptionList_ONCE:
		for {
			response, err := subscribeClient.Recv()
			if err != nil {
				t.Errors <- &TargetError{
					SubscriptionName: subscriptionName,
					Err:              err,
				}
				if errors.Is(err, io.EOF) {
					return
				}
				t.Errors <- &TargetError{
					SubscriptionName: subscriptionName,
					Err:              fmt.Errorf("retrying in %d", t.Config.RetryTimer),
				}
				cancel()
				time.Sleep(t.Config.RetryTimer)
				goto SUBSC
			}
			t.SubscribeResponses <- &SubscribeResponse{
				SubscriptionName: subscriptionName,
				Response:         response,
			}
			switch response.Response.(type) {
			case *gnmi.SubscribeResponse_SyncResponse:
				return
			}
		}
	case gnmi.SubscriptionList_POLL:
		for {
			select {
			case subName := <-t.PollChan:
				err = t.SubscribeClients[subName].Send(&gnmi.SubscribeRequest{
					Request: &gnmi.SubscribeRequest_Poll{
						Poll: &gnmi.Poll{},
					},
				})
				if err != nil {
					t.Errors <- &TargetError{
						SubscriptionName: subscriptionName,
						Err:              fmt.Errorf("failed to send PollRequest: %v", err),
					}
					continue
				}
				response, err := subscribeClient.Recv()
				if err != nil {
					t.Errors <- &TargetError{
						SubscriptionName: subscriptionName,
						Err:              err,
					}
					continue
				}
				t.SubscribeResponses <- &SubscribeResponse{
					SubscriptionName: subscriptionName,
					Response:         response,
				}
			case <-nctx.Done():
				return
			}
		}
	}
}

func loadCerts(tlscfg *tls.Config, c *TargetConfig) error {
	if *c.TLSCert != "" && *c.TLSKey != "" {
		certificate, err := tls.LoadX509KeyPair(*c.TLSCert, *c.TLSKey)
		if err != nil {
			return err
		}
		tlscfg.Certificates = []tls.Certificate{certificate}
		tlscfg.BuildNameToCertificate()
	}
	if c.TLSCA != nil && *c.TLSCA != "" {
		certPool := x509.NewCertPool()
		caFile, err := ioutil.ReadFile(*c.TLSCA)
		if err != nil {
			return err
		}
		if ok := certPool.AppendCertsFromPEM(caFile); !ok {
			return errors.New("failed to append certificate")
		}
		tlscfg.RootCAs = certPool
	}
	return nil
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
