// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package kafka_input

import (
	"bytes"
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
	"time"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/inputs"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/openconfig/gnmic/pkg/pipeline"
	pkgutils "github.com/openconfig/gnmic/pkg/utils"
	"github.com/zestor-dev/zestor/store"
	"google.golang.org/protobuf/proto"
)

const (
	loggingPrefix            = "[kafka_input] "
	defaultFormat            = "event"
	defaultTopic             = "telemetry"
	defaultNumWorkers        = 1
	defaultSessionTimeout    = 10 * time.Second
	defaultHeartbeatInterval = 3 * time.Second
	defaultRecoveryWaitTime  = 2 * time.Second
	defaultAddress           = "localhost:9092"
	defaultGroupID           = "gnmic-consumers"
)

var defaultVersion = sarama.V2_5_0_0

var openSquareBracket = []byte("[")
var openCurlyBrace = []byte("{")

func init() {
	inputs.Register("kafka", func() inputs.Input {
		return &KafkaInput{
			confLock: new(sync.RWMutex),
			cfg:      new(atomic.Pointer[config]),
			dynCfg:   new(atomic.Pointer[dynConfig]),
			logger:   log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
			wg:       new(sync.WaitGroup),
		}
	})
}

// KafkaInput //
type KafkaInput struct {
	// ensure only one Update or UpdateProcessor operation
	// are performed at a time
	confLock *sync.RWMutex

	inputs.BaseInput
	cfg    *atomic.Pointer[config]
	dynCfg *atomic.Pointer[dynConfig]

	ctx    context.Context
	cfn    context.CancelFunc
	logger *log.Logger

	wg       *sync.WaitGroup
	outputs  []outputs.Output // used when the cmd is subscribe
	store    store.Store[any]
	pipeline chan *pipeline.Msg
}

type dynConfig struct {
	evps       []formatters.EventProcessor
	outputsMap map[string]struct{} // used when the cmd is collector
}

// config //
type config struct {
	Name              string           `mapstructure:"name,omitempty"`
	Address           string           `mapstructure:"address,omitempty"`
	Topics            string           `mapstructure:"topics,omitempty"`
	SASL              *types.SASL      `mapstructure:"sasl,omitempty"`
	TLS               *types.TLSConfig `mapstructure:"tls,omitempty"`
	GroupID           string           `mapstructure:"group-id,omitempty"`
	SessionTimeout    time.Duration    `mapstructure:"session-timeout,omitempty"`
	HeartbeatInterval time.Duration    `mapstructure:"heartbeat-interval,omitempty"`
	RecoveryWaitTime  time.Duration    `mapstructure:"recovery-wait-time,omitempty"`
	Version           string           `mapstructure:"version,omitempty"`
	Format            string           `mapstructure:"format,omitempty"`
	Debug             bool             `mapstructure:"debug,omitempty"`
	NumWorkers        int              `mapstructure:"num-workers,omitempty"`
	Outputs           []string         `mapstructure:"outputs,omitempty"`
	EventProcessors   []string         `mapstructure:"event-processors,omitempty"`

	kafkaVersion sarama.KafkaVersion
}

func (k *KafkaInput) Start(ctx context.Context, name string, cfg map[string]interface{}, opts ...inputs.Option) error {
	k.confLock.Lock()
	defer k.confLock.Unlock()

	newCfg := new(config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	if newCfg.Name == "" {
		newCfg.Name = name
	}
	options := &inputs.InputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}
	k.store = options.Store
	k.pipeline = options.Pipeline

	k.setName(options.Name, newCfg)
	k.setLogger(options.Logger)
	outputs, outputsMap := k.getOutputs(options.Outputs, newCfg)
	k.outputs = outputs
	evps, err := k.buildEventProcessors(options.Logger, newCfg.EventProcessors)
	if err != nil {
		return err
	}
	err = k.setDefaultsFor(newCfg)
	if err != nil {
		return err
	}

	k.cfg.Store(newCfg)

	dc := &dynConfig{
		evps:       evps,
		outputsMap: outputsMap,
	}

	k.dynCfg.Store(dc)
	k.ctx = ctx                // save context for worker restarts
	var runCtx context.Context // create a run context for the workers
	runCtx, k.cfn = context.WithCancel(ctx)
	k.logger.Printf("input starting with config: %+v", newCfg)
	k.wg.Add(newCfg.NumWorkers)
	for i := 0; i < newCfg.NumWorkers; i++ {
		go k.worker(runCtx, i)
	}
	return nil
}

