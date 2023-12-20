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
)

func init() {
	outputs.Register("kafka", func() outputs.Output {
		return &kafkaOutput{
			Cfg:    &config{},
			wg:     new(sync.WaitGroup),
			logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

// kafkaOutput //
type kafkaOutput struct {
	Cfg      *config
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
	Name               string           `mapstructure:"name,omitempty"`
	SASL               *types.SASL      `mapstructure:"sasl,omitempty"`
	TLS                *types.TLSConfig `mapstructure:"tls,omitempty"`
	MaxRetry           int              `mapstructure:"max-retry,omitempty"`
	Timeout            time.Duration    `mapstructure:"timeout,omitempty"`
	RecoveryWaitTime   time.Duration    `mapstructure:"recovery-wait-time,omitempty"`
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
	b, err := json.Marshal(k.Cfg)
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
		k.Cfg.EventProcessors,
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
	err := outputs.DecodeConfig(cfg, k.Cfg)
	if err != nil {
		return err
	}
	if k.Cfg.Name == "" {
		k.Cfg.Name = name
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
	k.msgChan = make(chan *outputs.ProtoMsg, uint(k.Cfg.BufferSize))
	k.mo = &formatters.MarshalOptions{
		Format:     k.Cfg.Format,
		OverrideTS: k.Cfg.OverrideTimestamps,
	}

	if k.Cfg.TargetTemplate == "" {
		k.targetTpl = outputs.DefaultTargetTemplate
	} else if k.Cfg.AddTarget != "" {
		k.targetTpl, err = gtemplate.CreateTemplate("target-template", k.Cfg.TargetTemplate)
		if err != nil {
			return err
		}
		k.targetTpl = k.targetTpl.Funcs(outputs.TemplateFuncs)
	}

	if k.Cfg.MsgTemplate != "" {
		k.msgTpl, err = gtemplate.CreateTemplate("msg-template", k.Cfg.MsgTemplate)
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
	k.wg.Add(k.Cfg.NumWorkers)
	for i := 0; i < k.Cfg.NumWorkers; i++ {
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
	if k.Cfg.Format == "" {
		k.Cfg.Format = defaultFormat
	}
	if !(k.Cfg.Format == "event" || k.Cfg.Format == "protojson" || k.Cfg.Format == "prototext" || k.Cfg.Format == "proto" || k.Cfg.Format == "json") {
		return fmt.Errorf("unsupported output format '%s' for output type kafka", k.Cfg.Format)
	}
	if k.Cfg.Address == "" {
		k.Cfg.Address = defaultAddress
	}
	if k.Cfg.Topic == "" {
		k.Cfg.Topic = defaultKafkaTopic
	}
	if k.Cfg.MaxRetry == 0 {
		k.Cfg.MaxRetry = defaultKafkaMaxRetry
	}
	if k.Cfg.Timeout <= 0 {
		k.Cfg.Timeout = defaultKafkaTimeout
	}
	if k.Cfg.RecoveryWaitTime <= 0 {
		k.Cfg.RecoveryWaitTime = defaultRecoveryWaitTime
	}
	if k.Cfg.NumWorkers <= 0 {
		k.Cfg.NumWorkers = defaultNumWorkers
	}
	if k.Cfg.Name == "" {
		k.Cfg.Name = "gnmic-" + uuid.New().String()
	}
	if k.Cfg.SASL == nil {
		return nil
	}
	k.Cfg.SASL.Mechanism = strings.ToUpper(k.Cfg.SASL.Mechanism)
	switch k.Cfg.SASL.Mechanism {
	case "":
		k.Cfg.SASL.Mechanism = "PLAIN"
	case "OAUTHBEARER":
		if k.Cfg.SASL.TokenURL == "" {
			return errors.New("missing token-url for kafka SASL mechanism OAUTHBEARER")
		}
	}
	return nil
}

// Write //
func (k *kafkaOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}

	wctx, cancel := context.WithTimeout(ctx, k.Cfg.Timeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return
	case k.msgChan <- outputs.NewProtoMsg(rsp, meta):
	case <-wctx.Done():
		if k.Cfg.Debug {
			k.logger.Printf("writing expired after %s, Kafka output might not be initialized", k.Cfg.Timeout)
		}
		if k.Cfg.EnableMetrics {
			kafkaNumberOfFailSendMsgs.WithLabelValues(k.Cfg.Name, "timeout").Inc()
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
	if !k.Cfg.EnableMetrics {
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
	var producer sarama.SyncProducer
	var err error
	defer k.wg.Done()
	workerLogPrefix := fmt.Sprintf("worker-%d", idx)
	k.logger.Printf("%s starting", workerLogPrefix)
CRPROD:
	producer, err = sarama.NewSyncProducer(strings.Split(k.Cfg.Address, ","), config)
	if err != nil {
		k.logger.Printf("%s failed to create kafka producer: %v", workerLogPrefix, err)
		time.Sleep(k.Cfg.RecoveryWaitTime)
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
			pmsg, err = outputs.AddSubscriptionTarget(pmsg, m.GetMeta(), k.Cfg.AddTarget, k.targetTpl)
			if err != nil {
				k.logger.Printf("failed to add target to the response: %v", err)
			}
			bb, err := outputs.Marshal(pmsg, m.GetMeta(), k.mo, k.Cfg.SplitEvents, k.evps...)
			if err != nil {
				if k.Cfg.Debug {
					k.logger.Printf("%s failed marshaling proto msg: %v", workerLogPrefix, err)
				}
				if k.Cfg.EnableMetrics {
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
						if k.Cfg.Debug {
							log.Printf("failed to execute template: %v", err)
						}
						kafkaNumberOfFailSendMsgs.WithLabelValues(config.ClientID, "template_error").Inc()
						continue
					}
				}

				msg := &sarama.ProducerMessage{
					Topic: k.Cfg.Topic,
					Value: sarama.ByteEncoder(b),
				}
				if k.Cfg.InsertKey {
					msg.Key = sarama.ByteEncoder(k.partitionKey(m.GetMeta()))
				}
				var start time.Time
				if k.Cfg.EnableMetrics {
					start = time.Now()
				}
				_, _, err = producer.SendMessage(msg)
				if err != nil {
					if k.Cfg.Debug {
						k.logger.Printf("%s failed to send a kafka msg to topic '%s': %v", workerLogPrefix, k.Cfg.Topic, err)
					}
					if k.Cfg.EnableMetrics {
						kafkaNumberOfFailSendMsgs.WithLabelValues(config.ClientID, "send_error").Inc()
					}
					producer.Close()
					time.Sleep(k.Cfg.RecoveryWaitTime)
					goto CRPROD
				}
				if k.Cfg.EnableMetrics {
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
	sb.WriteString(k.Cfg.Name)
	sb.WriteString("-kafka-prod")
	k.Cfg.Name = sb.String()
}

func (k *kafkaOutput) SetClusterName(name string) {}

func (k *kafkaOutput) SetTargetsConfig(map[string]*types.TargetConfig) {}

func (k *kafkaOutput) createConfig() (*sarama.Config, error) {
	cfg := sarama.NewConfig()
	cfg.ClientID = k.Cfg.Name
	// SASL_PLAINTEXT or SASL_SSL
	if k.Cfg.SASL != nil {
		cfg.Net.SASL.Enable = true
		cfg.Net.SASL.User = k.Cfg.SASL.User
		cfg.Net.SASL.Password = k.Cfg.SASL.Password
		cfg.Net.SASL.Mechanism = sarama.SASLMechanism(k.Cfg.SASL.Mechanism)
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
			cfg.Net.SASL.TokenProvider = oauthbearer.NewTokenProvider(cfg.Net.SASL.User, cfg.Net.SASL.Password, k.Cfg.SASL.TokenURL)
		}
	}
	// SSL or SASL_SSL
	if k.Cfg.TLS != nil {
		var err error
		cfg.Net.TLS.Enable = true
		cfg.Net.TLS.Config, err = utils.NewTLSConfig(
			k.Cfg.TLS.CaFile,
			k.Cfg.TLS.CertFile,
			k.Cfg.TLS.KeyFile,
			"",
			k.Cfg.TLS.SkipVerify,
			false)
		if err != nil {
			return nil, err
		}
	}

	cfg.Producer.Retry.Max = k.Cfg.MaxRetry
	cfg.Producer.RequiredAcks = sarama.WaitForAll
	cfg.Producer.Return.Successes = true
	cfg.Producer.Timeout = k.Cfg.Timeout
	cfg.Metadata.Full = false

	switch k.Cfg.CompressionCodec {
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
