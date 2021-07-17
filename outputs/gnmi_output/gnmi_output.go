package gnmi_output

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"strings"
	"text/template"

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/openconfig/gnmi/cache"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/subscribe"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

const (
	loggingPrefix            = "[gnmi_output] "
	defaultSubscriptionLimit = 64
	defaultAddress           = ":57400"
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
	c        *cache.Cache
	teardown func()
}

type config struct {
	Name              string `mapstructure:"name,omitempty"`
	Address           string `mapstructure:"address,omitempty"`
	TargetTemplate    string `mapstructure:"target-template,omitempty"`
	SubscriptionLimit int    `mapstructure:"subscription-limit,omitempty"`
	SkipVerify        bool   `mapstructure:"skip-verify,omitempty"`
	CaFile            string `mapstructure:"ca-file,omitempty"`
	CertFile          string `mapstructure:"cert-file,omitempty"`
	KeyFile           string `mapstructure:"key-file,omitempty"`
	Debug             bool   `mapstructure:"debug,omitempty"`
}

func (g *gNMIOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, g.cfg)
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(g)
	}
	if g.cfg.SubscriptionLimit > 0 {
		subscribe.SubscriptionLimit = g.cfg.SubscriptionLimit
	} else {
		subscribe.SubscriptionLimit = defaultSubscriptionLimit
	}
	if g.cfg.Address == "" {
		g.cfg.Address = defaultAddress
	}
	if g.cfg.TargetTemplate == "" {
		g.targetTpl = outputs.DefaultTargetTemplate
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
			// g.logger.Printf("updating target %q local cache", target)
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
	return nil
}

func (g *gNMIOutput) RegisterMetrics(*prometheus.Registry) {}

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

func (g *gNMIOutput) startGRPCServer() error {
	var err error
	g.c = cache.New(nil)
	g.srv, err = g.newServer()
	if err != nil {
		return err
	}
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
	srv := grpc.NewServer()
	gnmi.RegisterGNMIServer(srv, g.srv)
	go srv.Serve(l)
	return nil
}
