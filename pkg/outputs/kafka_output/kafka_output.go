// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package kafka_output

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/outputs"
	pkgutils "github.com/openconfig/gnmic/pkg/utils"
	"github.com/zestor-dev/zestor/store"
)

const (
	defaultKafkaMaxRetry    = 2
	defaultKafkaTimeout     = 5 * time.Second
	defaultKafkaTopic       = "telemetry"
	defaultNumWorkers       = 1
	defaultFormat           = "event"
	defaultRecoveryWaitTime = 10 * time.Second
	defaultAddress          = "localhost:9092"
	loggingPrefixTpl        = "[kafka_output:%s] "
	defaultCompressionCodec = sarama.CompressionNone

	requiredAcksNoResponse   = "no-response"
	requiredAcksWaitForLocal = "wait-for-local"
	requiredAcksWaitForAll   = "wait-for-all"
)

var bytesBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

var stringBuilderPool = sync.Pool{
	New: func() any {
		return new(strings.Builder)
	},
}

func init() {
	outputs.Register("kafka", func() outputs.Output {
		return &kafkaOutput{}
	})
}

func (k *kafkaOutput) init() {
	k.cfg = new(atomic.Pointer[config])
	k.dynCfg = new(atomic.Pointer[dynConfig])
	k.msgChan = new(atomic.Pointer[chan *outputs.ProtoMsg])
	k.wg = new(sync.WaitGroup)
	k.logger = log.New(io.Discard, loggingPrefixTpl, utils.DefaultLoggingFlags)
	k.closeOnce = sync.Once{}
	k.closeSig = make(chan struct{})
}

// kafkaOutput //
type kafkaOutput struct {
	outputs.BaseOutput

	cfg       *atomic.Pointer[config]
	dynCfg    *atomic.Pointer[dynConfig]
	logger    sarama.StdLogger
	srcLogger *log.Logger
	msgChan   *atomic.Pointer[chan *outputs.ProtoMsg]
	wg        *sync.WaitGroup

	rootCtx   context.Context
	cancelFn  context.CancelFunc
	reg       *prometheus.Registry
	store     store.Store[any]
	closeOnce sync.Once
	closeSig  chan struct{}
}

type dynConfig struct {
	targetTpl *template.Template
	msgTpl    *template.Template
	evps      []formatters.EventProcessor
	mo        *formatters.MarshalOptions
}

// config //
type config struct {
	Address            string           `mapstructure:"address,omitempty"`
	Topic              string           `mapstructure:"topic,omitempty"`
	TopicPrefix        string           `mapstructure:"topic-prefix,omitempty"`
	Name               string           `mapstructure:"name,omitempty"`
	SASL               *types.SASL      `mapstructure:"sasl,omitempty"`
	TLS                *types.TLSConfig `mapstructure:"tls,omitempty"`
	MaxRetry           int              `mapstructure:"max-retry,omitempty"`
	Timeout            time.Duration    `mapstructure:"timeout,omitempty"`
	RecoveryWaitTime   time.Duration    `mapstructure:"recovery-wait-time,omitempty"`
	FlushFrequency     time.Duration    `mapstructure:"flush-frequency,omitempty"`
	SyncProducer       bool             `mapstructure:"sync-producer,omitempty"`
	RequiredAcks       string           `mapstructure:"required-acks,omitempty"`
	Format             string           `mapstructure:"format,omitempty"`
	InsertKey          bool             `mapstructure:"insert-key,omitempty"`
	AddTarget          string           `mapstructure:"add-target,omitempty"`
	TargetTemplate     string           `mapstructure:"target-template,omitempty"`
	MsgTemplate        string           `mapstructure:"msg-template,omitempty"`
	SplitEvents        bool             `mapstructure:"split-events,omitempty"`
	NumWorkers         int              `mapstructure:"num-workers,omitempty"`
	CompressionCodec   string           `mapstructure:"compression-codec,omitempty"`
	KafkaVersion       string           `mapstructure:"kafka-version,omitempty"`
	Debug              bool             `mapstructure:"debug,omitempty"`
	BufferSize         int              `mapstructure:"buffer-size,omitempty"`
	OverrideTimestamps bool             `mapstructure:"override-timestamps,omitempty"`
	EnableMetrics      bool             `mapstructure:"enable-metrics,omitempty"`
	EventProcessors    []string         `mapstructure:"event-processors,omitempty"`
}

