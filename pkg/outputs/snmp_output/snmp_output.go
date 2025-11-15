// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package snmpoutput

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	g "github.com/gosnmp/gosnmp"
	"github.com/itchyny/gojq"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/path"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/cache"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/openconfig/gnmic/pkg/store"
	gutils "github.com/openconfig/gnmic/pkg/utils"
)

const (
	loggingPrefix           = "[snmp_output:%s] "
	defaultPort             = 162
	defaultCommunity        = "public"
	minStartDelay           = 5 * time.Second
	initialEventsBufferSize = 1000
	//
	sysUpTimeInstanceOID = "1.3.6.1.2.1.1.3.0"
)

func init() {
	outputs.Register("snmp", func() outputs.Output {
		return &snmpOutput{}
	})
}

type snmpOutput struct {
	outputs.BaseOutput
	name       string
	cfg        *atomic.Pointer[Config]
	dynCfg     *atomic.Pointer[dynConfig]
	snmpClient *atomic.Pointer[g.Handler]
	logger     *log.Logger
	rootCtx    context.Context
	cancelFn   context.CancelFunc
	eventChan  chan *formatters.EventMsg
	wg         *sync.WaitGroup
	cache      cache.Cache
	startTime  time.Time

	reg   *prometheus.Registry
	store store.Store[any]
}

type dynConfig struct {
	targetTpl *template.Template
	evps      []formatters.EventProcessor
}

type Config struct {
	Address         string        `mapstructure:"address,omitempty" json:"address,omitempty"`
	Port            uint16        `mapstructure:"port,omitempty" json:"port,omitempty"`
	Community       string        `mapstructure:"community,omitempty" json:"community,omitempty"`
	StartDelay      time.Duration `mapstructure:"start-delay,omitempty" json:"start-delay,omitempty"`
	Traps           []*trap       `mapstructure:"traps,omitempty" json:"traps,omitempty"`
	AddTarget       string        `mapstructure:"add-target,omitempty" json:"add-target,omitempty"`
	TargetTemplate  string        `mapstructure:"target-template,omitempty" json:"target-template,omitempty"`
	EnableMetrics   bool          `mapstructure:"enable-metrics,omitempty" json:"enable-metrics,omitempty"`
	EventProcessors []string      `mapstructure:"event-processors,omitempty" json:"event-processors,omitempty"`
}

type binding struct {
	Path  string `mapstructure:"path,omitempty" json:"path,omitempty"`
	OID   string `mapstructure:"oid,omitempty" json:"oid,omitempty"`
	Type  string `mapstructure:"type,omitempty" json:"type,omitempty"`
	Value string `mapstructure:"value,omitempty" json:"value,omitempty"`

	pathTemplate *gojq.Code
	oidTemplate  *gojq.Code
	valTemplate  *gojq.Code
}

type trap struct {
	InformPDU bool       `mapstructure:"inform,omitempty" json:"inform,omitempty"`
	Trigger   *binding   `mapstructure:"trigger,omitempty" json:"trigger,omitempty"`
	Bindings  []*binding `mapstructure:"bindings,omitempty" json:"bindings,omitempty"`
}

func (s *snmpOutput) buildEventProcessors(logger *log.Logger, eventProcessors []string) ([]formatters.EventProcessor, error) {
	tcs, ps, acts, err := gutils.GetConfigMaps(s.store)
	if err != nil {
		return nil, err
	}
	evps, err := formatters.MakeEventProcessors(
		logger,
		eventProcessors,
		ps,
		tcs,
		acts,
	)
	if err != nil {
		return nil, err
	}
	return evps, nil
}

func (s *snmpOutput) setLogger(logger *log.Logger) {
	if logger != nil && s.logger != nil {
		s.logger.SetOutput(logger.Writer())
		s.logger.SetFlags(logger.Flags())
	}
}

