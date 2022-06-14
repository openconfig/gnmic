package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/fullstorydev/grpcurl"
	"github.com/gorilla/mux"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/jhump/protoreflect/desc"
	"github.com/karimra/gnmic/config"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/inputs"
	"github.com/karimra/gnmic/lockers"
	"github.com/karimra/gnmic/outputs"
	"github.com/karimra/gnmic/target"
	"github.com/karimra/gnmic/types"
	"github.com/openconfig/gnmi/cache"
	"github.com/openconfig/gnmi/match"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/openconfig/grpctunnel/tunnel"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v2"
)

const (
	defaultHTTPClientTimeout = 5 * time.Second
)

type App struct {
	ctx     context.Context
	Cfn     context.CancelFunc
	RootCmd *cobra.Command

	sem *semaphore.Weighted
	//
	configLock *sync.RWMutex
	Config     *config.Config
	// collector
	dialOpts      []grpc.DialOption
	operLock      *sync.RWMutex
	Outputs       map[string]outputs.Output
	Inputs        map[string]inputs.Input
	Targets       map[string]*target.Target
	targetsChan   chan *target.Target
	activeTargets map[string]struct{}
	targetsLockFn map[string]context.CancelFunc
	rootDesc      desc.Descriptor
	// end collector
	router *mux.Router
	locker lockers.Locker
	// api
	apiServices map[string]*lockers.Service
	isLeader    bool
	// prometheus registry
	reg *prometheus.Registry
	//
	Logger *log.Logger
	out    io.Writer
	// prompt mode
	PromptMode    bool
	PromptHistory []string
	SchemaTree    *yang.Entry
	// yang
	modules *yang.Modules
	//
	wg        *sync.WaitGroup
	printLock *sync.Mutex
	errCh     chan error
	// gnmi server
	gnmi.UnimplementedGNMIServer
	// gRPC server where the gNMI service will be registered
	grpcSrv *grpc.Server
	// gNMI cache
	c               *cache.Cache
	match           *match.Match
	subscribeRPCsem *semaphore.Weighted
	unaryRPCsem     *semaphore.Weighted
	// tunnel server
	// gRPC server where the tunnel service will be registered
	grpcTunnelSrv *grpc.Server
	tunServer     *tunnel.Server
	ttm           *sync.RWMutex
	tunTargets    map[tunnel.Target]struct{}
	tunTargetCfn  map[tunnel.Target]context.CancelFunc
}

func New() *App {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		ctx:        ctx,
		Cfn:        cancel,
		RootCmd:    new(cobra.Command),
		sem:        semaphore.NewWeighted(1),
		configLock: new(sync.RWMutex),
		Config:     config.New(),
		reg:        prometheus.NewRegistry(),
		//
		operLock:      new(sync.RWMutex),
		Targets:       make(map[string]*target.Target),
		Outputs:       make(map[string]outputs.Output),
		Inputs:        make(map[string]inputs.Input),
		targetsChan:   make(chan *target.Target),
		activeTargets: make(map[string]struct{}),
		targetsLockFn: make(map[string]context.CancelFunc),
		//
		router:        mux.NewRouter(),
		apiServices:   make(map[string]*lockers.Service),
		Logger:        log.New(io.Discard, "[gnmic] ", log.LstdFlags|log.Lmsgprefix),
		out:           os.Stdout,
		PromptHistory: make([]string, 0, 128),
		SchemaTree: &yang.Entry{
			Dir: make(map[string]*yang.Entry),
		},

		wg:        new(sync.WaitGroup),
		printLock: new(sync.Mutex),
		c:         cache.New(nil),
		// tunnel server
		ttm:          new(sync.RWMutex),
		tunTargets:   make(map[tunnel.Target]struct{}),
		tunTargetCfn: make(map[tunnel.Target]context.CancelFunc),
	}
	a.router.StrictSlash(true)
	a.router.Use(headersMiddleware, a.loggingMiddleware)
	return a
}

func (a *App) Context() context.Context {
	if a.ctx == nil {
		return context.Background()
	}
	return a.ctx
}