func (k *kafkaOutput) String() string {
	cfg := k.cfg.Load()
	if cfg == nil {
		return ""
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

func (k *kafkaOutput) buildEventProcessors(logger *log.Logger, eventProcessors []string) ([]formatters.EventProcessor, error) {
	tcs, ps, acts, err := pkgutils.GetConfigMaps(k.store)
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

// Init /
func (k *kafkaOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	k.init() // init struct fields
	newCfg := new(config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	if newCfg.Name == "" {
		newCfg.Name = name
	}
	loggingPrefix := fmt.Sprintf(loggingPrefixTpl, newCfg.Name)
	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}

	k.store = options.Store
	if options.Logger != nil {
		k.srcLogger = options.Logger
		sarama.Logger = log.New(options.Logger.Writer(), loggingPrefix, options.Logger.Flags())
		k.logger = sarama.Logger
	}

	err = k.setDefaultsFor(newCfg)
	if err != nil {
		return err
	}
	// store config
	k.cfg.Store(newCfg)

	// initialize registry
	k.reg = options.Registry
	err = k.registerMetrics()
	if err != nil {
		return err
	}
	dc := new(dynConfig)
	// initialize event processors
	evps, err := k.buildEventProcessors(options.Logger, newCfg.EventProcessors)
	if err != nil {
		return err
	}
	dc.evps = evps
	newMsgChan := make(chan *outputs.ProtoMsg, uint(newCfg.BufferSize))
	k.msgChan.Store(&newMsgChan)

	if newCfg.TargetTemplate == "" {
		dc.targetTpl = outputs.DefaultTargetTemplate
	} else if newCfg.AddTarget != "" {
		dc.targetTpl, err = gtemplate.CreateTemplate("target-template", newCfg.TargetTemplate)
		if err != nil {
			return err
		}
		dc.targetTpl = dc.targetTpl.Funcs(outputs.TemplateFuncs)
	}

	if newCfg.MsgTemplate != "" {
		dc.msgTpl, err = gtemplate.CreateTemplate("msg-template", newCfg.MsgTemplate)
		if err != nil {
			return err
		}
		dc.msgTpl = dc.msgTpl.Funcs(outputs.TemplateFuncs)
	}

	dc.mo = &formatters.MarshalOptions{
		Format:     newCfg.Format,
		OverrideTS: newCfg.OverrideTimestamps,
	}

	k.dynCfg.Store(dc)
	config, err := k.createConfigFor(newCfg)
	if err != nil {
		return err
	}
	k.rootCtx = ctx
	ctx, k.cancelFn = context.WithCancel(k.rootCtx)
	k.wg.Add(newCfg.NumWorkers)
	for i := 0; i < newCfg.NumWorkers; i++ {
		cfg := *config
		cfg.ClientID = fmt.Sprintf("%s-%d", config.ClientID, i)
		go k.worker(ctx, i, &cfg, *k.msgChan.Load())
	}
	return nil
}

func (k *kafkaOutput) Validate(cfg map[string]any) error {
	ncfg := new(config)
	err := outputs.DecodeConfig(cfg, ncfg)
	if err != nil {
		return err
	}
	err = k.setDefaultsFor(ncfg)
	if err != nil {
		return err
	}
	_, err = gtemplate.CreateTemplate("target-template", ncfg.TargetTemplate)
	if err != nil {
		return err
	}
	_, err = gtemplate.CreateTemplate("msg-template", ncfg.MsgTemplate)
	if err != nil {
		return err
	}
	return nil
}

func (k *kafkaOutput) Update(ctx context.Context, cfg map[string]any) error {
	newCfg := new(config)
	if err := outputs.DecodeConfig(cfg, newCfg); err != nil {
		return err
	}
	currCfg := k.cfg.Load()
	if newCfg.Name == "" && currCfg != nil {
		newCfg.Name = currCfg.Name
	}

	err := k.setDefaultsFor(newCfg)
	if err != nil {
		return err
	}

	swapChannel := channelNeedsSwap(currCfg, newCfg)
	restartWorkers := needsWorkerRestart(currCfg, newCfg)
	rebuildProcessors := slices.Compare(currCfg.EventProcessors, newCfg.EventProcessors) != 0

	var targetTpl *template.Template
	if newCfg.TargetTemplate == "" {
		targetTpl = outputs.DefaultTargetTemplate
	} else if newCfg.AddTarget != "" {
		t, err := gtemplate.CreateTemplate("target-template", newCfg.TargetTemplate)
		if err != nil {
			return err
		}
		targetTpl = t.Funcs(outputs.TemplateFuncs)
	} else {
		targetTpl = outputs.DefaultTargetTemplate
	}

	var msgTpl *template.Template
	if newCfg.MsgTemplate != "" {
		t, err := gtemplate.CreateTemplate("msg-template", newCfg.MsgTemplate)
		if err != nil {
			return err
		}
		msgTpl = t.Funcs(outputs.TemplateFuncs)
	}

	dc := &dynConfig{
		targetTpl: targetTpl,
		msgTpl:    msgTpl,
		mo: &formatters.MarshalOptions{
			Format:     newCfg.Format,
			OverrideTS: newCfg.OverrideTimestamps,
		},
	}

	prevDC := k.dynCfg.Load()
	if rebuildProcessors {
		dc.evps, err = k.buildEventProcessors(log.New(os.Stderr, fmt.Sprintf(loggingPrefixTpl, newCfg.Name), utils.DefaultLoggingFlags), newCfg.EventProcessors)
		if err != nil {
			return err
		}
	} else if prevDC != nil {
		dc.evps = prevDC.evps
	}

	k.dynCfg.Store(dc)
	k.cfg.Store(newCfg)

	if swapChannel || restartWorkers {
		var newChan chan *outputs.ProtoMsg
		if swapChannel {
			newChan = make(chan *outputs.ProtoMsg, newCfg.BufferSize)
		} else {
			newChan = *k.msgChan.Load()
		}

		baseCfg, err := k.createConfigFor(newCfg)
		if err != nil {
			return err
		}
		runCtx, cancel := context.WithCancel(k.rootCtx)
		newWG := new(sync.WaitGroup)
		// save old pointers
		oldCancel := k.cancelFn
		oldWG := k.wg
		oldMsgChan := *k.msgChan.Load()

		// swap
		k.cancelFn = cancel
		k.wg = newWG
		k.msgChan.Store(&newChan)

		k.wg.Add(currCfg.NumWorkers)
		for i := 0; i < currCfg.NumWorkers; i++ {
			cfgCopy := *baseCfg
			cfgCopy.ClientID = fmt.Sprintf("%s-%d", baseCfg.ClientID, i)
			go k.worker(runCtx, i, &cfgCopy, newChan)
		}

		drainDone := make(chan struct{})
		go func() {
			defer close(drainDone)
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-oldMsgChan:
					if !ok {
						return
					}
					select {
					case newChan <- msg:
					default:
					}
				default:
					return
				}
			}
		}()
		// wait for drain to complete
		<-drainDone
		// cancel old workers and loops
		if oldCancel != nil {
			oldCancel()
		}
		if oldWG != nil {
			oldWG.Wait()
		}
	}
	k.logger.Printf("updated kafka output: %s", k.String())
	return nil
}

func (k *kafkaOutput) setDefaultsFor(cfg *config) error {
	if cfg.Format == "" {
		cfg.Format = defaultFormat
	}
	if !(cfg.Format == "event" || cfg.Format == "protojson" || cfg.Format == "prototext" || cfg.Format == "proto" || cfg.Format == "json") {
		return fmt.Errorf("unsupported output format '%s' for output type kafka", cfg.Format)
	}
	if cfg.Address == "" {
		cfg.Address = defaultAddress
	}
	if cfg.Topic == "" {
		cfg.Topic = defaultKafkaTopic
	}
	if cfg.MaxRetry == 0 {
		cfg.MaxRetry = defaultKafkaMaxRetry
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultKafkaTimeout
	}
	if cfg.RecoveryWaitTime <= 0 {
		cfg.RecoveryWaitTime = defaultRecoveryWaitTime
	}
	if cfg.NumWorkers <= 0 {
		cfg.NumWorkers = defaultNumWorkers
	}
	if cfg.Name == "" {
		cfg.Name = "gnmic-" + uuid.New().String()
	}
	if cfg.SASL == nil {
		return nil
	}
	cfg.SASL.Mechanism = strings.ToUpper(cfg.SASL.Mechanism)
	switch cfg.SASL.Mechanism {
	case "":
		cfg.SASL.Mechanism = "PLAIN"
	case "OAUTHBEARER":
		if cfg.SASL.TokenURL == "" {
			return errors.New("missing token-url for kafka SASL mechanism OAUTHBEARER")
		}
	}

	switch cfg.RequiredAcks {
	case requiredAcksNoResponse:
	case requiredAcksWaitForLocal:
	case requiredAcksWaitForAll:
	case "":
		cfg.RequiredAcks = requiredAcksWaitForLocal
	default:
		return fmt.Errorf("unknown `required-acks` value %s: must be one of %q, %q or %q", cfg.RequiredAcks, requiredAcksNoResponse, requiredAcksWaitForLocal, requiredAcksWaitForAll)
	}
	return nil
}

func (k *kafkaOutput) UpdateProcessor(name string, pcfg map[string]any) error {
	cfg := k.cfg.Load()
	dc := k.dynCfg.Load()

	newEvps, changed, err := outputs.UpdateProcessorInSlice(
		k.srcLogger,
		k.store,
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
		k.dynCfg.Store(&newDC)
		k.logger.Printf("updated event processor %s", name)
	}
	return nil
}

// Write //
func (k *kafkaOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	currentCfg := k.cfg.Load()
	if rsp == nil {
		return
	}

	msgChan := *k.msgChan.Load()
	wctx, cancel := context.WithTimeout(ctx, currentCfg.Timeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return
	case msgChan <- outputs.NewProtoMsg(rsp, meta):
	case <-k.closeSig:
		return
	case <-wctx.Done():
		if currentCfg.Debug {
			k.logger.Printf("writing expired after %s, Kafka output might not be initialized", currentCfg.Timeout)
		}
		if currentCfg.EnableMetrics {
			kafkaNumberOfFailSendMsgs.WithLabelValues(currentCfg.Name, currentCfg.Name, "timeout").Inc()
		}
		return
	}
}

func (k *kafkaOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {}

// Close //
func (k *kafkaOutput) Close() error {
	k.cancelFn()
	k.wg.Wait()
	k.closeOnce.Do(func() {
		close(k.closeSig)
	})
	k.logger.Printf("closed kafka output: %s", k.String())
	return nil
}

func (k *kafkaOutput) worker(ctx context.Context, idx int, kafkaCfg *sarama.Config, msgChan <-chan *outputs.ProtoMsg) {
	currentCfg := k.cfg.Load()
	if currentCfg.SyncProducer {
		k.syncProducerWorker(ctx, idx, kafkaCfg, msgChan)
		return
	}
	k.asyncProducerWorker(ctx, idx, kafkaCfg, msgChan)
}

func (k *kafkaOutput) asyncProducerWorker(ctx context.Context, idx int, kafkaCfg *sarama.Config, msgChan <-chan *outputs.ProtoMsg) {
	var producer sarama.AsyncProducer
	var err error
	defer k.wg.Done()
	workerLogPrefix := fmt.Sprintf("worker-%d", idx)
	k.logger.Printf("%s starting", workerLogPrefix)
CRPROD:
	if ctx.Err() != nil {
		return
	}
	cfg := k.cfg.Load()
	producer, err = sarama.NewAsyncProducer(strings.Split(cfg.Address, ","), kafkaCfg)
	if err != nil {
		k.logger.Printf("%s failed to create kafka producer: %v", workerLogPrefix, err)
		time.Sleep(cfg.RecoveryWaitTime)
		goto CRPROD
	}
	defer producer.Close()
	k.logger.Printf("%s initialized kafka producer: %s", workerLogPrefix, k.String())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-producer.Successes():
				if !ok {
					return
				}
				cfg := k.cfg.Load()
				if cfg.EnableMetrics {
					start, ok := msg.Metadata.(time.Time)
					if ok {
						kafkaSendDuration.WithLabelValues(cfg.Name, kafkaCfg.ClientID).Set(float64(time.Since(start).Nanoseconds()))
					}
					kafkaNumberOfSentMsgs.WithLabelValues(cfg.Name, kafkaCfg.ClientID).Inc()
					kafkaNumberOfSentBytes.WithLabelValues(cfg.Name, kafkaCfg.ClientID).Add(float64(msg.Value.Length()))
				}
			case err, ok := <-producer.Errors():
				if !ok {
					return
				}
				cfg := k.cfg.Load()
				if cfg.Debug {
					k.logger.Printf("%s failed to send a kafka msg to topic '%s': %v", workerLogPrefix, err.Msg.Topic, err.Err)
				}
				if cfg.EnableMetrics {
					kafkaNumberOfFailSendMsgs.WithLabelValues(cfg.Name, kafkaCfg.ClientID, "send_error").Inc()
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			k.logger.Printf("%s shutting down", workerLogPrefix)
			return
		case m, ok := <-msgChan:
			if !ok {
				return
			}
			pmsg := m.GetMsg()
			cfg := k.cfg.Load()
			dc := k.dynCfg.Load()
			pmsg, err = outputs.AddSubscriptionTarget(pmsg, m.GetMeta(), cfg.AddTarget, dc.targetTpl)
			if err != nil {
				k.logger.Printf("failed to add target to the response: %v", err)
			}
			bb, err := outputs.Marshal(pmsg, m.GetMeta(), dc.mo, cfg.SplitEvents, dc.evps...)
			if err != nil {
				if cfg.Debug {
					k.logger.Printf("%s failed marshaling proto msg: %v", workerLogPrefix, err)
				}
				if cfg.EnableMetrics {
					kafkaNumberOfFailSendMsgs.WithLabelValues(cfg.Name, kafkaCfg.ClientID, "marshal_error").Inc()
				}
				continue
			}
			if len(bb) == 0 {
				continue
			}
			for _, b := range bb {
				if dc.msgTpl != nil {
					b, err = outputs.ExecTemplate(b, dc.msgTpl)
					if err != nil {
						if cfg.Debug {
							log.Printf("failed to execute template: %v", err)
						}
						kafkaNumberOfFailSendMsgs.WithLabelValues(cfg.Name, kafkaCfg.ClientID, "template_error").Inc()
						continue
					}
				}

				topic := k.selectTopic(m.GetMeta())
				msg := &sarama.ProducerMessage{
					Topic: topic,
					Value: sarama.ByteEncoder(b),
				}
				if cfg.InsertKey {
					msg.Key = sarama.ByteEncoder(k.partitionKey(m.GetMeta()))
				}
				var start time.Time
				if cfg.EnableMetrics {
					start = time.Now()
					msg.Metadata = start
				}
				producer.Input() <- msg
			}
		}
	}
}

func (k *kafkaOutput) syncProducerWorker(ctx context.Context, idx int, kafkaCfg *sarama.Config, msgChan <-chan *outputs.ProtoMsg) {
	var producer sarama.SyncProducer
	var err error
	defer k.wg.Done()
	workerLogPrefix := fmt.Sprintf("worker-%d", idx)
	k.logger.Printf("%s starting", workerLogPrefix)
CRPROD:
	cfg := k.cfg.Load()
	producer, err = sarama.NewSyncProducer(strings.Split(cfg.Address, ","), kafkaCfg)
	if err != nil {
		k.logger.Printf("%s failed to create kafka producer: %v", workerLogPrefix, err)
		time.Sleep(cfg.RecoveryWaitTime)
		goto CRPROD
	}
	defer producer.Close()
	k.logger.Printf("%s initialized kafka producer: %s", workerLogPrefix, k.String())
	for {
		select {
		case <-ctx.Done():
			k.logger.Printf("%s shutting down", workerLogPrefix)
			return
		case m, ok := <-msgChan:
			if !ok {
				return
			}
			pmsg := m.GetMsg()
			cfg := k.cfg.Load()
			dc := k.dynCfg.Load()
			pmsg, err = outputs.AddSubscriptionTarget(pmsg, m.GetMeta(), cfg.AddTarget, dc.targetTpl)
			if err != nil {
				k.logger.Printf("failed to add target to the response: %v", err)
			}
			bb, err := outputs.Marshal(pmsg, m.GetMeta(), dc.mo, cfg.SplitEvents, dc.evps...)
			if err != nil {
				if cfg.Debug {
					k.logger.Printf("%s failed marshaling proto msg: %v", workerLogPrefix, err)
				}
				if cfg.EnableMetrics {
					kafkaNumberOfFailSendMsgs.WithLabelValues(cfg.Name, kafkaCfg.ClientID, "marshal_error").Inc()
				}
				continue
			}
			if len(bb) == 0 {
				continue
			}
			for _, b := range bb {
				if dc.msgTpl != nil {
					b, err = outputs.ExecTemplate(b, dc.msgTpl)
					if err != nil {
						if cfg.Debug {
							log.Printf("failed to execute template: %v", err)
						}
						kafkaNumberOfFailSendMsgs.WithLabelValues(cfg.Name, kafkaCfg.ClientID, "template_error").Inc()
						continue
					}
				}

				topic := k.selectTopic(m.GetMeta())
				msg := &sarama.ProducerMessage{
					Topic: topic,
					Value: sarama.ByteEncoder(b),
				}
				if cfg.InsertKey {
					msg.Key = sarama.ByteEncoder(k.partitionKey(m.GetMeta()))
				}
				var start time.Time
				if cfg.EnableMetrics {
					start = time.Now()
				}
				_, _, err = producer.SendMessage(msg)
				if err != nil {
					if cfg.Debug {
						k.logger.Printf("%s failed to send a kafka msg to topic '%s': %v", workerLogPrefix, topic, err)
					}
					if cfg.EnableMetrics {
						kafkaNumberOfFailSendMsgs.WithLabelValues(cfg.Name, kafkaCfg.ClientID, "send_error").Inc()
					}
					producer.Close()
					time.Sleep(cfg.RecoveryWaitTime)
					goto CRPROD
				}
				if cfg.EnableMetrics {
					kafkaSendDuration.WithLabelValues(cfg.Name, kafkaCfg.ClientID).Set(float64(time.Since(start).Nanoseconds()))
					kafkaNumberOfSentMsgs.WithLabelValues(cfg.Name, kafkaCfg.ClientID).Inc()
					kafkaNumberOfSentBytes.WithLabelValues(cfg.Name, kafkaCfg.ClientID).Add(float64(len(b)))
				}
			}
		}
	}
}

func (k *kafkaOutput) createConfigFor(c *config) (*sarama.Config, error) {
	cfg := sarama.NewConfig()
	cfg.ClientID = c.Name
	if c.KafkaVersion != "" {
		var err error
		cfg.Version, err = sarama.ParseKafkaVersion(c.KafkaVersion)
		if err != nil {
			return nil, err
		}
	}
	// SASL_PLAINTEXT or SASL_SSL
	if c.SASL != nil {
		cfg.Net.SASL.Enable = true
		cfg.Net.SASL.User = c.SASL.User
		cfg.Net.SASL.Password = c.SASL.Password
		cfg.Net.SASL.Mechanism = sarama.SASLMechanism(c.SASL.Mechanism)
		switch cfg.Net.SASL.Mechanism {
		case sarama.SASLTypeSCRAMSHA256:
			cfg.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &XDGSCRAMClient{HashGeneratorFcn: SHA256}
			}
		case sarama.SASLTypeSCRAMSHA512:
			cfg.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &XDGSCRAMClient{HashGeneratorFcn: SHA512}
			}
		case sarama.SASLTypeOAuth:
			cfg.Net.SASL.TokenProvider = pkgutils.NewTokenProvider(cfg.Net.SASL.User, cfg.Net.SASL.Password, c.SASL.TokenURL)
		}
	}
	// SSL or SASL_SSL
	if c.TLS != nil {
		var err error
		cfg.Net.TLS.Enable = true
		cfg.Net.TLS.Config, err = utils.NewTLSConfig(
			c.TLS.CaFile,
			c.TLS.CertFile,
			c.TLS.KeyFile,
			"",
			c.TLS.SkipVerify,
			false)
		if err != nil {
			return nil, err
		}
	}

	cfg.Producer.Retry.Max = c.MaxRetry
	cfg.Producer.Return.Successes = true
	cfg.Producer.Timeout = c.Timeout
	cfg.Producer.Flush.Frequency = c.FlushFrequency
	switch c.RequiredAcks {
	case requiredAcksNoResponse:
	case requiredAcksWaitForLocal:
		cfg.Producer.RequiredAcks = sarama.WaitForLocal
	case requiredAcksWaitForAll:
		cfg.Producer.RequiredAcks = sarama.WaitForAll
	}

	cfg.Metadata.Full = false

	switch c.CompressionCodec {
	case "gzip":
		cfg.Producer.Compression = sarama.CompressionGZIP
	case "snappy":
		cfg.Producer.Compression = sarama.CompressionSnappy
	case "zstd":
		cfg.Producer.Compression = sarama.CompressionZSTD
	case "lz4":
		cfg.Producer.Compression = sarama.CompressionLZ4
	default:
		cfg.Producer.Compression = defaultCompressionCodec
	}

	return cfg, nil
}