func (s *snmpOutput) init() {
	s.cfg = new(atomic.Pointer[Config])
	s.dynCfg = new(atomic.Pointer[dynConfig])
	s.logger = log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags)
	s.eventChan = make(chan *formatters.EventMsg, initialEventsBufferSize)
	s.snmpClient = new(atomic.Pointer[g.Handler])
	s.wg = new(sync.WaitGroup)
}

func (s *snmpOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	s.init() // init struct fields
	newCfg := new(Config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	s.name = name //TODO: atomic ?
	s.logger.SetPrefix(fmt.Sprintf(loggingPrefix, name))

	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}

	s.store = options.Store

	// apply logger
	s.setLogger(options.Logger)
	s.setDefaultsFor(newCfg)

	s.cfg.Store(newCfg)

	if len(newCfg.Traps) == 0 {
		return errors.New("missing traps definition")
	}
	dc := new(dynConfig)
	dc.evps, err = s.buildEventProcessors(options.Logger, newCfg.EventProcessors)
	if err != nil {
		return err
	}
	dc.targetTpl = outputs.DefaultTargetTemplate
	if newCfg.TargetTemplate != "" {
		dc.targetTpl, err = gtemplate.CreateTemplate("target-template", newCfg.TargetTemplate)
		if err != nil {
			return err
		}
		dc.targetTpl = dc.targetTpl.Funcs(outputs.TemplateFuncs)
	}
	s.dynCfg.Store(dc)
	// initialize registry
	s.reg = options.Registry
	err = s.registerMetrics()
	if err != nil {
		return err
	}

	// initialize traps
	err = s.initializeTrapsFor(newCfg)
	if err != nil {
		return err
	}

	s.cache, err = cache.New(&cache.Config{Expiration: -1}, cache.WithLogger(s.logger))
	if err != nil {
		return err
	}

	s.rootCtx = ctx
	ctx, s.cancelFn = context.WithCancel(s.rootCtx)
	s.startTime = time.Now()
	s.wg.Add(1)
	go s.start(ctx)
	s.logger.Printf("initialized SNMP output: %s", s.String())
	return nil
}

