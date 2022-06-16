package gnmi_output

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"text/template"

	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/openconfig/gnmi/cache"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/protobuf/proto"
)

const (
	loggingPrefix           = "[gnmi_output:%s] "
	defaultMaxSubscriptions = 64
	defaultMaxGetRPC        = 64
	defaultAddress          = ":57400"
)

func init() {
	outputs.Register("gnmi", func() outputs.Output {
		return &gNMIOutput{
			cfg:    new(config),
			logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

// gNMIOutput //
type gNMIOutput struct {
	cfg       *config
	logger    *log.Logger
	targetTpl *template.Template
	//
	srv     *server
	grpcSrv *grpc.Server
	c       *cache.Cache
	//teardown func()
}

type config struct {
	//Name             string `mapstructure:"name,omitempty"`
	Address          string `mapstructure:"address,omitempty"`
	TargetTemplate   string `mapstructure:"target-template,omitempty"`
	MaxSubscriptions int64  `mapstructure:"max-subscriptions,omitempty"`
	MaxUnaryRPC      int64  `mapstructure:"max-unary-rpc,omitempty"`
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
	g.c = cache.New(nil)
	g.srv = g.newServer()

	for _, opt := range opts {
		opt(g)
	}

	err = g.setDefaults()
	if err != nil {
		return err
	}
	g.logger.SetPrefix(fmt.Sprintf(loggingPrefix, name))
	if g.targetTpl == nil {
		g.targetTpl, err = utils.CreateTemplate(fmt.Sprintf("%s-target-template", name), g.cfg.TargetTemplate)
		if err != nil {
			return err
		}
	}
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
	//g.teardown()
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

func (g *gNMIOutput) SetEventProcessors(map[string]map[string]interface{}, *log.Logger, map[string]*types.TargetConfig, map[string]map[string]interface{}) {
}

func (g *gNMIOutput) SetName(string) {}

func (g *gNMIOutput) SetClusterName(string) {}

func (g *gNMIOutput) SetTargetsConfig(tcs map[string]*types.TargetConfig) {
	if g.srv == nil {
		return
	}
	g.srv.mu.Lock()
	for n, tc := range tcs {
		if tc.Name != "" {
			g.srv.targets[tc.Name] = tc
			continue
		}
		g.srv.targets[n] = tc
	}
	for n := range g.srv.targets {
		if _, ok := tcs[n]; !ok {
			delete(g.srv.targets, n)
		}
	}
	g.srv.mu.Unlock()
}

//

func (g *gNMIOutput) setDefaults() error {
	if g.cfg.Address == "" {
		g.cfg.Address = defaultAddress
	}
	if g.cfg.TargetTemplate == "" {
		g.targetTpl = outputs.DefaultTargetTemplate
	}
	if g.cfg.MaxSubscriptions <= 0 {
		g.cfg.MaxSubscriptions = defaultMaxSubscriptions
	}
	if g.cfg.MaxUnaryRPC <= 0 {
		g.cfg.MaxUnaryRPC = defaultMaxGetRPC
	}
	return nil
}

func (g *gNMIOutput) startGRPCServer() error {
	g.srv.subscribeRPCsem = semaphore.NewWeighted(g.cfg.MaxSubscriptions)
	g.srv.unaryRPCsem = semaphore.NewWeighted(g.cfg.MaxUnaryRPC)
	g.c.SetClient(g.srv.Update)

	var l net.Listener
	var err error
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

	tlscfg, err := utils.NewTLSConfig(g.cfg.CaFile, g.cfg.CertFile, g.cfg.KeyFile, g.cfg.SkipVerify, true)
	if err != nil {
		return nil, err
	}
	if tlscfg != nil {
		opts = append(opts, grpc.Creds(credentials.NewTLS(tlscfg)))
	}

	return opts, nil
}