func (a *App) InitGlobalFlags() {
	a.RootCmd.ResetFlags()

	a.RootCmd.PersistentFlags().StringVar(&a.Config.CfgFile, "config", "", "config file (default is $HOME/gnmic.yaml)")
	a.RootCmd.PersistentFlags().StringSliceVarP(&a.Config.GlobalFlags.Address, "address", "a", []string{}, "comma separated gnmi targets addresses")
	a.RootCmd.PersistentFlags().StringVarP(&a.Config.GlobalFlags.Username, "username", "u", "", "username")
	a.RootCmd.PersistentFlags().StringVarP(&a.Config.GlobalFlags.Password, "password", "p", "", "password")
	a.RootCmd.PersistentFlags().StringVarP(&a.Config.GlobalFlags.Port, "port", "", defaultGrpcPort, "gRPC port")
	a.RootCmd.PersistentFlags().StringVarP(&a.Config.GlobalFlags.Encoding, "encoding", "e", "json", fmt.Sprintf("one of %q. Case insensitive", encodingNames))
	a.RootCmd.PersistentFlags().BoolVarP(&a.Config.GlobalFlags.Insecure, "insecure", "", false, "insecure connection")
	a.RootCmd.PersistentFlags().StringVarP(&a.Config.GlobalFlags.TLSCa, "tls-ca", "", "", "tls certificate authority")
	a.RootCmd.PersistentFlags().StringVarP(&a.Config.GlobalFlags.TLSCert, "tls-cert", "", "", "tls certificate")
	a.RootCmd.PersistentFlags().StringVarP(&a.Config.GlobalFlags.TLSKey, "tls-key", "", "", "tls key")
	a.RootCmd.PersistentFlags().DurationVarP(&a.Config.GlobalFlags.Timeout, "timeout", "", 10*time.Second, "grpc timeout, valid formats: 10s, 1m30s, 1h")
	a.RootCmd.PersistentFlags().BoolVarP(&a.Config.GlobalFlags.Debug, "debug", "d", false, "debug mode")
	a.RootCmd.PersistentFlags().BoolVarP(&a.Config.GlobalFlags.SkipVerify, "skip-verify", "", false, "skip verify tls connection")
	a.RootCmd.PersistentFlags().BoolVarP(&a.Config.GlobalFlags.NoPrefix, "no-prefix", "", false, "do not add [ip:port] prefix to print output in case of multiple targets")
	a.RootCmd.PersistentFlags().BoolVarP(&a.Config.GlobalFlags.ProxyFromEnv, "proxy-from-env", "", false, "use proxy from environment")
	a.RootCmd.PersistentFlags().StringVarP(&a.Config.GlobalFlags.Format, "format", "", "", fmt.Sprintf("output format, one of: %q", formatNames))
	a.RootCmd.PersistentFlags().StringVarP(&a.Config.GlobalFlags.LogFile, "log-file", "", "", "log file path")
	a.RootCmd.PersistentFlags().BoolVarP(&a.Config.GlobalFlags.Log, "log", "", false, "write log messages to stderr")
	a.RootCmd.PersistentFlags().IntVarP(&a.Config.GlobalFlags.MaxMsgSize, "max-msg-size", "", msgSize, "max grpc msg size")
	a.RootCmd.PersistentFlags().BoolVarP(&a.Config.GlobalFlags.PrintRequest, "print-request", "", false, "print request as well as the response(s)")
	a.RootCmd.PersistentFlags().DurationVarP(&a.Config.GlobalFlags.Retry, "retry", "", defaultRetryTimer, "retry timer for RPCs")
	a.RootCmd.PersistentFlags().StringVarP(&a.Config.GlobalFlags.TLSMinVersion, "tls-min-version", "", "", fmt.Sprintf("minimum TLS supported version, one of %q", tlsVersions))
	a.RootCmd.PersistentFlags().StringVarP(&a.Config.GlobalFlags.TLSMaxVersion, "tls-max-version", "", "", fmt.Sprintf("maximum TLS supported version, one of %q", tlsVersions))
	a.RootCmd.PersistentFlags().StringVarP(&a.Config.GlobalFlags.TLSVersion, "tls-version", "", "", fmt.Sprintf("set TLS version. Overwrites --tls-min-version and --tls-max-version, one of %q", tlsVersions))
	a.RootCmd.PersistentFlags().BoolVarP(&a.Config.GlobalFlags.LogTLSSecret, "log-tls-secret", "", false, "enable logging of a TLS pre-master secret to a file")
	a.RootCmd.PersistentFlags().StringVarP(&a.Config.GlobalFlags.ClusterName, "cluster-name", "", defaultClusterName, "cluster name the gnmic instance belongs to, this is used for target loadsharing via a locker")
	a.RootCmd.PersistentFlags().StringVarP(&a.Config.GlobalFlags.InstanceName, "instance-name", "", "", "gnmic instance name")
	a.RootCmd.PersistentFlags().StringVarP(&a.Config.GlobalFlags.API, "api", "", "", "gnmic api address")
	a.RootCmd.PersistentFlags().StringArrayVarP(&a.Config.GlobalFlags.ProtoFile, "proto-file", "", nil, "proto file(s) name(s)")
	a.RootCmd.PersistentFlags().StringArrayVarP(&a.Config.GlobalFlags.ProtoDir, "proto-dir", "", nil, "directory to look for proto files specified with --proto-file")
	a.RootCmd.PersistentFlags().StringVarP(&a.Config.GlobalFlags.TargetsFile, "targets-file", "", "", "path to file with targets configuration")
	a.RootCmd.PersistentFlags().BoolVarP(&a.Config.GlobalFlags.Gzip, "gzip", "", false, "enable gzip compression on gRPC connections")
	a.RootCmd.PersistentFlags().StringVarP(&a.Config.GlobalFlags.Token, "token", "", "", "token value, used for gRPC token based authentication")

	a.RootCmd.PersistentFlags().StringArrayVarP(&a.Config.GlobalFlags.File, "file", "", nil, "YANG file(s)")
	a.RootCmd.PersistentFlags().StringArrayVarP(&a.Config.GlobalFlags.Dir, "dir", "", nil, "YANG dir(s)")
	a.RootCmd.PersistentFlags().StringArrayVarP(&a.Config.GlobalFlags.Exclude, "exclude", "", nil, "YANG module names to be excluded")

	a.RootCmd.PersistentFlags().BoolVarP(&a.Config.GlobalFlags.UseTunnelServer, "use-tunnel-server", "", false, "use tunnel server to dial targets")

	a.RootCmd.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(flag.Name, flag)
	})
}

