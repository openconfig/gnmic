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
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/Shopify/sarama"
	"github.com/damiannolan/sasl/oauthbearer"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/outputs"
	"github.com/openconfig/gnmic/pkg/types"
	"github.com/openconfig/gnmic/pkg/utils"
)

const (
	defaultKafkaMaxRetry    = 2
	defaultKafkaTimeout     = 5 * time.Second
	defaultKafkaTopic       = "telemetry"
	defaultNumWorkers       = 1
	defaultFormat           = "event"
	defaultRecoveryWaitTime = 10 * time.Second
	defaultAddress          = "localhost:9092"
	loggingPrefix           = "[kafka_output:%s] "
	defaultCompressionCodec = sarama.CompressionNone

	requiredAcksNoResponse   = "no-response"
	requiredAcksWaitForLocal = "wait-for-local"
	requiredAcksWaitForAll   = "wait-for-all"
)

func init() {
	outputs.Register("kafka", func() outputs.Output {
		return &kafkaOutput{
			cfg:    &config{},
			wg:     new(sync.WaitGroup),
			logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

// kafkaOutput //
type kafkaOutput struct {
	cfg      *config
	logger   sarama.StdLogger
	mo       *formatters.MarshalOptions
	cancelFn context.CancelFunc
	msgChan  chan *outputs.ProtoMsg
	wg       *sync.WaitGroup
	evps     []formatters.EventProcessor

	targetTpl *template.Template
	msgTpl    *template.Template
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
	Debug              bool             `mapstructure:"debug,omitempty"`
	BufferSize         int              `mapstructure:"buffer-size,omitempty"`
	OverrideTimestamps bool             `mapstructure:"override-timestamps,omitempty"`
	EnableMetrics      bool             `mapstructure:"enable-metrics,omitempty"`
	EventProcessors    []string         `mapstructure:"event-processors,omitempty"`
}

func (k *kafkaOutput) String() string {
	b, err := json.Marshal(k.cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

func (k *kafkaOutput) SetLogger(logger *log.Logger) {
	if logger != nil {
		sarama.Logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
		k.logger = sarama.Logger
	}
}

func (k *kafkaOutput) SetEventProcessors(ps map[string]map[string]interface{},
	logger *log.Logger,
	tcs map[string]*types.TargetConfig,
	acts map[string]map[string]interface{}) error {
	var err error
	k.evps, err = formatters.MakeEventProcessors(
		logger,
		k.cfg.EventProcessors,
		ps,
		tcs,
		acts,
	)
	if err != nil {
		return err
	}
	return nil
}

// Init /
func (k *kafkaOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, k.cfg)
	if err != nil {
		return err
	}
	if k.cfg.Name == "" {
		k.cfg.Name = name
	}
	for _, opt := range opts {
		if err := opt(k); err != nil {
			return err
		}
	}
	err = k.setDefaults()
	if err != nil {
		return err
	}
	k.msgChan = make(chan *outputs.ProtoMsg, uint(k.cfg.BufferSize))
	k.mo = &formatters.MarshalOptions{
		Format:     k.cfg.Format,
		OverrideTS: k.cfg.OverrideTimestamps,
	}

	if k.cfg.TargetTemplate == "" {
		k.targetTpl = outputs.DefaultTargetTemplate
	} else if k.cfg.AddTarget != "" {
		k.targetTpl, err = gtemplate.CreateTemplate("target-template", k.cfg.TargetTemplate)
		if err != nil {
			return err
		}
		k.targetTpl = k.targetTpl.Funcs(outputs.TemplateFuncs)
	}

	if k.cfg.MsgTemplate != "" {
		k.msgTpl, err = gtemplate.CreateTemplate("msg-template", k.cfg.MsgTemplate)
		if err != nil {
			return err
		}
		k.msgTpl = k.msgTpl.Funcs(outputs.TemplateFuncs)
	}

	config, err := k.createConfig()
	if err != nil {
		return err
	}
	ctx, k.cancelFn = context.WithCancel(ctx)
	k.wg.Add(k.cfg.NumWorkers)
	for i := 0; i < k.cfg.NumWorkers; i++ {
		cfg := *config
		cfg.ClientID = fmt.Sprintf("%s-%d", config.ClientID, i)
		go k.worker(ctx, i, &cfg)
	}
	go func() {
		<-ctx.Done()
		k.Close()
	}()
	return nil
}

func (k *kafkaOutput) setDefaults() error {
	if k.cfg.Format == "" {
		k.cfg.Format = defaultFormat
	}
	if !(k.cfg.Format == "event" || k.cfg.Format == "protojson" || k.cfg.Format == "prototext" || k.cfg.Format == "proto" || k.cfg.Format == "json") {
		return fmt.Errorf("unsupported output format '%s' for output type kafka", k.cfg.Format)
	}
	if k.cfg.Address == "" {
		k.cfg.Address = defaultAddress
	}
	if k.cfg.Topic == "" {
		k.cfg.Topic = defaultKafkaTopic
	}
	if k.cfg.MaxRetry == 0 {
		k.cfg.MaxRetry = defaultKafkaMaxRetry
	}
	if k.cfg.Timeout <= 0 {
		k.cfg.Timeout = defaultKafkaTimeout
	}
	if k.cfg.RecoveryWaitTime <= 0 {
		k.cfg.RecoveryWaitTime = defaultRecoveryWaitTime
	}
	if k.cfg.NumWorkers <= 0 {
		k.cfg.NumWorkers = defaultNumWorkers
	}
	if k.cfg.Name == "" {
		k.cfg.Name = "gnmic-" + uuid.New().String()
	}
	if k.cfg.SASL == nil {
		return nil
	}
	k.cfg.SASL.Mechanism = strings.ToUpper(k.cfg.SASL.Mechanism)
	switch k.cfg.SASL.Mechanism {
	case "":
		k.cfg.SASL.Mechanism = "PLAIN"
	case "OAUTHBEARER":
		if k.cfg.SASL.TokenURL == "" {
			return errors.New("missing token-url for kafka SASL mechanism OAUTHBEARER")
		}
	}

	switch k.cfg.RequiredAcks {
	case requiredAcksNoResponse:
	case requiredAcksWaitForLocal:
	case requiredAcksWaitForAll:
	case "":
		k.cfg.RequiredAcks = requiredAcksWaitForLocal
	default:
		return fmt.Errorf("unknown `required-acks` value %s: must be one of %q, %q or %q", k.cfg.RequiredAcks, requiredAcksNoResponse, requiredAcksWaitForLocal, requiredAcksWaitForAll)
	}
	return nil
}

// Write //
func (k *kafkaOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}

	wctx, cancel := context.WithTimeout(ctx, k.cfg.Timeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return
	case k.msgChan <- outputs.NewProtoMsg(rsp, meta):
	case <-wctx.Done():
		if k.cfg.Debug {
			k.logger.Printf("writing expired after %s, Kafka output might not be initialized", k.cfg.Timeout)
		}
		if k.cfg.EnableMetrics {
			kafkaNumberOfFailSendMsgs.WithLabelValues(k.cfg.Name, "timeout").Inc()
		}
		return
	}
}

func (k *kafkaOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {}

// Close //
func (k *kafkaOutput) Close() error {
	k.cancelFn()
	k.wg.Wait()
	return nil
}

// Metrics //
func (k *kafkaOutput) RegisterMetrics(reg *prometheus.Registry) {
	if !k.cfg.EnableMetrics {
		return
	}
	if reg == nil {
		k.logger.Printf("ERR: output metrics enabled but main registry is not initialized, enable main metrics under `api-server`")
		return
	}
	if err := registerMetrics(reg); err != nil {
		k.logger.Printf("failed to register metric: %v", err)
	}
}

func (k *kafkaOutput) worker(ctx context.Context, idx int, config *sarama.Config) {
	if k.cfg.SyncProducer {
		k.syncProducerWorker(ctx, idx, config)
	}
	k.asyncProducerWorker(ctx, idx, config)
}

func (k *kafkaOutput) asyncProducerWorker(ctx context.Context, idx int, config *sarama.Config) {
	var producer sarama.AsyncProducer
	var err error
	defer k.wg.Done()
	workerLogPrefix := fmt.Sprintf("worker-%d", idx)
	k.logger.Printf("%s starting", workerLogPrefix)
CRPROD:
	producer, err = sarama.NewAsyncProducer(strings.Split(k.cfg.Address, ","), config)
	if err != nil {
		k.logger.Printf("%s failed to create kafka producer: %v", workerLogPrefix, err)
		time.Sleep(k.cfg.RecoveryWaitTime)
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
				if k.cfg.EnableMetrics {
					start, ok := msg.Metadata.(time.Time)
					if ok {
						kafkaSendDuration.WithLabelValues(config.ClientID).Set(float64(time.Since(start).Nanoseconds()))
					}
					kafkaNumberOfSentMsgs.WithLabelValues(config.ClientID).Inc()
					kafkaNumberOfSentBytes.WithLabelValues(config.ClientID).Add(float64(msg.Value.Length()))
				}
			case err, ok := <-producer.Errors():
				if !ok {
					return
				}
				if k.cfg.Debug {
					k.logger.Printf("%s failed to send a kafka msg to topic '%s': %v", workerLogPrefix, err.Msg.Topic, err.Err)
				}
				if k.cfg.EnableMetrics {
					kafkaNumberOfFailSendMsgs.WithLabelValues(config.ClientID, "send_error").Inc()
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			k.logger.Printf("%s shutting down", workerLogPrefix)
			return
		case m := <-k.msgChan:
			pmsg := m.GetMsg()
			pmsg, err = outputs.AddSubscriptionTarget(pmsg, m.GetMeta(), k.cfg.AddTarget, k.targetTpl)
			if err != nil {
				k.logger.Printf("failed to add target to the response: %v", err)
			}
			bb, err := outputs.Marshal(pmsg, m.GetMeta(), k.mo, k.cfg.SplitEvents, k.evps...)
			if err != nil {
				if k.cfg.Debug {
					k.logger.Printf("%s failed marshaling proto msg: %v", workerLogPrefix, err)
				}
				if k.cfg.EnableMetrics {
					kafkaNumberOfFailSendMsgs.WithLabelValues(config.ClientID, "marshal_error").Inc()
				}
				continue
			}
			if len(bb) == 0 {
				continue
			}
			for _, b := range bb {
				if k.msgTpl != nil {
					b, err = outputs.ExecTemplate(b, k.msgTpl)
					if err != nil {
						if k.cfg.Debug {
							log.Printf("failed to execute template: %v", err)
						}
						kafkaNumberOfFailSendMsgs.WithLabelValues(config.ClientID, "template_error").Inc()
						continue
					}
				}

				topic := k.selectTopic(m.GetMeta())
				msg := &sarama.ProducerMessage{
					Topic: topic,
					Value: sarama.ByteEncoder(b),
				}
				if k.cfg.InsertKey {
					msg.Key = sarama.ByteEncoder(k.partitionKey(m.GetMeta()))
				}
				var start time.Time
				if k.cfg.EnableMetrics {
					start = time.Now()
					msg.Metadata = start
				}
				producer.Input() <- msg
			}
		}
	}
}

func (k *kafkaOutput) syncProducerWorker(ctx context.Context, idx int, config *sarama.Config) {
	var producer sarama.SyncProducer
	var err error
	defer k.wg.Done()
	workerLogPrefix := fmt.Sprintf("worker-%d", idx)
	k.logger.Printf("%s starting", workerLogPrefix)
CRPROD:
	producer, err = sarama.NewSyncProducer(strings.Split(k.cfg.Address, ","), config)
	if err != nil {
		k.logger.Printf("%s failed to create kafka producer: %v", workerLogPrefix, err)
		time.Sleep(k.cfg.RecoveryWaitTime)
		goto CRPROD
	}
	defer producer.Close()
	k.logger.Printf("%s initialized kafka producer: %s", workerLogPrefix, k.String())
	for {
		select {
		case <-ctx.Done():
			k.logger.Printf("%s shutting down", workerLogPrefix)
			return
		case m := <-k.msgChan:
			pmsg := m.GetMsg()
			pmsg, err = outputs.AddSubscriptionTarget(pmsg, m.GetMeta(), k.cfg.AddTarget, k.targetTpl)
			if err != nil {
				k.logger.Printf("failed to add target to the response: %v", err)
			}
			bb, err := outputs.Marshal(pmsg, m.GetMeta(), k.mo, k.cfg.SplitEvents, k.evps...)
			if err != nil {
				if k.cfg.Debug {
					k.logger.Printf("%s failed marshaling proto msg: %v", workerLogPrefix, err)
				}
				if k.cfg.EnableMetrics {
					kafkaNumberOfFailSendMsgs.WithLabelValues(config.ClientID, "marshal_error").Inc()
				}
				continue
			}
			if len(bb) == 0 {
				continue
			}
			for _, b := range bb {
				if k.msgTpl != nil {
					b, err = outputs.ExecTemplate(b, k.msgTpl)
					if err != nil {
						if k.cfg.Debug {
							log.Printf("failed to execute template: %v", err)
						}
						kafkaNumberOfFailSendMsgs.WithLabelValues(config.ClientID, "template_error").Inc()
						continue
					}
				}

				topic := k.selectTopic(m.GetMeta())
				msg := &sarama.ProducerMessage{
					Topic: topic,
					Value: sarama.ByteEncoder(b),
				}
				if k.cfg.InsertKey {
					msg.Key = sarama.ByteEncoder(k.partitionKey(m.GetMeta()))
				}
				var start time.Time
				if k.cfg.EnableMetrics {
					start = time.Now()
				}
				_, _, err = producer.SendMessage(msg)
				if err != nil {
					if k.cfg.Debug {
						k.logger.Printf("%s failed to send a kafka msg to topic '%s': %v", workerLogPrefix, topic, err)
					}
					if k.cfg.EnableMetrics {
						kafkaNumberOfFailSendMsgs.WithLabelValues(config.ClientID, "send_error").Inc()
					}
					producer.Close()
					time.Sleep(k.cfg.RecoveryWaitTime)
					goto CRPROD
				}
				if k.cfg.EnableMetrics {
					kafkaSendDuration.WithLabelValues(config.ClientID).Set(float64(time.Since(start).Nanoseconds()))
					kafkaNumberOfSentMsgs.WithLabelValues(config.ClientID).Inc()
					kafkaNumberOfSentBytes.WithLabelValues(config.ClientID).Add(float64(len(b)))
				}
			}
		}
	}
}

func (k *kafkaOutput) SetName(name string) {
	sb := strings.Builder{}
	if name != "" {
		sb.WriteString(name)
		sb.WriteString("-")
	}
	sb.WriteString(k.cfg.Name)
	sb.WriteString("-kafka-prod")
	k.cfg.Name = sb.String()
}

func (k *kafkaOutput) SetClusterName(name string) {}

func (k *kafkaOutput) SetTargetsConfig(map[string]*types.TargetConfig) {}

func (k *kafkaOutput) createConfig() (*sarama.Config, error) {
	cfg := sarama.NewConfig()
	cfg.ClientID = k.cfg.Name
	// SASL_PLAINTEXT or SASL_SSL
	if k.cfg.SASL != nil {
		cfg.Net.SASL.Enable = true
		cfg.Net.SASL.User = k.cfg.SASL.User
		cfg.Net.SASL.Password = k.cfg.SASL.Password
		cfg.Net.SASL.Mechanism = sarama.SASLMechanism(k.cfg.SASL.Mechanism)
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
			cfg.Net.SASL.TokenProvider = oauthbearer.NewTokenProvider(cfg.Net.SASL.User, cfg.Net.SASL.Password, k.cfg.SASL.TokenURL)
		}
	}
	// SSL or SASL_SSL
	if k.cfg.TLS != nil {
		var err error
		cfg.Net.TLS.Enable = true
		cfg.Net.TLS.Config, err = utils.NewTLSConfig(
			k.cfg.TLS.CaFile,
			k.cfg.TLS.CertFile,
			k.cfg.TLS.KeyFile,
			"",
			k.cfg.TLS.SkipVerify,
			false)
		if err != nil {
			return nil, err
		}
	}

	cfg.Producer.Retry.Max = k.cfg.MaxRetry
	if k.cfg.SyncProducer {
		cfg.Producer.Return.Successes = true
	}
	cfg.Producer.Timeout = k.cfg.Timeout
	switch k.cfg.RequiredAcks {
	case requiredAcksNoResponse:
	case requiredAcksWaitForLocal:
		cfg.Producer.RequiredAcks = sarama.WaitForLocal
	case requiredAcksWaitForAll:
		cfg.Producer.RequiredAcks = sarama.WaitForAll
	}

	cfg.Metadata.Full = false

	switch k.cfg.CompressionCodec {
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

func (k *kafkaOutput) partitionKey(m outputs.Meta) []byte {
	b := new(bytes.Buffer)
	fmt.Fprintf(b, "%s_%s", m["source"], m["subscription-name"])
	return b.Bytes()
}

func (k *kafkaOutput) selectTopic(m outputs.Meta) string {
	if k.cfg.TopicPrefix == "" {
		return k.cfg.Topic
	}

	sb := strings.Builder{}
	sb.WriteString(k.cfg.TopicPrefix)
	if subname, ok := m["subscription-name"]; ok {
		sb.WriteString("_")
		sb.WriteString(subname)
	}
	if s, ok := m["source"]; ok {
		source := strings.ReplaceAll(s, ":", "_")
		sb.WriteString("_")
		sb.WriteString(source)
	}
	return sb.String()
}
