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
	Config        *TargetConfig                  `json:"config,omitempty"`
	Subscriptions map[string]*SubscriptionConfig `json:"subscriptions,omitempty"`

	m                  *sync.Mutex
	Client             gnmi.GNMIClient                      `json:"-"`
	SubscribeClients   map[string]gnmi.GNMI_SubscribeClient `json:"-"` // subscription name to subscribeClient
	subscribeCancelFn  map[string]context.CancelFunc
	pollChan           chan string // subscription name to be polled
	subscribeResponses chan *SubscribeResponse
	errors             chan *TargetError
	stopChan           chan struct{}
	cfn                context.CancelFunc
}

// TargetConfig //
type TargetConfig struct {
	Name          string        `mapstructure:"name,omitempty" json:"name,omitempty"`
	Address       string        `mapstructure:"address,omitempty" json:"address,omitempty"`
	Username      *string       `mapstructure:"username,omitempty" json:"username,omitempty"`
	Password      *string       `mapstructure:"password,omitempty" json:"password,omitempty"`
	Timeout       time.Duration `mapstructure:"timeout,omitempty" json:"timeout,omitempty"`
	Insecure      *bool         `mapstructure:"insecure,omitempty" json:"insecure,omitempty"`
	TLSCA         *string       `mapstructure:"tls-ca,omitempty" json:"tls-ca,omitempty"`
	TLSCert       *string       `mapstructure:"tls-cert,omitempty" json:"tls-cert,omitempty"`
	TLSKey        *string       `mapstructure:"tls-key,omitempty" json:"tls-key,omitempty"`
	SkipVerify    *bool         `mapstructure:"skip-verify,omitempty" json:"skip-verify,omitempty"`
	Subscriptions []string      `mapstructure:"subscriptions,omitempty" json:"subscriptions,omitempty"`
	Outputs       []string      `mapstructure:"outputs,omitempty" json:"outputs,omitempty"`
	BufferSize    uint          `mapstructure:"buffer-size,omitempty" json:"buffer-size,omitempty"`
	RetryTimer    time.Duration `mapstructure:"retry,omitempty" json:"retry-timer,omitempty"`
	TLSMinVersion string        `mapstructure:"tls-min-version,omitempty" json:"tls-min-version,omitempty"`
	TLSMaxVersion string        `mapstructure:"tls-max-version,omitempty" json:"tls-max-version,omitempty"`
	TLSVersion    string        `mapstructure:"tls-version,omitempty" json:"tls-version,omitempty"`
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
		subscribeCancelFn:  make(map[string]context.CancelFunc),
		pollChan:           make(chan string),
		subscribeResponses: make(chan *SubscribeResponse, c.BufferSize),
		errors:             make(chan *TargetError),
		stopChan:           make(chan struct{}),
	}
	return t
}

// NewTLS //
func (tc *TargetConfig) newTLS() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		Renegotiation:      tls.RenegotiateNever,
		InsecureSkipVerify: *tc.SkipVerify,
		MaxVersion:         tc.getTLSMaxVersion(),
		MinVersion:         tc.getTLSMinVersion(),
	}
	err := loadCerts(tlsConfig, tc)
	if err != nil {
		return nil, err
	}
	return tlsConfig, nil
}

// CreateGNMIClient //
func (t *Target) CreateGNMIClient(ctx context.Context, opts ...grpc.DialOption) error {
	tOpts := make([]grpc.DialOption, 0, len(opts)+1)
	tOpts = append(tOpts, opts...)

	if *t.Config.Insecure {
		tOpts = append(tOpts, grpc.WithInsecure())
	} else {
		tlsConfig, err := t.Config.newTLS()
		if err != nil {
			return err
		}
		tOpts = append(tOpts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, t.Config.Timeout)
	defer cancel()
	conn, err := grpc.DialContext(timeoutCtx, t.Config.Address, tOpts...)
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
				SubscriptionName: subscriptionName,
				Response:         response,
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
					SubscriptionName: subscriptionName,
					Response:         response,
				}
			case <-nctx.Done():
				return
			}
		}
	}
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
	close(t.stopChan)
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

func (tc *TargetConfig) UsernameString() string {
	if tc.Username == nil {
		return "NA"
	}
	return *tc.Username
}

func (tc *TargetConfig) PasswordString() string {
	if tc.Password == nil {
		return "NA"
	}
	return *tc.Password
}

func (tc *TargetConfig) InsecureString() string {
	if tc.Insecure == nil {
		return "NA"
	}
	return fmt.Sprintf("%t", *tc.Insecure)
}

func (tc *TargetConfig) TLSCAString() string {
	if tc.TLSCA == nil || *tc.TLSCA == "" {
		return "NA"
	}
	return *tc.TLSCA
}

func (tc *TargetConfig) TLSKeyString() string {
	if tc.TLSKey == nil || *tc.TLSKey == "" {
		return "NA"
	}
	return *tc.TLSKey
}

func (tc *TargetConfig) TLSCertString() string {
	if tc.TLSCert == nil || *tc.TLSCert == "" {
		return "NA"
	}
	return *tc.TLSCert
}

func (tc *TargetConfig) SkipVerifyString() string {
	if tc.SkipVerify == nil {
		return "NA"
	}
	return fmt.Sprintf("%t", *tc.SkipVerify)
}

func (tc *TargetConfig) SubscriptionString() string {
	return fmt.Sprintf("- %s", strings.Join(tc.Subscriptions, "\n"))
}

func (tc *TargetConfig) OutputsString() string {
	return strings.Join(tc.Outputs, "\n")
}

func (tc *TargetConfig) BufferSizeString() string {
	return fmt.Sprintf("%d", tc.BufferSize)
}

func (tc *TargetConfig) getTLSMinVersion() uint16 {
	v := tlsVersionStringToUint(tc.TLSVersion)
	if v > 0 {
		return v
	}
	return tlsVersionStringToUint(tc.TLSMinVersion)
}

func (tc *TargetConfig) getTLSMaxVersion() uint16 {
	v := tlsVersionStringToUint(tc.TLSVersion)
	if v > 0 {
		return v
	}
	return tlsVersionStringToUint(tc.TLSMaxVersion)
}

func tlsVersionStringToUint(v string) uint16 {
	switch v {
	default:
		return 0
	case "1.3":
		return tls.VersionTLS13
	case "1.2":
		return tls.VersionTLS12
	case "1.1":
		return tls.VersionTLS11
	case "1.0", "1":
		return tls.VersionTLS10
	}
}