func (a *App) PreRunE(cmd *cobra.Command, args []string) error {
	a.Config.SetPersistentFlagsFromFile(a.RootCmd)

	logOutput, flags, err := a.Config.SetLogger()
	if err != nil {
		return err
	}
	a.Logger.SetOutput(logOutput)
	a.Logger.SetFlags(flags)
	a.Config.Address = config.SanitizeArrayFlagValue(a.Config.Address)
	a.Logger.Printf("version=%s, commit=%s, date=%s, gitURL=%s, docs=https://gnmic.kmrd.dev", version, commit, date, gitURL)

	if a.Config.Debug {
		grpclog.SetLogger(a.Logger) //lint:ignore SA1019 see https://github.com/karimra/gnmic/issues/59
	}
	a.Logger.Printf("using config file %q", a.Config.FileConfig.ConfigFileUsed())
	a.logConfigKVs()
	return a.validateGlobals(cmd)
}

func (a *App) validateGlobals(cmd *cobra.Command) error {
	if a.Config.Insecure {
		if a.Config.SkipVerify {
			return errors.New("flags --insecure and --skip-verify are mutually exclusive")
		}
		if a.Config.TLSCa != "" {
			return errors.New("flags --insecure and --tls-ca are mutually exclusive")
		}
		if a.Config.TLSCert != "" {
			return errors.New("flags --insecure and --tls-cert are mutually exclusive")
		}
		if a.Config.TLSKey != "" {
			return errors.New("flags --insecure and --tls-key are mutually exclusive")
		}
		if a.Config.TLSVersion != "" {
			return errors.New("flags --insecure and --tls-version are mutually exclusive")
		}
		if a.Config.TLSMaxVersion != "" {
			return errors.New("flags --insecure and --tls-max-version are mutually exclusive")
		}
		if a.Config.TLSMinVersion != "" {
			return errors.New("flags --insecure and --tls-min-version are mutually exclusive")
		}
	}
	return nil
}