func (k *KafkaInput) Validate(cfg map[string]any) error {
	newCfg := new(config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	return k.setDefaultsFor(newCfg)
}

// Update updates the input configuration and restarts the workers if
// necessary.
// It works only when the command is collector (not subscribe).
func (k *KafkaInput) Update(cfg map[string]any) error {
	k.confLock.Lock()
	defer k.confLock.Unlock()

	newCfg := new(config)
	err := outputs.DecodeConfig(cfg, newCfg)
	if err != nil {
		return err
	}
	k.setDefaultsFor(newCfg)
	currCfg := k.cfg.Load()

	restartWorkers := needsWorkerRestart(currCfg, newCfg)
	rebuildProcessors := slices.Compare(currCfg.EventProcessors, newCfg.EventProcessors) != 0
	// build new dynamic config
	dc := &dynConfig{
		outputsMap: make(map[string]struct{}),
	}
	for _, o := range newCfg.Outputs {
		dc.outputsMap[o] = struct{}{}
	}

	prevDC := k.dynCfg.Load()

	if rebuildProcessors {
		dc.evps, err = k.buildEventProcessors(k.logger, newCfg.EventProcessors)
		if err != nil {
			return err
		}
	} else if prevDC != nil {
		dc.evps = prevDC.evps
	}

	k.dynCfg.Store(dc)
	k.cfg.Store(newCfg)

	if restartWorkers {
		runCtx, cancel := context.WithCancel(k.ctx)
		newWG := new(sync.WaitGroup)
		// save old pointers
		oldCancel := k.cfn
		oldWG := k.wg
		// swap
		k.cfn = cancel
		k.wg = newWG

		k.wg.Add(newCfg.NumWorkers)
		for i := 0; i < newCfg.NumWorkers; i++ {
			go k.worker(runCtx, i)
		}
		// cancel old workers and loops
		if oldCancel != nil {
			oldCancel()
		}
		if oldWG != nil {
			oldWG.Wait()
		}
	}
	return nil
}

func (k *KafkaInput) UpdateProcessor(name string, pcfg map[string]any) error {
	k.confLock.Lock()
	defer k.confLock.Unlock()

	cfg := k.cfg.Load()
	dc := k.dynCfg.Load()

	newEvps, changed, err := inputs.UpdateProcessorInSlice(
		k.logger,
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

func (k *KafkaInput) worker(ctx context.Context, idx int) {
	defer k.wg.Done()

	workerLogPrefix := fmt.Sprintf("worker-%d", idx)
	k.logger.Printf("%s starting", workerLogPrefix)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		k.logger.Printf("worker %d loading config", idx)
		cfg := k.cfg.Load()
		wCfg := *cfg
		wCfg.Name = fmt.Sprintf("%s-%d", wCfg.Name, idx)
		// scoped connection, subscription and cleanup
		err := k.doWork(ctx, &wCfg, workerLogPrefix, idx)
		if err != nil {
			k.logger.Printf("%s Kafka client failed: %v", workerLogPrefix, err)
		}

		// backoff before retry
		select {
		case <-ctx.Done():
			return
		case <-time.After(wCfg.RecoveryWaitTime):
		}
	}
}

// scoped connection, subscription and cleanup
func (k *KafkaInput) doWork(ctx context.Context, wCfg *config, workerLogPrefix string, idx int) error {
	saramaConfig, err := k.createConfig(wCfg)
	if err != nil {
		return fmt.Errorf("create Kafka config: %w", err)
	}
	saramaConfig.ClientID = fmt.Sprintf("%s-%d", wCfg.Name, idx)

	consumerGrp, err := sarama.NewConsumerGroup(strings.Split(wCfg.Address, ","), wCfg.GroupID, saramaConfig)
	if err != nil {
		return fmt.Errorf("create consumer group: %w", err)
	}
	defer consumerGrp.Close()

	cons := &consumer{
		ready:   make(chan bool),
		msgChan: make(chan *sarama.ConsumerMessage),
	}
	stopConsume := make(chan struct{})
	go func() {
		var err error
		for {
			select {
			case <-ctx.Done():
				return
			case <-stopConsume:
				return
			default:
			}
			err = consumerGrp.Consume(ctx, strings.Split(wCfg.Topics, ","), cons)
			if err != nil {
				if wCfg.Debug {
					k.logger.Printf("%s failed to start consumer, topics=%q, group=%q : %v", workerLogPrefix, wCfg.Topics, wCfg.GroupID, err)
				}
				continue
			}
			cons.ready = make(chan bool)
		}
	}()
	// wait for the consumer to be ready
	select {
	case <-ctx.Done():
		return nil
	case <-cons.ready:
		k.logger.Printf("%s kafka consumer ready", workerLogPrefix)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case m := <-cons.msgChan:
			if len(m.Value) == 0 {
				continue
			}
			// load current config for dynamic fields like Format
			cfg := k.cfg.Load()
			if cfg.Debug {
				k.logger.Printf("%s client=%s received msg, topic=%s, partition=%d, key=%q, length=%d, value=%s", workerLogPrefix, saramaConfig.ClientID, m.Topic, m.Partition, string(m.Key), len(m.Value), string(m.Value))
			}

			dc := k.dynCfg.Load()
			switch cfg.Format {
			case "event":
				m.Value = bytes.TrimSpace(m.Value)
				evMsgs := make([]*formatters.EventMsg, 1)
				var err error
				switch {
				case len(m.Value) == 0:
					continue
				case m.Value[0] == openSquareBracket[0]:
					err = json.Unmarshal(m.Value, &evMsgs)
				case m.Value[0] == openCurlyBrace[0]:
					evMsgs[0] = &formatters.EventMsg{}
					err = json.Unmarshal(m.Value, evMsgs[0])
				}
				if err != nil {
					if cfg.Debug {
						k.logger.Printf("%s failed to unmarshal event msg: %v", workerLogPrefix, err)
					}
					continue
				}

				for _, p := range dc.evps {
					evMsgs = p.Apply(evMsgs...)
				}

				if k.pipeline != nil {
					select {
					case <-ctx.Done():
						return nil
					case k.pipeline <- &pipeline.Msg{
						Events:  evMsgs,
						Outputs: dc.outputsMap,
					}:
					default:
						k.logger.Printf("pipeline channel is full, dropping event")
					}
				}
				for _, o := range k.outputs {
					for _, ev := range evMsgs {
						o.WriteEvent(ctx, ev)
					}
				}

			case "proto":
				protoMsg := new(gnmi.SubscribeResponse)
				if err := proto.Unmarshal(m.Value, protoMsg); err != nil {
					if cfg.Debug {
						k.logger.Printf("%s failed to unmarshal proto msg: %v", workerLogPrefix, err)
					}
					continue
				}
				fmt.Printf("m.Key: %s\n", string(m.Key))
				meta := k.partitionKeyToMeta(m.Key)
				fmt.Printf("meta: %+v\n", meta)
				if k.pipeline != nil {
					select {
					case <-ctx.Done():
						return nil
					case k.pipeline <- &pipeline.Msg{
						Msg:     protoMsg,
						Meta:    meta,
						Outputs: dc.outputsMap,
					}:
					default:
						k.logger.Printf("pipeline channel is full, dropping message")
					}
				}
				for _, o := range k.outputs {
					o.Write(ctx, protoMsg, meta)
				}
			}
		case err := <-consumerGrp.Errors():
			cfg := k.cfg.Load()
			k.logger.Printf("%s client=%s, consumer-group=%s error: %v", workerLogPrefix, saramaConfig.ClientID, cfg.GroupID, err)

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(cfg.RecoveryWaitTime):
			}
			close(stopConsume)
			// restart worker in case of error
			go k.doWork(ctx, cfg, workerLogPrefix, idx)
			return nil
		}
	}
}

