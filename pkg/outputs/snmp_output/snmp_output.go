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
	"strings"
	"text/template"
	"time"

	g "github.com/gosnmp/gosnmp"
	"github.com/itchyny/gojq"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/cache"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/openconfig/gnmic/pkg/path"
	"github.com/openconfig/gnmic/pkg/types"
	"github.com/openconfig/gnmic/pkg/utils"
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
		return &snmpOutput{
			cfg:       &Config{},
			logger:    log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
			eventChan: make(chan *formatters.EventMsg, initialEventsBufferSize),
		}
	})
}

type snmpOutput struct {
	name       string
	cfg        *Config
	logger     *log.Logger
	cancelFn   context.CancelFunc
	snmpClient g.Handler
	eventChan  chan *formatters.EventMsg
	evps       []formatters.EventProcessor
	targetTpl  *template.Template

	cache     cache.Cache
	startTime time.Time
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

func (s *snmpOutput) SetLogger(logger *log.Logger) {
	if logger != nil && s.logger != nil {
		s.logger.SetOutput(logger.Writer())
		s.logger.SetFlags(logger.Flags())
	}
}

func (s *snmpOutput) SetEventProcessors(ps map[string]map[string]interface{},
	logger *log.Logger,
	tcs map[string]*types.TargetConfig,
	acts map[string]map[string]interface{}) error {
	var err error
	s.evps, err = formatters.MakeEventProcessors(
		logger,
		s.cfg.EventProcessors,
		ps,
		tcs,
		acts,
	)
	if err != nil {
		return err
	}
	return nil
}

func (s *snmpOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	s.name = name
	err := outputs.DecodeConfig(cfg, s.cfg)
	if err != nil {
		return err
	}
	s.logger.SetPrefix(fmt.Sprintf(loggingPrefix, name))

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return err
		}
	}

	s.setDefaults()

	if len(s.cfg.Traps) == 0 {
		return errors.New("missing traps definition")
	}
	for i, trap := range s.cfg.Traps {
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

	if s.cfg.TargetTemplate == "" {
		s.targetTpl = outputs.DefaultTargetTemplate
	} else if s.cfg.AddTarget != "" {
		s.targetTpl, err = gtemplate.CreateTemplate("target-template", s.cfg.TargetTemplate)
		if err != nil {
			return err
		}
		s.targetTpl = s.targetTpl.Funcs(outputs.TemplateFuncs)
	}
	s.cache, err = cache.New(&cache.Config{Expiration: -1}, cache.WithLogger(s.logger))
	if err != nil {
		return err
	}

	ctx, s.cancelFn = context.WithCancel(ctx)
	go func() {
		<-ctx.Done()
		s.Close()
	}()
	s.startTime = time.Now()
	go s.start(ctx)
	s.logger.Printf("initialized SNMP output: %s", s.String())
	return nil
}

func (s *snmpOutput) Write(ctx context.Context, m proto.Message, meta outputs.Meta) {
	if m == nil {
		return
	}

	select {
	case <-ctx.Done():
		return
	default:
		rsp, err := outputs.AddSubscriptionTarget(m, meta, "if-not-present", s.targetTpl)
		if err != nil {
			s.logger.Printf("failed to add target to the response: %v", err)
			return
		}

		measName := meta["subscription-name"]
		if measName == "" {
			measName = "default"
		}

		s.cache.Write(ctx, measName, rsp)

		events, err := formatters.ResponseToEventMsgs(measName, rsp, meta, s.evps...)
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
	return s.snmpClient.Close()
}

func (s *snmpOutput) RegisterMetrics(reg *prometheus.Registry) {
	if !s.cfg.EnableMetrics {
		return
	}
	if reg == nil {
		s.logger.Printf("ERR: output metrics enabled but main registry is not initialized, enable main metrics under `api-server`")
		return
	}
	if err := s.registerMetrics(reg); err != nil {
		s.logger.Printf("failed to register metrics: %+v", err)
	}
}

func (s *snmpOutput) String() string {
	b, err := json.Marshal(s.cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

func (s *snmpOutput) start(ctx context.Context) {
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
			if init {
				<-time.After(s.cfg.StartDelay)
				init = false
			}
			for idx := range s.cfg.Traps {
				err := s.handleEvent(ev, idx)
				if err != nil {
					s.logger.Printf("failed to handle event %+v : %v", ev, err)
				}
			}
		}
	}
}

func (s *snmpOutput) SetName(name string)                               {}
func (s *snmpOutput) SetClusterName(name string)                        {}
func (s *snmpOutput) SetTargetsConfig(c map[string]*types.TargetConfig) {}

func (s *snmpOutput) setDefaults() {
	if s.cfg.Port <= 0 {
		s.cfg.Port = defaultPort
	}
	if s.cfg.Community == "" {
		s.cfg.Community = defaultCommunity
	}
	if s.cfg.StartDelay < minStartDelay {
		s.cfg.StartDelay = minStartDelay
	}
}

func (s *snmpOutput) createSNMPHandler() {
	s.snmpClient = g.NewHandler()
	s.snmpClient.SetTarget(s.cfg.Address)
	s.snmpClient.SetCommunity(s.cfg.Community)
	s.snmpClient.SetPort(s.cfg.Port)
	s.snmpClient.SetVersion(g.Version2c)
CONN:
	err := s.snmpClient.Connect()
	if err != nil {
		s.logger.Printf("failed to connect: %v", err)
		time.Sleep(time.Second)
		goto CONN
	}
	s.logger.Print("SNMP connected")
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

func (s *snmpOutput) handleEvent(ev *formatters.EventMsg, idx int) error {
	trap := *s.cfg.Traps[idx]
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
	_, err = s.snmpClient.SendTrap(g.SnmpTrap{
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
	var val interface{}
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