const (
	partitionKeyTemplate = "%s:::%s"
)

func (k *kafkaOutput) partitionKey(m outputs.Meta) []byte {
	b := bytesBufferPool.Get().(*bytes.Buffer)
	defer func() {
		b.Reset()
		bytesBufferPool.Put(b)
	}()
	fmt.Fprintf(b, partitionKeyTemplate, m["source"], m["subscription-name"])
	return b.Bytes()
}

func (k *kafkaOutput) selectTopic(m outputs.Meta) string {
	cfg := k.cfg.Load()
	if cfg.TopicPrefix == "" {
		return cfg.Topic
	}

	sb := stringBuilderPool.Get().(*strings.Builder)
	defer func() {
		sb.Reset()
		stringBuilderPool.Put(sb)
	}()
	sb.WriteString(cfg.TopicPrefix)
	if subname, ok := m["subscription-name"]; ok {
		sb.WriteString("_")
		sb.WriteString(subname)
	}
	if s, ok := m["source"]; ok {
		sb.WriteString("_")
		for _, r := range s {
			if r == ':' {
				sb.WriteRune('_')
			} else {
				sb.WriteRune(r)
			}
		}
	}
	return sb.String()
}

// config swap requirements:

// decides if we need to rebuild sarama.Config and producers
func needsProducerRestart(old, nw *config) bool {
	if old == nil || nw == nil {
		return true
	}
	// anything that maps into sarama.Config or producer type
	if old.Address != nw.Address ||
		!old.TLS.Equal(nw.TLS) ||
		!saslEq(old.SASL, nw.SASL) ||
		old.KafkaVersion != nw.KafkaVersion ||
		old.MaxRetry != nw.MaxRetry ||
		old.Timeout != nw.Timeout ||
		old.FlushFrequency != nw.FlushFrequency ||
		old.RequiredAcks != nw.RequiredAcks ||
		old.CompressionCodec != nw.CompressionCodec ||
		old.SyncProducer != nw.SyncProducer ||
		old.Name != nw.Name {
		return true
	}
	return false
}

func needsWorkerRestart(old, nw *config) bool {
	if old == nil || nw == nil {
		return true
	}
	// producer dependencies OR worker count change
	return needsProducerRestart(old, nw) || old.NumWorkers != nw.NumWorkers
}

func channelNeedsSwap(old, nw *config) bool {
	if old != nil && nw != nil {
		return old.BufferSize != nw.BufferSize
	}
	return true
}

func saslEq(a, b *types.SASL) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.User == b.User &&
		a.Password == b.Password &&
		strings.EqualFold(a.Mechanism, b.Mechanism) &&
		a.TokenURL == b.TokenURL
}