func (k *KafkaInput) Close() error {
	if k.cfn != nil {
		k.cfn()
	}
	if k.wg != nil {
		k.wg.Wait()
	}
	return nil
}

const (
	partitionKeySeparator = ":::"
)

func (k *KafkaInput) partitionKeyToMeta(key []byte) outputs.Meta {
	if len(key) == 0 {
		return outputs.Meta{}
	}
	parts := strings.SplitN(string(key), partitionKeySeparator, 2)
	if len(parts) != 2 {
		return outputs.Meta{}
	}
	return outputs.Meta{
		"source":            parts[0],
		"subscription-name": parts[1],
	}
}

func (k *KafkaInput) setLogger(logger *log.Logger) {
	if logger != nil {
		k.logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
		sarama.Logger = k.logger
	}
}

func (k *KafkaInput) getOutputs(outs map[string]outputs.Output, cfg *config) ([]outputs.Output, map[string]struct{}) {
	outputs := make([]outputs.Output, 0)

	if len(cfg.Outputs) == 0 {
		for _, o := range outs {
			outputs = append(outputs, o)
		}
		return outputs, nil
	}
	outputsMap := make(map[string]struct{})
	for _, name := range cfg.Outputs {
		outputsMap[name] = struct{}{} // for collector
		if o, ok := outs[name]; ok {  // for subscribe
			outputs = append(outputs, o)
		}
	}
	return outputs, outputsMap
}

func (k *KafkaInput) setName(name string, cfg *config) {
	sb := strings.Builder{}
	if name != "" {
		sb.WriteString(name)
		sb.WriteString("-")
	}
	sb.WriteString(cfg.Name)
	sb.WriteString("-kafka-cons")
	cfg.Name = sb.String()
}

func (k *KafkaInput) buildEventProcessors(logger *log.Logger, eventProcessors []string) ([]formatters.EventProcessor, error) {
	tcs, ps, acts, err := pkgutils.GetConfigMaps(k.store)
	if err != nil {
		return nil, err
	}
	return formatters.MakeEventProcessors(
		logger,
		eventProcessors,
		ps,
		tcs,
		acts,
	)
}

