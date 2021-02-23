package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/mux"
	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/config"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/lockers"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/spf13/cobra"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/protobuf/proto"
)

const (
	defaultHTTPClientTimeout = 5 * time.Second
)

type App struct {
	ctx     context.Context
	Cfn     context.CancelFunc
	RootCmd *cobra.Command

	sem *semaphore.Weighted

	m           *sync.Mutex
	Config      *config.Config
	collector   *collector.Collector
	router      *mux.Router
	locker      lockers.Locker
	apiServices map[string]*lockers.Service
	isLeader    bool

	httpClient *http.Client

	Logger        *log.Logger
	out           io.Writer
	PromptMode    bool
	PromptHistory []string
	SchemaTree    *yang.Entry

	wg        *sync.WaitGroup
	printLock *sync.Mutex
	errCh     chan error
}

func New() *App {
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{
		ctx:         ctx,
		Cfn:         cancel,
		RootCmd:     new(cobra.Command),
		sem:         semaphore.NewWeighted(1),
		m:           new(sync.Mutex),
		Config:      config.New(),
		router:      mux.NewRouter(),
		apiServices: make(map[string]*lockers.Service),
		httpClient: &http.Client{
			Timeout: defaultHTTPClientTimeout,
		},
		Logger:        log.New(ioutil.Discard, "", log.LstdFlags),
		out:           os.Stdout,
		PromptHistory: make([]string, 0, 128),
		SchemaTree: &yang.Entry{
			Dir: make(map[string]*yang.Entry),
		},

		wg:        new(sync.WaitGroup),
		printLock: new(sync.Mutex),
	}
	a.router.StrictSlash(true)
	a.router.Use(headersMiddleware, a.loggingMiddleware)
	return a
}

func (a *App) PreRun(_ *cobra.Command, args []string) error {
	a.Config.SetLogger()
	a.Logger.SetOutput(a.Config.LogOutput())
	a.Logger.SetFlags(a.Config.LogFlags())
	a.Config.SetPersistantFlagsFromFile(a.RootCmd)
	a.Config.Address = config.SanitizeArrayFlagValue(a.Config.Address)
	a.Logger = log.New(ioutil.Discard, "[gnmic] ", log.LstdFlags|log.Lmicroseconds)
	if a.Config.LogFile != "" {
		f, err := os.OpenFile(a.Config.LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("error opening log file: %v", err)
		}
		a.Logger.SetOutput(f)
	} else {
		if a.Config.Debug {
			a.Config.Log = true
		}
		if a.Config.Log {
			a.Logger.SetOutput(os.Stderr)
		}
	}
	if a.Config.Debug {
		a.Logger.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Llongfile)
	}

	if a.Config.Debug {
		grpclog.SetLogger(a.Logger) //lint:ignore SA1019 see https://github.com/karimra/gnmic/issues/59
		a.Logger.Printf("version=%s, commit=%s, date=%s, gitURL=%s, docs=https://gnmic.kmrd.dev", version, commit, date, gitURL)
	}
	cfgFile := a.Config.FileConfig.ConfigFileUsed()
	if len(cfgFile) != 0 {
		a.Logger.Printf("using config file %s", cfgFile)
		b, err := ioutil.ReadFile(cfgFile)
		if err != nil {
			if a.RootCmd.Flag("config").Changed {
				return err
			}
			a.Logger.Printf("failed reading config file: %v", err)
		}
		if a.Config.Debug {
			a.Logger.Printf("config file:\n%s", string(b))
		}
	}
	// logConfig
	return nil
}

func (a *App) PrintMsg(address string, msgName string, msg proto.Message) error {
	a.printLock.Lock()
	defer a.printLock.Unlock()
	fmt.Fprint(os.Stderr, msgName)
	fmt.Fprintln(os.Stderr, "")
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
		Multiline: true,
		Indent:    "  ",
		Format:    a.Config.Format,
	}
	b, err := mo.Marshal(msg, map[string]string{"address": address})
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
	opts := []grpc.DialOption{}
	opts = append(opts, grpc.WithBlock())
	if a.Config.MaxMsgSize > 0 {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(a.Config.MaxMsgSize)))
	}
	if !a.Config.ProxyFromEnv {
		opts = append(opts, grpc.WithNoProxy())
	}
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
			currentTargets := a.collector.Targets
			// delete targets
			for n := range currentTargets {
				if _, ok := newTargets[n]; !ok {
					if a.Config.Debug {
						a.Logger.Printf("target %q deleted from config", n)
					}
					err = a.collector.DeleteTarget(a.ctx, n)
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
					err = a.collector.AddTarget(tc)
					if err != nil {
						a.Logger.Printf("failed adding target %q: %v", n, err)
						continue
					}
					a.wg.Add(1)
					go a.collector.StartTarget(a.ctx, n)
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
				err = a.deleteTarget(t)
				if err != nil {
					a.Logger.Printf("failed to delete target %q: %v", t, err)
					continue
				}
			}
		}
		// add new targets to cluster
		for _, tc := range newTargets {
			if _, ok := dist[tc.Name]; !ok {
				err = a.dispatchTarget(a.ctx, tc)
				if err != nil {
					a.Logger.Printf("failed to add target %q: %v", tc.Name, err)
				}
				time.Sleep(dispatchPace)
			}
		}
	}
}

func (a *App) startAPI() {
	if a.Config.API == "" {
		return
	}
	a.routes()
	s := &http.Server{
		Addr:    a.Config.API,
		Handler: a.router,
	}
	go func() {
		err := s.ListenAndServe()
		if err != nil {
			a.Logger.Printf("API server err: %v", err)
			return
		}
	}()
}
