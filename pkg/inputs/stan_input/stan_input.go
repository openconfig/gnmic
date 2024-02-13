// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package stan_input

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/stan.go"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/inputs"
	"github.com/openconfig/gnmic/pkg/outputs"
)

const (
	loggingPrefix           = "[stan_input] "
	defaultAddress          = "localhost:4222"
	stanConnectWait         = 2 * time.Second
	stanDefaultPingInterval = 5
	stanDefaultPingRetry    = 2
	stanDefaultClusterName  = "test-cluster"
	defaultFormat           = "event"
	defaultSubject          = "telemetry"
	defaultNumWorkers       = 1
)

func init() {
	inputs.Register("stan", func() inputs.Input {
		return &StanInput{
			Cfg:    &Config{},
			logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
			wg:     new(sync.WaitGroup),
		}
	})
}

// StanInput //
type StanInput struct {
	Cfg    *Config
	ctx    context.Context
	cfn    context.CancelFunc
	logger *log.Logger

	wg      *sync.WaitGroup
	outputs []outputs.Output
	evps    []formatters.EventProcessor
}

// Config //
type Config struct {
	Name            string           `mapstructure:"name,omitempty"`
	Address         string           `mapstructure:"address,omitempty"`
	Subject         string           `mapstructure:"subject,omitempty"`
	Queue           string           `mapstructure:"queue,omitempty"`
	Username        string           `mapstructure:"username,omitempty"`
	Password        string           `mapstructure:"password,omitempty"`
	ConnectTimeWait time.Duration    `mapstructure:"connect-time-wait,omitempty"`
	TLS             *types.TLSConfig `mapstructure:"tls,omitempty" json:"tls,omitempty"`
	ClusterName     string           `mapstructure:"cluster-name,omitempty"`
	PingInterval    int              `mapstructure:"ping-interval,omitempty"`
	PingRetry       int              `mapstructure:"ping-retry,omitempty"`
	Format          string           `mapstructure:"format,omitempty"`
	Debug           bool             `mapstructure:"debug,omitempty"`
	NumWorkers      int              `mapstructure:"num-workers,omitempty"`
	Outputs         []string         `mapstructure:"outputs,omitempty"`
	EventProcessors []string         `mapstructure:"event-processors,omitempty"`
}

func (s *StanInput) Start(ctx context.Context, name string, cfg map[string]interface{}, opts ...inputs.Option) error {
	err := outputs.DecodeConfig(cfg, s.Cfg)
	if err != nil {
		return err
	}
	if s.Cfg.Name == "" {
		s.Cfg.Name = name
	}
	for _, opt := range opts {
		if err := opt(s); err != nil {
			return err
		}
	}
	err = s.setDefaults()
	if err != nil {
		return err
	}
	s.ctx, s.cfn = context.WithCancel(ctx)
	s.wg.Add(s.Cfg.NumWorkers)
	for i := 0; i < s.Cfg.NumWorkers; i++ {
		go s.worker(ctx, i)
	}
	return nil
}

func (s *StanInput) worker(ctx context.Context, idx int) {
	defer s.wg.Done()
	var stanConn stan.Conn
	var err error
	workerLogPrefix := fmt.Sprintf("worker-%d", idx)
	cfg := *s.Cfg
	cfg.Name = fmt.Sprintf("%s-%d", cfg.Name, idx)
	s.logger.Printf("%s starting", workerLogPrefix)
START:
	stanConn, err = s.createSTANConn(&cfg)
	if err != nil {
		s.logger.Printf("%s failed to create NATS connection: %v", workerLogPrefix, err)
		time.Sleep(s.Cfg.ConnectTimeWait)
		goto START
	}
	s.logger.Printf("%s initialized stan input connection: %+v", workerLogPrefix, s)
	defer stanConn.Close()
	defer stanConn.NatsConn().Close()
	s.logger.Printf("%s subscribing to subject=%q, queue=%q", workerLogPrefix, s.Cfg.Subject, s.Cfg.Queue)
	sub, err := stanConn.QueueSubscribe(s.Cfg.Subject, s.Cfg.Queue, s.handleMsg, stan.DurableName(cfg.Name))
	if err != nil {
		s.logger.Printf("%s failed to subscribe to STAN subject=%q, queue=%q, err=%v", workerLogPrefix, s.Cfg.Subject, s.Cfg.Queue, err)
		stanConn.Close()
		stanConn.NatsConn().Close()
		time.Sleep(s.Cfg.ConnectTimeWait)
		goto START
	}
	defer sub.Close()
	defer sub.Unsubscribe()
	<-ctx.Done()
}

func (s *StanInput) Close() error {
	s.cfn()
	s.wg.Wait()
	return nil
}

func (s *StanInput) SetLogger(logger *log.Logger) {
	if logger != nil && s.logger != nil {
		s.logger.SetOutput(logger.Writer())
		s.logger.SetFlags(logger.Flags())
	}
}

func (s *StanInput) SetOutputs(outs map[string]outputs.Output) {
	if len(s.Cfg.Outputs) == 0 {
		for _, o := range outs {
			s.outputs = append(s.outputs, o)
		}
		return
	}
	for _, name := range s.Cfg.Outputs {
		if o, ok := outs[name]; ok {
			s.outputs = append(s.outputs, o)
		}
	}
}

func (s *StanInput) SetName(name string) {
	sb := strings.Builder{}
	if name != "" {
		sb.WriteString(name)
		sb.WriteString("-")
	}
	sb.WriteString(s.Cfg.Name)
	sb.WriteString("-stan-sub")
	s.Cfg.Name = sb.String()
}

