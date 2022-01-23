package kafka_output

import (
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
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
)

const (
	defaultKafkaMaxRetry    = 2
	defaultKafkaTimeout     = 5 * time.Second
	defaultKafkaTopic       = "telemetry"
	defaultNumWorkers       = 1
	defaultFormat           = "event"
	defaultRecoveryWaitTime = 10 * time.Second
	defaultAddress          = "localhost:9092"
	loggingPrefix           = "[kafka_output] "
)

type protoMsg struct {
	m    proto.Message
	meta outputs.Meta
}

func init() {
	outputs.Register("kafka", func() outputs.Output {
		return &KafkaOutput{
			Cfg:    &Config{},
			wg:     new(sync.WaitGroup),
			logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
		}
	})
}

// KafkaOutput //
type KafkaOutput struct {
	Cfg      *Config
	logger   sarama.StdLogger
	mo       *formatters.MarshalOptions
	cancelFn context.CancelFunc
	msgChan  chan *protoMsg
	wg       *sync.WaitGroup
	evps     []formatters.EventProcessor

	targetTpl *template.Template
	msgTpl    *template.Template
}

// Config //
type Config struct {
	Address            string        `mapstructure:"address,omitempty"`
	Topic              string        `mapstructure:"topic,omitempty"`
	Name               string        `mapstructure:"name,omitempty"`
	SASL               *sasl         `mapstructure:"sasl,omitempty"`
	TLS                *tlsConfig    `mapstructure:"tls,omitempty"`
	MaxRetry           int           `mapstructure:"max-retry,omitempty"`
	Timeout            time.Duration `mapstructure:"timeout,omitempty"`
	RecoveryWaitTime   time.Duration `mapstructure:"recovery-wait-time,omitempty"`
	Format             string        `mapstructure:"format,omitempty"`
	AddTarget          string        `mapstructure:"add-target,omitempty"`
	TargetTemplate     string        `mapstructure:"target-template,omitempty"`
	MsgTemplate        string        `mapstructure:"msg-template,omitempty"`
	NumWorkers         int           `mapstructure:"num-workers,omitempty"`
	Debug              bool          `mapstructure:"debug,omitempty"`
	BufferSize         int           `mapstructure:"buffer-size,omitempty"`
	OverrideTimestamps bool          `mapstructure:"override-timestamps,omitempty"`
	EnableMetrics      bool          `mapstructure:"enable-metrics,omitempty"`
	EventProcessors    []string      `mapstructure:"event-processors,omitempty"`
}
type sasl struct {
	User      string `mapstructure:"user,omitempty"`
	Password  string `mapstructure:"password,omitempty"`
	Mechanism string `mapstructure:"mechanism,omitempty"`
	TokenURL  string `mapstructure:"token-url,omitempty"`
}

type tlsConfig struct {
	CaFile     string `mapstructure:"ca-file,omitempty"`
	KeyFile    string `mapstructure:"key-file,omitempty"`
	CertFile   string `mapstructure:"cert-file,omitempty"`
	SkipVerify bool   `mapstructure:"skip-verify,omitempty"`
}

func (k *KafkaOutput) String() string {
	b, err := json.Marshal(k)
	if err != nil {
		return ""
	}
	return string(b)
}

func (k *KafkaOutput) SetLogger(logger *log.Logger) {
	if logger != nil {
		sarama.Logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
		k.logger = sarama.Logger
	}
}

func (k *KafkaOutput) SetEventProcessors(ps map[string]map[string]interface{},
	logger *log.Logger,
	tcs map[string]*types.TargetConfig,
	acts map[string]map[string]interface{}) {
	for _, epName := range k.Cfg.EventProcessors {
		if epCfg, ok := ps[epName]; ok {
			epType := ""
			for k := range epCfg {
				epType = k
				break
			}
			if in, ok := formatters.EventProcessors[epType]; ok {
				ep := in()
				err := ep.Init(epCfg[epType],
					formatters.WithLogger(logger),
					formatters.WithTargets(tcs),
					formatters.WithActions(acts),
				)
				if err != nil {
					k.logger.Printf("failed initializing event processor '%s' of type='%s': %v", epName, epType, err)
					continue
				}
				k.evps = append(k.evps, ep)
				k.logger.Printf("added event processor '%s' of type=%s to kafka output", epName, epType)
				continue
			}
			k.logger.Printf("%q event processor has an unknown type=%q", epName, epType)
			continue
		}
		k.logger.Printf("%q event processor not found!", epName)
	}
}

