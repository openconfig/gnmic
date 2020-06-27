package collector

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/google/gnxi/utils/xpath"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/proto/gnmi_ext"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// Target represents a gNMI enabled box
type Target struct {
	Config   *TargetConfig
	TLS      *tls.Config
	Client   gnmi.GNMIClient
	PollChan chan struct{}
}
type TargetConfig struct {
	Name       string
	Address    string
	Username   *string
	Password   *string
	Timeout    time.Duration
	Insecure   *bool
	TLSCA      *string
	TLSCert    *string
	TLSKey     *string
	SkipVerify *bool
}

func (tc *TargetConfig) String() string {
	b, err := json.Marshal(tc)
	if err != nil {
		return ""
	}
	return string(b)
}

// NewTarget //
func NewTarget(c *TargetConfig) (*Target, error) {
	tlsConfig, err := c.NewTLS()
	if err != nil {
		return nil, err
	}
	t := &Target{
		Config:   c,
		TLS:      tlsConfig,
		PollChan: make(chan struct{}),
	}
	return t, nil

}

// NewTLS //
func (c *TargetConfig) NewTLS() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		Renegotiation:      tls.RenegotiateNever,
		InsecureSkipVerify: *c.SkipVerify,
	}
	err := loadCerts(tlsConfig, c)
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
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(t.TLS)))
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
	nctx, cancel := context.WithTimeout(ctx, t.Config.Timeout)
	defer cancel()
	nctx = metadata.AppendToOutgoingContext(nctx, "username", *t.Config.Username, "password", *t.Config.Password)
	response, err := t.Client.Capabilities(nctx, &gnmi.CapabilityRequest{Extension: ext})
	if err != nil {
		return nil, fmt.Errorf("failed sending capabilities request: %v", err)
	}
	return response, nil
}

// Get sends a gnmi.GetRequest to the target *t and returns a gnmi.GetResponse and an error
func (t *Target) Get(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	nctx, cancel := context.WithTimeout(ctx, t.Config.Timeout)
	defer cancel()
	nctx = metadata.AppendToOutgoingContext(nctx, "username", *t.Config.Username, "password", *t.Config.Password)
	response, err := t.Client.Get(nctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed sending GetRequest to '%s': %v", t.Config.Address, err)
	}
	return response, nil
}

// Set sends a gnmi.SetRequest to the target *t and returns a gnmi.SetResponse and an error
func (t *Target) Set(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	nctx, cancel := context.WithTimeout(ctx, t.Config.Timeout)
	defer cancel()
	nctx = metadata.AppendToOutgoingContext(nctx, "username", *t.Config.Username, "password", *t.Config.Password)
	response, err := t.Client.Set(nctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed sending SetRequest to '%s': %v", t.Config.Address, err)
	}
	return response, nil
}

// Subscribe sends a gnmi.SubscribeRequest to the target *t and returns a chan of gnmi.SetResponse and an error
func (t *Target) Subscribe(ctx context.Context, req *gnmi.SubscribeRequest, responseChan chan *gnmi.SubscribeResponse, errs chan error) {
	nctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer close(responseChan)
	defer close(errs)
	nctx = metadata.AppendToOutgoingContext(nctx, "username", *t.Config.Username, "password", *t.Config.Password)
	subscribeClient, err := t.Client.Subscribe(nctx)
	if err != nil {
		errs <- fmt.Errorf("failed to create a subscribe client, target='%s': %v", t.Config.Name, err)
		return
	}
	err = subscribeClient.Send(req)
	if err != nil {
		errs <- fmt.Errorf("target '%s' send error: %v", t.Config.Name, err)
		return
	}
	switch req.GetSubscribe().Mode {
	case gnmi.SubscriptionList_ONCE, gnmi.SubscriptionList_STREAM:
		for {
			response, err := subscribeClient.Recv()
			if err != nil {
				errs <- fmt.Errorf("receive error: %v", err)
				return
			}
			responseChan <- response
			if req.GetSubscribe().Mode == gnmi.SubscriptionList_ONCE {
				switch response.Response.(type) {
				case *gnmi.SubscribeResponse_SyncResponse:
					return
				}
			}
		}
	case gnmi.SubscriptionList_POLL:
		for {
			select {
			case <-t.PollChan:
				err = subscribeClient.Send(&gnmi.SubscribeRequest{
					Request: &gnmi.SubscribeRequest_Poll{
						Poll: &gnmi.Poll{},
					},
				})
				if err != nil {
					errs <- fmt.Errorf("failed to send PollRequest: %v", err)
					continue
				}
				response, err := subscribeClient.Recv()
				if err != nil {
					errs <- fmt.Errorf("rcv error: %v", err)
					continue
				}
				responseChan <- response
			case <-ctx.Done():
				return
			}
		}
	}
	return
}

// CreateGetRequest creates a *gnmi.GetRequest
func CreateGetRequest(prefix string, paths []string, dataType string, encoding string, models []string) (*gnmi.GetRequest, error) {
	encodingVal, ok := gnmi.Encoding_value[strings.Replace(strings.ToUpper(encoding), "-", "_", -1)]
	if !ok {
		return nil, fmt.Errorf("invalid encoding type '%s'", encoding)
	}
	req := &gnmi.GetRequest{
		UseModels: make([]*gnmi.ModelData, 0),
		Path:      make([]*gnmi.Path, 0, len(paths)),
		Encoding:  gnmi.Encoding(encodingVal),
	}
	if prefix != "" {
		gnmiPrefix, err := xpath.ToGNMIPath(prefix)
		if err != nil {
			return nil, fmt.Errorf("prefix parse error: %v", err)
		}
		req.Prefix = gnmiPrefix
	}
	if dataType != "" {
		dti, ok := gnmi.GetRequest_DataType_value[strings.ToUpper(dataType)]
		if !ok {
			return nil, fmt.Errorf("unknown data type %s", dataType)
		}
		req.Type = gnmi.GetRequest_DataType(dti)
	}
	for _, p := range paths {
		gnmiPath, err := xpath.ToGNMIPath(strings.TrimSpace(p))
		if err != nil {
			return nil, fmt.Errorf("path parse error: %v", err)
		}
		req.Path = append(req.Path, gnmiPath)
	}
	return req, nil
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