func (s *StanInput) SetEventProcessors(ps map[string]map[string]interface{}, logger *log.Logger, tcs map[string]*types.TargetConfig, acts map[string]map[string]interface{}) error {
	var err error
	s.evps, err = formatters.MakeEventProcessors(
		logger,
		s.Cfg.EventProcessors,
		ps,
		tcs,
		acts,
	)
	if err != nil {
		return err
	}
	return nil
}

// helper functions

func (s *StanInput) setDefaults() error {
	if s.Cfg.Format == "" {
		s.Cfg.Format = defaultFormat
	}
	if !(strings.ToLower(s.Cfg.Format) == "event" || strings.ToLower(s.Cfg.Format) == "proto") {
		return fmt.Errorf("unsupported input format")
	}
	if s.Cfg.Name == "" {
		s.Cfg.Name = "gnmic-" + uuid.New().String()
	}
	if s.Cfg.Subject == "" {
		s.Cfg.Subject = defaultSubject
	}
	if s.Cfg.Address == "" {
		s.Cfg.Address = defaultAddress
	}
	if s.Cfg.ConnectTimeWait <= 0 {
		s.Cfg.ConnectTimeWait = stanConnectWait
	}
	if s.Cfg.Queue == "" {
		s.Cfg.Queue = s.Cfg.Name
	}
	if s.Cfg.NumWorkers <= 0 {
		s.Cfg.NumWorkers = defaultNumWorkers
	}
	if s.Cfg.PingInterval <= 1 {
		s.Cfg.PingInterval = stanDefaultPingInterval
	}
	if s.Cfg.PingRetry <= 1 {
		s.Cfg.PingRetry = stanDefaultPingRetry
	}
	if s.Cfg.ClusterName == "" {
		s.Cfg.ClusterName = stanDefaultClusterName
	}
	return nil
}

func (s *StanInput) createSTANConn(c *Config) (stan.Conn, error) {
	opts := []nats.Option{
		nats.Name(c.Name),
	}
	if c.Username != "" && c.Password != "" {
		opts = append(opts, nats.UserInfo(c.Username, c.Password))
	}
	if s.Cfg.TLS != nil {
		tlsConfig, err := utils.NewTLSConfig(
			s.Cfg.TLS.CaFile, s.Cfg.TLS.CertFile, s.Cfg.TLS.KeyFile,
			"",
			s.Cfg.TLS.SkipVerify,
			false,
		)
		if err != nil {
			return nil, err
		}
		if tlsConfig != nil {
			opts = append(opts, nats.Secure(tlsConfig))
		}
	}
	var nc *nats.Conn
	var sc stan.Conn
	var err error
CRCONN:
	s.logger.Printf("attempting to connect to %s", c.Address)
	nc, err = nats.Connect(c.Address, opts...)
	if err != nil {
		s.logger.Printf("failed to create connection: %v", err)
		time.Sleep(s.Cfg.ConnectTimeWait)
		goto CRCONN
	}

	sc, err = stan.Connect(c.ClusterName, c.Name,
		stan.NatsConn(nc),
		stan.Pings(c.PingInterval, c.PingRetry),
		stan.SetConnectionLostHandler(func(_ stan.Conn, err error) {
			s.logger.Printf("STAN connection lost, reason: %v", err)
			s.logger.Printf("retrying...")
			//sc = s.createSTANConn(c)
		}),
	)
	if err != nil {
		s.logger.Printf("failed to create connection: %v", err)
		nc.Close()
		time.Sleep(s.Cfg.ConnectTimeWait)
		goto CRCONN
	}
	s.logger.Printf("successfully connected to STAN server %s", c.Address)
	return sc, nil
}

func (s *StanInput) handleMsg(m *stan.Msg) {
	if m == nil || len(m.Data) == 0 {
		return
	}
	if s.Cfg.Debug {
		s.logger.Printf("received msg, subject=%q, queue=%q, len=%d, data=%s", m.Subject, s.Cfg.Queue, len(m.Data), string(m.Data))
	}
	var err error
	switch s.Cfg.Format {
	case "event":
		evMsgs := make([]*formatters.EventMsg, 1)
		err = json.Unmarshal(m.Data, &evMsgs)
		if err != nil {
			if s.Cfg.Debug {
				s.logger.Printf("failed to unmarshal event msg: %v", err)
			}
			return
		}

		for _, p := range s.evps {
			evMsgs = p.Apply(evMsgs...)
		}

		go func() {
			for _, o := range s.outputs {
				for _, ev := range evMsgs {
					o.WriteEvent(s.ctx, ev)
				}
			}
		}()
	case "proto":
		var protoMsg proto.Message
		err = proto.Unmarshal(m.Data, protoMsg)
		if err != nil {
			if s.Cfg.Debug {
				s.logger.Printf("failed to unmarshal proto msg: %v", err)
			}
			return
		}
		meta := outputs.Meta{}
		subjectSections := strings.SplitN(m.Subject, ".", 3)
		if len(subjectSections) == 3 {
			meta["source"] = strings.ReplaceAll(subjectSections[1], "-", ".")
			meta["subscription-name"] = subjectSections[2]
		}
		go func() {
			for _, o := range s.outputs {
				o.Write(s.ctx, protoMsg, meta)
			}
		}()
	}
}

func (s *StanInput) String() string {
	b, err := json.Marshal(s)
	if err != nil {
		return ""
	}
	return string(b)
}