// helper funcs

func (k *KafkaInput) setDefaultsFor(cfg *config) error {
	var err error
	if cfg.Version != "" {
		cfg.kafkaVersion, err = sarama.ParseKafkaVersion(cfg.Version)
		if err != nil {
			return err
		}
	} else {
		cfg.kafkaVersion = defaultVersion

	}
	if cfg.Format == "" {
		cfg.Format = defaultFormat
	}
	if !(strings.ToLower(cfg.Format) == "event" || strings.ToLower(cfg.Format) == "proto") {
		return fmt.Errorf("unsupported input format")
	}
	cfg.Format = strings.ToLower(cfg.Format)
	if cfg.Topics == "" {
		cfg.Topics = defaultTopic
	}
	if cfg.Address == "" {
		cfg.Address = defaultAddress
	}
	if cfg.NumWorkers <= 0 {
		cfg.NumWorkers = defaultNumWorkers
	}
	if cfg.SessionTimeout <= 2*time.Millisecond {
		cfg.SessionTimeout = defaultSessionTimeout
	}
	if cfg.HeartbeatInterval <= 1*time.Millisecond {
		cfg.HeartbeatInterval = defaultHeartbeatInterval
	}
	if cfg.GroupID == "" {
		cfg.GroupID = defaultGroupID
	}
	if cfg.RecoveryWaitTime <= 0 {
		cfg.RecoveryWaitTime = defaultRecoveryWaitTime
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
	return nil
}

func (k *KafkaInput) createConfig(cfg *config) (*sarama.Config, error) {
	saramaCfg := sarama.NewConfig()
	saramaCfg.Version = cfg.kafkaVersion
	saramaCfg.Consumer.Return.Errors = true
	saramaCfg.Consumer.Group.Session.Timeout = cfg.SessionTimeout
	saramaCfg.Consumer.Group.Heartbeat.Interval = cfg.HeartbeatInterval
	saramaCfg.Consumer.Group.Rebalance.Strategy = sarama.NewBalanceStrategyRange()
	// SASL_PLAINTEXT or SASL_SSL
	if cfg.SASL != nil {
		saramaCfg.Net.SASL.Enable = true
		saramaCfg.Net.SASL.User = cfg.SASL.User
		saramaCfg.Net.SASL.Password = cfg.SASL.Password
		saramaCfg.Net.SASL.Mechanism = sarama.SASLMechanism(cfg.SASL.Mechanism)
		switch saramaCfg.Net.SASL.Mechanism {
		case sarama.SASLTypeSCRAMSHA256:
			saramaCfg.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &XDGSCRAMClient{HashGeneratorFcn: SHA256}
			}
		case sarama.SASLTypeSCRAMSHA512:
			saramaCfg.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &XDGSCRAMClient{HashGeneratorFcn: SHA512}
			}
		case sarama.SASLTypeOAuth:
			saramaCfg.Net.SASL.TokenProvider = pkgutils.NewTokenProvider(saramaCfg.Net.SASL.User, saramaCfg.Net.SASL.Password, cfg.SASL.TokenURL)
		}
	}
	// SSL or SASL_SSL
	if cfg.TLS != nil {
		var err error
		saramaCfg.Net.TLS.Enable = true
		saramaCfg.Net.TLS.Config, err = utils.NewTLSConfig(
			cfg.TLS.CaFile,
			cfg.TLS.CertFile,
			cfg.TLS.KeyFile,
			"",
			cfg.TLS.SkipVerify,
			false)
		if err != nil {
			return nil, err
		}
	}
	return saramaCfg, nil
}

func needsWorkerRestart(old, nw *config) bool {
	if old == nil || nw == nil {
		return true
	}
	return old.NumWorkers != nw.NumWorkers ||
		old.Address != nw.Address ||
		old.Topics != nw.Topics ||
		old.GroupID != nw.GroupID ||
		old.SessionTimeout != nw.SessionTimeout ||
		old.HeartbeatInterval != nw.HeartbeatInterval ||
		old.RecoveryWaitTime != nw.RecoveryWaitTime ||
		old.kafkaVersion != nw.kafkaVersion ||
		!old.TLS.Equal(nw.TLS) ||
		!saslEq(old.SASL, nw.SASL)
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

// consumer
// ref: https://github.com/Shopify/sarama/blob/master/examples/consumergroup/main.go
// consumer represents a Sarama consumer group consumer
type consumer struct {
	ready   chan bool
	msgChan chan *sarama.ConsumerMessage
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (consumer *consumer) Setup(sarama.ConsumerGroupSession) error {
	// Mark the consumer as ready
	close(consumer.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (consumer *consumer) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (consumer *consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for message := range claim.Messages() {
		consumer.msgChan <- message
		session.MarkMessage(message, "")
	}
	return nil
}