func (a *App) logConfigKVs() {
	if a.Config.Debug {
		b, err := yaml.Marshal(a.Config.FileConfig.AllSettings())
		if err != nil {
			a.Logger.Printf("could not marshal settings: %v", err)
		} else {
			a.Logger.Printf("set flags/config:\n%s\n", string(b))
		}
		keys := a.Config.FileConfig.AllKeys()
		sort.Strings(keys)

		for _, k := range keys {
			if !a.Config.FileConfig.IsSet(k) {
				continue
			}
			v := a.Config.FileConfig.Get(k)
			a.Logger.Printf("%s='%v'(%T)", k, v, v)
		}
	}
}

func (a *App) PrintMsg(address string, msgName string, msg proto.Message) error {
	a.printLock.Lock()
	defer a.printLock.Unlock()
	if a.Config.PrintRequest {
		fmt.Fprint(os.Stderr, msgName)
		fmt.Fprintln(os.Stderr, "")
	}
	printPrefix := ""
	if len(a.Config.TargetsList()) > 1 && !a.Config.NoPrefix {
		printPrefix = fmt.Sprintf("[%s] ", address)
	}

	switch msg := msg.ProtoReflect().Interface().(type) {
	case *gnmi.CapabilityResponse:
		if len(a.Config.Format) == 0 {
			a.printCapResponse(printPrefix, msg)
			return nil
		}
	}
	mo := formatters.MarshalOptions{
		Multiline:  true,
		Indent:     "  ",
		Format:     a.Config.Format,
		ValuesOnly: a.Config.GetValuesOnly,
	}
	b, err := mo.Marshal(msg, map[string]string{"source": address})
	if err != nil {
		a.Logger.Printf("error marshaling message: %v", err)
		if !a.Config.Log {
			fmt.Printf("error marshaling message: %v", err)
		}
		return err
	}
	sb := strings.Builder{}
	sb.Write(b)
	fmt.Fprintf(a.out, "%s\n", indent(printPrefix, sb.String()))
	return nil
}

func (a *App) createCollectorDialOpts() []grpc.DialOption {
	opts := []grpc.DialOption{grpc.WithBlock()}
	if a.Config.MaxMsgSize > 0 {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(a.Config.MaxMsgSize)))
	}
	if !a.Config.ProxyFromEnv {
		opts = append(opts, grpc.WithNoProxy())
	}
	opts = append(opts, grpc.WithUserAgent(fmt.Sprintf("gNMIc/%s", version)))
	if a.Config.Gzip {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)))
	}
	if a.Config.APIServer != nil && a.Config.APIServer.EnableMetrics && a.reg != nil {
		grpcClientMetrics := grpc_prometheus.NewClientMetrics()
		opts = append(opts,
			grpc.WithUnaryInterceptor(grpcClientMetrics.UnaryClientInterceptor()),
			grpc.WithStreamInterceptor(grpcClientMetrics.StreamClientInterceptor()),
		)
		a.reg.MustRegister(grpcClientMetrics)
	}
	a.dialOpts = opts
	return opts
}

func (a *App) watchConfig() {
	a.Logger.Printf("watching config...")
	a.Config.FileConfig.OnConfigChange(a.loadTargets)
	a.Config.FileConfig.WatchConfig()
}

func (a *App) loadTargets(e fsnotify.Event) {
	a.Logger.Printf("got config change notification: %v", e)
	ctx, cancel := context.WithCancel(a.ctx)
	defer cancel()
	err := a.sem.Acquire(ctx, 1)
	if err != nil {
		a.Logger.Printf("failed to acquire target loading semaphore: %v", err)
		return
	}
	defer a.sem.Release(1)
	switch e.Op {
	case fsnotify.Write, fsnotify.Create:
		newTargets, err := a.Config.GetTargets()
		if err != nil && !errors.Is(err, config.ErrNoTargetsFound) {
			a.Logger.Printf("failed getting targets from new config: %v", err)
			return
		}
		if !a.inCluster() {
			currentTargets := a.Targets
			// delete targets
			for n := range currentTargets {
				if _, ok := newTargets[n]; !ok {
					if a.Config.Debug {
						a.Logger.Printf("target %q deleted from config", n)
					}
					err = a.DeleteTarget(a.ctx, n)
					if err != nil {
						a.Logger.Printf("failed to delete target %q: %v", n, err)
					}
				}
			}
			// add targets
			for n, tc := range newTargets {
				if _, ok := currentTargets[n]; !ok {
					if a.Config.Debug {
						a.Logger.Printf("target %q added to config", n)
					}
					a.AddTargetConfig(tc)
					a.wg.Add(1)
					go a.TargetSubscribeStream(a.ctx, tc)
				}
			}
			return
		}
		// in a cluster
		if !a.isLeader {
			return
		}
		// in cluster && leader
		dist, err := a.getTargetToInstanceMapping()
		if err != nil {
			a.Logger.Printf("failed to get target to instance mapping: %v", err)
			return
		}
		// delete targets
		for t := range dist {
			if _, ok := newTargets[t]; !ok {
				err = a.deleteTarget(ctx, t)
				if err != nil {
					a.Logger.Printf("failed to delete target %q: %v", t, err)
					continue
				}
			}
		}
		// add new targets to cluster
		a.configLock.Lock()
		for _, tc := range newTargets {
			if _, ok := dist[tc.Name]; !ok {
				err = a.dispatchTarget(a.ctx, tc)
				if err != nil {
					a.Logger.Printf("failed to add target %q: %v", tc.Name, err)
				}
			}
		}
		a.configLock.Unlock()
	}
}