func (s *snmpOutput) Validate(cfg map[string]any) error {
	newCfg := new(Config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	if len(newCfg.Traps) == 0 {
		return errors.New("missing traps definition")
	}
	return s.initializeTrapsFor(newCfg)
}

func (s *snmpOutput) Update(_ context.Context, cfg map[string]any) error {
	newCfg := new(Config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	s.setDefaultsFor(newCfg)
	err = s.initializeTrapsFor(newCfg)
	if err != nil {
		return err
	}
	currCfg := s.cfg.Load()
	prevDC := s.dynCfg.Load()
	dc := new(dynConfig)

	processorsChanged := slices.Compare(currCfg.EventProcessors, newCfg.EventProcessors) != 0
	if processorsChanged {
		dc.evps, err = s.buildEventProcessors(s.logger, newCfg.EventProcessors)
		if err != nil {
			return err
		}
	} else if prevDC != nil {
		dc.evps = prevDC.evps
	}

	if newCfg.TargetTemplate == "" {
		dc.targetTpl = outputs.DefaultTargetTemplate
	} else if newCfg.AddTarget != "" {
		dc.targetTpl, err = gtemplate.CreateTemplate("target-template", newCfg.TargetTemplate)
		if err != nil {
			return err
		}
		dc.targetTpl = dc.targetTpl.Funcs(outputs.TemplateFuncs)
	} else {
		dc.targetTpl = outputs.DefaultTargetTemplate
	}

	s.dynCfg.Store(dc)
	s.cfg.Store(newCfg)
	// cancel old context if running
	if s.cancelFn != nil {
		s.cancelFn()
		s.wg.Wait()
	}

	// create new context and start new loop
	var ctx context.Context
	ctx, s.cancelFn = context.WithCancel(s.rootCtx)
	s.wg.Add(1)
	go s.start(ctx)
	s.logger.Printf("updated SNMP output: %s", s.String())
	return nil
}

func (s *snmpOutput) UpdateProcessor(name string, pcfg map[string]any) error {
	cfg := s.cfg.Load()
	dc := s.dynCfg.Load()

	newEvps, changed, err := outputs.UpdateProcessorInSlice(
		s.logger,
		s.store,
		cfg.EventProcessors,
		dc.evps,
		name,
		pcfg,
	)
	if err != nil {
		return err
	}
	if changed {
		newDC := *dc
		newDC.evps = newEvps
		s.dynCfg.Store(&newDC)
		s.logger.Printf("updated event processor %s", name)
	}
	return nil
}

func (s *snmpOutput) initializeTrapsFor(cfg *Config) error {
	var err error
	for i, trap := range cfg.Traps {
		if trap.Trigger == nil {
			return fmt.Errorf("trap index %d missing \"trigger\"", i)
		}
		if trap.Trigger.Path == "" {
			return fmt.Errorf("trap index %d missing \"path\"", i)
		}
		// init trap and bindings
		trap.Trigger.oidTemplate, err = parseJQ(trap.Trigger.OID)
		if err != nil {
			return err
		}
		trap.Trigger.valTemplate, err = parseJQ(trap.Trigger.Value)
		if err != nil {
			return err
		}
		for _, bd := range trap.Bindings {
			bd.pathTemplate, err = parseJQ(bd.Path)
			if err != nil {
				return err
			}
			bd.oidTemplate, err = parseJQ(bd.OID)
			if err != nil {
				return err
			}
			bd.valTemplate, err = parseJQ(bd.Value)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *snmpOutput) Write(ctx context.Context, m proto.Message, meta outputs.Meta) {
	if m == nil {
		return
	}

	cfg := s.cfg.Load()
	if cfg == nil {
		return
	}

	select {
	case <-ctx.Done():
		return
	default:
		dc := s.dynCfg.Load()
		if dc == nil {
			return
		}
		rsp, err := outputs.AddSubscriptionTarget(m, meta, "if-not-present", dc.targetTpl)
		if err != nil {
			s.logger.Printf("failed to add target to the response: %v", err)
			return
		}

		measName := meta["subscription-name"]
		if measName == "" {
			measName = "default"
		}

		s.cache.Write(ctx, measName, rsp)

		events, err := formatters.ResponseToEventMsgs(measName, rsp, meta, dc.evps...)
		if err != nil {
			s.logger.Printf("failed to convert message to event: %v", err)
			return
		}
		for _, ev := range events {
			select {
			case <-ctx.Done():
				return
			case s.eventChan <- ev:
			}
		}
	}
}

func (s *snmpOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {}

func (s *snmpOutput) Close() error {
	s.cancelFn()
	s.wg.Wait()
	snmpClient := s.snmpClient.Load()
	if snmpClient != nil {
		return (*snmpClient).Close()
	}
	return nil
}

func (s *snmpOutput) String() string {
	cfg := s.cfg.Load()
	if cfg == nil {
		return ""
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

func (s *snmpOutput) start(ctx context.Context) {
	defer s.wg.Done()
	s.createSNMPHandler()
	var init = true
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-s.eventChan:
			if ev == nil {
				return
			}
			cfg := s.cfg.Load()
			if cfg == nil {
				continue
			}
			if init {
				<-time.After(cfg.StartDelay)
				init = false
			}
			for idx := range cfg.Traps {
				err := s.handleEvent(cfg, ev, idx)
				if err != nil {
					s.logger.Printf("failed to handle event %+v : %v", ev, err)
				}
			}
		}
	}
}

func (s *snmpOutput) setDefaultsFor(cfg *Config) {
	if cfg.Port <= 0 {
		cfg.Port = defaultPort
	}
	if cfg.Community == "" {
		cfg.Community = defaultCommunity
	}
	if cfg.StartDelay < minStartDelay {
		cfg.StartDelay = minStartDelay
	}
}

func (s *snmpOutput) createSNMPHandler() {
	cfg := s.cfg.Load()
	if cfg == nil {
		return
	}
	snmpClient := g.NewHandler()
	snmpClient.SetTarget(cfg.Address)
	snmpClient.SetCommunity(cfg.Community)
	snmpClient.SetPort(cfg.Port)
	snmpClient.SetVersion(g.Version2c)
CONN:
	err := snmpClient.Connect()
	if err != nil {
		s.logger.Printf("failed to connect: %v", err)
		time.Sleep(time.Second)
		goto CONN
	}
	s.logger.Print("SNMP connected")
	s.snmpClient.Store(&snmpClient)
}

func pduType(typ string) g.Asn1BER {
	switch typ {
	case "bool":
		return g.Boolean
	case "int":
		return g.Integer
	case "bitString":
		return g.BitString
	case "octetString":
		return g.OctetString
	case "null":
		return g.Null
	case "objectID":
		return g.ObjectIdentifier
	case "objectDescription":
		return g.ObjectDescription
	case "ipAddress":
		return g.IPAddress
	case "counter32":
		return g.Counter32
	case "gauge32":
		return g.Gauge32
	case "timeTicks":
		return g.TimeTicks
	case "opaque":
		return g.Opaque
	case "nsapAddress":
		return g.NsapAddress
	case "counter64":
		return g.Counter64
	case "uint32":
		return g.Uinteger32
	case "opaqueFloat":
		return g.OpaqueFloat
	case "opaqueDouble":
		return g.OpaqueDouble
	}
	return g.UnknownType
}

func parseJQ(code string) (*gojq.Code, error) {
	q, err := gojq.Parse(strings.TrimSpace(code))
	if err != nil {
		return nil, err
	}
	return gojq.Compile(q)
}

func (s *snmpOutput) runJQ(code *gojq.Code, ev map[string]interface{}) (interface{}, error) {
	iter := code.Run(ev)
	for {
		r, ok := iter.Next()
		if !ok {
			break
		}
		switch r := r.(type) {
		case error:
			return nil, r
		default:
			return r, nil
		}
	}
	return nil, nil
}

func (s *snmpOutput) handleEvent(cfg *Config, ev *formatters.EventMsg, idx int) error {
	trap := cfg.Traps[idx]
	// trigger ?
	if _, ok := ev.Values[trap.Trigger.Path]; !ok {
		return nil
	}
	start := time.Now()
	var err error
	var target string

	if tg, ok := ev.Tags["source"]; ok {
		target = tg
	} else if tg, ok := ev.Tags["target"]; ok {
		target = tg
	} else {
		err = errors.New("missing 'source' or 'target' field")
		snmpNumberOfFailedTrapGeneration.WithLabelValues(s.name, fmt.Sprintf("%d", idx), err.Error()).Inc()
		return err
	}
	//
	pdus := make([]g.SnmpPDU, 0, len(trap.Bindings)+2)

	// append systemUptime pdu
	pdus = append(pdus, g.SnmpPDU{
		Name:  sysUpTimeInstanceOID,
		Type:  g.TimeTicks,
		Value: uint32(time.Since(s.startTime).Seconds()),
	})

	pdu, err := s.buildTriggerPDU(trap.Trigger, target, ev)
	if err != nil {
		err = fmt.Errorf("failed to build PDU from trigger: %v", err)
		snmpNumberOfFailedTrapGeneration.WithLabelValues(s.name, fmt.Sprintf("%d", idx), err.Error()).Inc()
		return err
	}

	pdus = append(pdus, pdu)

	for i, bd := range trap.Bindings {
		pdu, err := s.buildPDUFromCache(bd, target, ev)
		if err != nil {
			err = fmt.Errorf("failed to build PDU from binding index %d: %v", i, err)
			s.logger.Printf("%v", err)
			snmpNumberOfFailedTrapGeneration.WithLabelValues(s.name, fmt.Sprintf("%d", idx), err.Error()).Inc()
			continue
		}

		pdus = append(pdus, pdu)
	}
	//
	snmpNumberOfSentTraps.WithLabelValues(s.name, fmt.Sprintf("%d", idx)).Add(1)
	snmpClient := s.snmpClient.Load()
	if snmpClient == nil {
		return fmt.Errorf("SNMP client not initialized")
	}
	_, err = (*snmpClient).SendTrap(g.SnmpTrap{
		Variables: pdus,
		IsInform:  trap.InformPDU,
	})
	if err != nil {
		snmpNumberOfTrapSendFailureTraps.WithLabelValues(s.name, fmt.Sprintf("%d", idx), err.Error()).Inc()
		return fmt.Errorf("failed to send trap: %v", err)
	}
	snmpTrapGenerationDuration.WithLabelValues(s.name, fmt.Sprintf("%d", idx)).Set(float64(time.Since(start).Nanoseconds()))
	return nil
}

func (s *snmpOutput) buildTriggerPDU(bd *binding, targetName string, ev *formatters.EventMsg) (g.SnmpPDU, error) {
	var oid string
	var val any
	input := ev.ToMap()
	oidResult, err := s.runJQ(bd.oidTemplate, input)
	if err != nil {
		return g.SnmpPDU{}, fmt.Errorf("failed to run OID JQ: %v", err)
	}

	var ok bool
	oid, ok = oidResult.(string)
	if !ok {
		return g.SnmpPDU{}, fmt.Errorf("unexpected OID result type: %T", oidResult)
	}
	val, err = s.runJQ(bd.valTemplate, input)
	if err != nil {
		return g.SnmpPDU{}, fmt.Errorf("failed to run Value JQ: %v", err)
	}

	pdu := g.SnmpPDU{
		Name:  oid,
		Type:  pduType(bd.Type),
		Value: val,
	}
	return pdu, nil
}

func (s *snmpOutput) buildPDUFromCache(bd *binding, targetName string, ev *formatters.EventMsg) (g.SnmpPDU, error) {
	input := ev.ToMap()
	pathResult, err := s.runJQ(bd.pathTemplate, input)
	if err != nil {
		return g.SnmpPDU{}, fmt.Errorf("failed to run path JQ: %v", err)
	}
	xpath, ok := pathResult.(string)
	if !ok {
		return g.SnmpPDU{}, fmt.Errorf("unexpected XPATH result type: %T", pathResult)
	}

	gp, err := path.ParsePath(xpath)
	if err != nil {
		return g.SnmpPDU{}, err
	}
	rsps, err := s.cache.Read("*", targetName, gp)
	if err != nil {
		return g.SnmpPDU{}, err
	}
	evs := make([]*formatters.EventMsg, 0)
	for subName, notifs := range rsps {
		for _, notif := range notifs {
			revs, err := formatters.ResponseToEventMsgs(ev.Name, &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{
					Update: notif,
				},
			}, map[string]string{"subscription-name": subName})
			if err != nil {
				return g.SnmpPDU{}, err
			}
			evs = append(evs, revs...)
		}
	}

	if len(evs) != 1 {
		return g.SnmpPDU{}, errors.New("failed to build PDU, corresponding value not found or too many values found")
	}

	pduInput := evs[0].ToMap()
	oidResult, err := s.runJQ(bd.oidTemplate, pduInput)
	if err != nil {
		return g.SnmpPDU{}, fmt.Errorf("failed to run OID JQ: %v", err)
	}

	oid, ok := oidResult.(string)
	if !ok {
		return g.SnmpPDU{}, fmt.Errorf("unexpected OID result type: %T", oidResult)
	}
	val, err := s.runJQ(bd.valTemplate, pduInput)
	if err != nil {
		return g.SnmpPDU{}, fmt.Errorf("failed to run Value JQ: %v", err)
	}

	pdu := g.SnmpPDU{
		Name:  oid,
		Type:  pduType(bd.Type),
		Value: val,
	}
	return pdu, nil
}
