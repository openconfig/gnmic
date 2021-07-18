package gnmi_output

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io/ioutil"
	"log"
	"math/big"
	"net"
	"strings"
	"text/template"
	"time"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/openconfig/gnmi/cache"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/proto"
)

const (
	loggingPrefix           = "[gnmi_output] "
	defaultMaxSubscriptions = 64
	defaultAddress          = ":57400"
)

func init() {
	outputs.Register("gnmi", func() outputs.Output {
		return &gNMIOutput{
			cfg:    new(config),
			logger: log.New(ioutil.Discard, loggingPrefix, log.LstdFlags|log.Lmicroseconds),
		}
	})
}

// gNMIOutput //
type gNMIOutput struct {
	cfg       *config
	logger    *log.Logger
	targetTpl *template.Template
	//
	srv      *server
	grpcSrv  *grpc.Server
	c        *cache.Cache
	teardown func()
}

type config struct {
	Name             string `mapstructure:"name,omitempty"`
	Address          string `mapstructure:"address,omitempty"`
	TargetTemplate   string `mapstructure:"target-template,omitempty"`
	MaxSubscriptions int64  `mapstructure:"max-subscriptions,omitempty"`
	// TLS
	SkipVerify bool   `mapstructure:"skip-verify,omitempty"`
	CaFile     string `mapstructure:"ca-file,omitempty"`
	CertFile   string `mapstructure:"cert-file,omitempty"`
	KeyFile    string `mapstructure:"key-file,omitempty"`
	//
	EnableMetrics bool `mapstructure:"enable-metrics,omitempty"`
	Debug         bool `mapstructure:"debug,omitempty"`
}

func (g *gNMIOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, g.cfg)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(g)
	}
	g.setDefaults()
	err = g.startGRPCServer()
	if err != nil {
		return err
	}
	g.logger.Printf("started gnmi output: %v", g)
	return nil
}

func (g *gNMIOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	err := outputs.AddSubscriptionTarget(rsp, meta, "if-not-present", g.targetTpl)
	if err != nil {
		g.logger.Printf("failed to add target to the response: %v", err)
	}
	switch rsp := rsp.(type) {
	case *gnmi.SubscribeResponse:
		switch rsp := rsp.Response.(type) {
		case *gnmi.SubscribeResponse_Update:
			target := rsp.Update.GetPrefix().GetTarget()
			if target == "" {
				g.logger.Printf("response missing target")
				return
			}
			if !g.c.HasTarget(target) {
				g.c.Add(target)
				g.logger.Printf("target %q added to the local cache", target)
			}
			if g.cfg.Debug {
				g.logger.Printf("updating target %q local cache", target)
			}
			err = g.c.GnmiUpdate(rsp.Update)
			if err != nil {
				g.logger.Printf("failed to update gNMI cache: %v", err)
				return
			}
		case *gnmi.SubscribeResponse_SyncResponse:
		}
	}
}

func (g *gNMIOutput) WriteEvent(context.Context, *formatters.EventMsg) {}

func (g *gNMIOutput) Close() error {
	g.teardown()
	g.grpcSrv.Stop()
	return nil
}

func (g *gNMIOutput) RegisterMetrics(reg *prometheus.Registry) {
	if !g.cfg.EnableMetrics {
		return
	}
	srvMetrics := grpc_prometheus.NewServerMetrics()
	srvMetrics.InitializeMetrics(g.grpcSrv)
	if err := reg.Register(srvMetrics); err != nil {
		g.logger.Printf("failed to register prometheus metrics: %v", err)
	}
}

func (g *gNMIOutput) String() string {
	b, err := json.Marshal(g.cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

func (g *gNMIOutput) SetLogger(logger *log.Logger) {
	if logger != nil && g.logger != nil {
		g.logger.SetOutput(logger.Writer())
		g.logger.SetFlags(logger.Flags())
	}
}

func (g *gNMIOutput) SetEventProcessors(map[string]map[string]interface{}, *log.Logger, map[string]interface{}) {
}

func (g *gNMIOutput) SetName(string) {}

func (g *gNMIOutput) SetClusterName(string) {}

//

func (g *gNMIOutput) setDefaults() error {
	if g.cfg.MaxSubscriptions <= 0 {
		g.cfg.MaxSubscriptions = defaultMaxSubscriptions
	}
	if g.cfg.Address == "" {
		g.cfg.Address = defaultAddress
	}
	if g.cfg.TargetTemplate == "" {
		g.targetTpl = outputs.DefaultTargetTemplate
	}
	return nil
}

func (g *gNMIOutput) startGRPCServer() error {
	var err error
	g.c = cache.New(nil)
	g.srv = g.newServer()
	g.c.SetClient(g.srv.Update)

	var l net.Listener
	network := "tcp"
	addr := g.cfg.Address
	if strings.HasPrefix(g.cfg.Address, "unix://") {
		network = "unix"
		addr = strings.TrimPrefix(addr, "unix://")
	}
	l, err = net.Listen(network, addr)
	if err != nil {
		return err
	}
	opts, err := g.serverOpts()
	if err != nil {
		return err
	}
	g.grpcSrv = grpc.NewServer(opts...)
	gnmi.RegisterGNMIServer(g.grpcSrv, g.srv)
	go g.grpcSrv.Serve(l)
	return nil
}

func (g *gNMIOutput) serverOpts() ([]grpc.ServerOption, error) {
	opts := make([]grpc.ServerOption, 0)
	if g.cfg.EnableMetrics {
		opts = append(opts, grpc.StreamInterceptor(grpc_prometheus.StreamServerInterceptor))
	}
	if g.cfg.SkipVerify || g.cfg.CaFile != "" || (g.cfg.CertFile != "" && g.cfg.KeyFile != "") {
		tlscfg := &tls.Config{
			Renegotiation:      tls.RenegotiateNever,
			InsecureSkipVerify: g.cfg.SkipVerify,
		}
		if g.cfg.CertFile != "" && g.cfg.KeyFile != "" {
			certificate, err := tls.LoadX509KeyPair(g.cfg.CertFile, g.cfg.KeyFile)
			if err != nil {
				return nil, err
			}
			tlscfg.Certificates = []tls.Certificate{certificate}
			// tlscfg.BuildNameToCertificate()
		} else {
			cert, err := selfSignedCerts()
			if err != nil {
				return nil, err
			}
			tlscfg.Certificates = []tls.Certificate{cert}
		}
		if g.cfg.CaFile != "" {
			certPool := x509.NewCertPool()
			caFile, err := ioutil.ReadFile(g.cfg.CaFile)
			if err != nil {
				return nil, err
			}
			if ok := certPool.AppendCertsFromPEM(caFile); !ok {
				return nil, errors.New("failed to append certificate")
			}
			tlscfg.RootCAs = certPool
		}
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlscfg)))
	}
	return opts, nil
}

func selfSignedCerts() (tls.Certificate, error) {
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, nil
	}
	certTemplate := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"kmrd.dev"},
		},
		DNSNames:              []string{"kmrd.dev"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	priv, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return tls.Certificate{}, nil
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, certTemplate, &priv.PublicKey, priv)
	if err != nil {
		return tls.Certificate{}, nil
	}
	certBuff := new(bytes.Buffer)
	keyBuff := new(bytes.Buffer)
	pem.Encode(certBuff, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	pem.Encode(keyBuff, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return tls.X509KeyPair(certBuff.Bytes(), keyBuff.Bytes())
}