func (a *App) startAPIServer() {
	if a.Config.APIServer == nil {
		return
	}
	s, err := a.newAPIServer()
	if err != nil {
		a.Logger.Printf("failed to create a new API server: %v", err)
		return
	}
	go func() {
		var err error
		if s.TLSConfig != nil {
			err = s.ListenAndServeTLS("", "")
			if err != nil {
				a.Logger.Printf("API server err: %v", err)
				return
			}
		} else {
			err = s.ListenAndServe()
			if err != nil {
				a.Logger.Printf("API server err: %v", err)
				return
			}
		}
	}()
}

func (a *App) LoadProtoFiles() (desc.Descriptor, error) {
	if len(a.Config.ProtoFile) == 0 {
		return nil, nil
	}
	a.Logger.Printf("loading proto files...")
	descSource, err := grpcurl.DescriptorSourceFromProtoFiles(a.Config.ProtoDir, a.Config.ProtoFile...)
	if err != nil {
		a.Logger.Printf("failed to load proto files: %v", err)
		return nil, err
	}
	rootDesc, err := descSource.FindSymbol("Nokia.SROS.root")
	if err != nil {
		a.Logger.Printf("could not get symbol 'Nokia.SROS.root': %v", err)
		return nil, err
	}
	a.Logger.Printf("loaded proto files")
	a.rootDesc = rootDesc
	return rootDesc, nil
}

// GetTargets reads the targets configuration from flags or config file.
// If enabled it will load targets from a configured tunnel server.
func (a *App) GetTargets() (map[string]*types.TargetConfig, error) {
	targetsConfig, err := a.Config.GetTargets()
	if errors.Is(err, config.ErrNoTargetsFound) {
		if a.Config.UseTunnelServer {
			a.Logger.Printf("waiting %s for targets to register with the tunnel server...", a.Config.TunnelServer.TargetWaitTime)
			time.Sleep(a.Config.TunnelServer.TargetWaitTime)
			a.ttm.RLock()
			defer a.ttm.RUnlock()
			for tt := range a.tunTargets {
				tc := a.getTunnelTargetMatch(tt)
				if tc == nil {
					continue
				}
				err = a.Config.SetTargetConfigDefaults(tc)
				if err != nil {
					return nil, err
				}
				tc.Address = tc.Name
				a.AddTargetConfig(tc)
			}
		} else {
			return nil, fmt.Errorf("failed reading targets config: %v", err)
		}
	} else if err != nil {
		return nil, err
	}

	return targetsConfig, nil
}

func (a *App) CreateGNMIClient(ctx context.Context, t *target.Target) error {
	if t.Client != nil {
		return nil
	}
	targetDialOpts := a.dialOpts
	if a.Config.UseTunnelServer {
		targetDialOpts = append(targetDialOpts,
			grpc.WithContextDialer(a.tunDialerFn(ctx, t.Config)),
		)
		t.Config.Address = t.Config.Name
	}
	a.Logger.Printf("creating gRPC client for target %q", t.Config.Name)
	if err := t.CreateGNMIClient(ctx, targetDialOpts...); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("failed to create a gRPC client for target %q, timeout (%s) reached", t.Config.Name, t.Config.Timeout)
		}
		return fmt.Errorf("failed to create a gRPC client for target %q : %w", t.Config.Name, err)
	}
	return nil
}