// Init /
func (k *KafkaOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, k.Cfg)
	if err != nil {
		return err
	}
	if k.Cfg.Name == "" {
		k.Cfg.Name = name
	}
	for _, opt := range opts {
		opt(k)
	}
	err = k.setDefaults()
	if err != nil {
		return err
	}
	k.msgChan = make(chan *protoMsg, uint(k.Cfg.BufferSize))
	k.mo = &formatters.MarshalOptions{
		Format:     k.Cfg.Format,
		OverrideTS: k.Cfg.OverrideTimestamps,
	}

	if k.Cfg.TargetTemplate == "" {
		k.targetTpl = outputs.DefaultTargetTemplate
	} else if k.Cfg.AddTarget != "" {
		k.targetTpl, err = utils.CreateTemplate("target-template", k.Cfg.TargetTemplate)
		if err != nil {
			return err
		}
		k.targetTpl = k.targetTpl.Funcs(outputs.TemplateFuncs)
	}

	if k.Cfg.MsgTemplate != "" {
		k.msgTpl, err = utils.CreateTemplate("msg-template", k.Cfg.MsgTemplate)
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

func (k *KafkaOutput) setDefaults() error {
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
func (k *KafkaOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}

	wctx, cancel := context.WithTimeout(ctx, k.Cfg.Timeout)
	defer cancel()

	select {
	case <-ctx.Done():
		return
	case k.msgChan <- &protoMsg{m: rsp, meta: meta}:
	case <-wctx.Done():
		if k.Cfg.Debug {
			k.logger.Printf("writing expired after %s, Kafka output might not be initialized", k.Cfg.Timeout)
		}
		if k.Cfg.EnableMetrics {
			KafkaNumberOfFailSendMsgs.WithLabelValues(k.Cfg.Name, "timeout").Inc()
		}
		return
	}
}

func (k *KafkaOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {}

// Close //
func (k *KafkaOutput) Close() error {
	k.cancelFn()
	k.wg.Wait()
	return nil
}

// Metrics //
func (k *KafkaOutput) RegisterMetrics(reg *prometheus.Registry) {
	if !k.Cfg.EnableMetrics {
		return
	}
	if err := registerMetrics(reg); err != nil {
		k.logger.Printf("failed to register metric: %v", err)
	}
}

func (k *KafkaOutput) worker(ctx context.Context, idx int, config *sarama.Config) {
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
			err = outputs.AddSubscriptionTarget(m.m, m.meta, k.Cfg.AddTarget, k.targetTpl)
			if err != nil {
				k.logger.Printf("failed to add target to the response: %v", err)
			}
			b, err := k.mo.Marshal(m.m, m.meta, k.evps...)
			if err != nil {
				if k.Cfg.Debug {
					k.logger.Printf("%s failed marshaling proto msg: %v", workerLogPrefix, err)
				}
				if k.Cfg.EnableMetrics {
					KafkaNumberOfFailSendMsgs.WithLabelValues(config.ClientID, "marshal_error").Inc()
				}
				continue
			}

			if k.msgTpl != nil && len(b) > 0 {
				b, err = outputs.ExecTemplate(b, k.msgTpl)
				if err != nil {
					if k.Cfg.Debug {
						log.Printf("failed to execute template: %v", err)
					}
					KafkaNumberOfFailSendMsgs.WithLabelValues(config.ClientID, "template_error").Inc()
					return
				}
			}

			msg := &sarama.ProducerMessage{
				Topic: k.Cfg.Topic,
				Value: sarama.ByteEncoder(b),
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
					KafkaNumberOfFailSendMsgs.WithLabelValues(config.ClientID, "send_error").Inc()
				}
				producer.Close()
				time.Sleep(k.Cfg.RecoveryWaitTime)
				goto CRPROD
			}
			if k.Cfg.EnableMetrics {
				KafkaSendDuration.WithLabelValues(config.ClientID).Set(float64(time.Since(start).Nanoseconds()))
				KafkaNumberOfSentMsgs.WithLabelValues(config.ClientID).Inc()
				KafkaNumberOfSentBytes.WithLabelValues(config.ClientID).Add(float64(len(b)))
			}
		}
	}
}

func (k *KafkaOutput) SetName(name string) {
	sb := strings.Builder{}
	if name != "" {
		sb.WriteString(name)
		sb.WriteString("-")
	}
	sb.WriteString(k.Cfg.Name)
	sb.WriteString("-kafka-prod")
	k.Cfg.Name = sb.String()
}

func (k *KafkaOutput) SetClusterName(name string) {}

func (k *KafkaOutput) SetTargetsConfig(map[string]*types.TargetConfig) {}

func (k *KafkaOutput) createConfig() (*sarama.Config, error) {
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

	return cfg, nil
}
