package kafka_output

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/google/uuid"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
)

const (
	defaultKafkaMaxRetry    = 2
	defaultKafkaTimeout     = 5 * time.Second
	defaultKafkaTopic       = "telemetry"
	defaultNumWorkers       = 1
	defaultFormat           = "json"
	defaultRecoveryWaitTime = 10 * time.Second
	loggingPrefix           = "kafka_output "
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
			logger: log.New(ioutil.Discard, loggingPrefix, log.LstdFlags|log.Lmicroseconds),
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
}

// Config //
type Config struct {
	Address          string        `mapstructure:"address,omitempty"`
	Topic            string        `mapstructure:"topic,omitempty"`
	Name             string        `mapstructure:"name,omitempty"`
	MaxRetry         int           `mapstructure:"max-retry,omitempty"`
	Timeout          time.Duration `mapstructure:"timeout,omitempty"`
	RecoveryWaitTime time.Duration `mapstructure:"recovery-wait-time,omitempty"`
	Format           string        `mapstructure:"format,omitempty"`
	NumWorkers       int           `mapstructure:"num-workers,omitempty"`
	Debug            bool          `mapstructure:"debug,omitempty"`
	BufferSize       int           `mapstructure:"buffer-size,omitempty"`
	EnableMetrics    bool          `mapstructure:"enable-metrics,omitempty"`
	EventProcessors  []string      `mapstructure:"event-processors,omitempty"`
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

func (k *KafkaOutput) SetEventProcessors(ps map[string]map[string]interface{}, log *log.Logger) {
	for _, epName := range k.Cfg.EventProcessors {
		if epCfg, ok := ps[epName]; ok {
			epType := ""
			for k := range epCfg {
				epType = k
				break
			}
			if in, ok := formatters.EventProcessors[epType]; ok {
				ep := in()
				err := ep.Init(epCfg[epType], log)
				if err != nil {
					k.logger.Printf("failed initializing event processor '%s' of type='%s': %v", epName, epType, err)
					continue
				}
				k.evps = append(k.evps, ep)
				k.logger.Printf("added event processor '%s' of type=%s to kafka output", epName, epType)
			}
		}
	}
}

// Init /
func (k *KafkaOutput) Init(ctx context.Context, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, k.Cfg)
	if err != nil {
		return err
	}
	err = k.setDefaults()
	if err != nil {
		return err
	}
	for _, opt := range opts {
		opt(k)
	}
	k.msgChan = make(chan *protoMsg, uint(k.Cfg.BufferSize))
	k.mo = &formatters.MarshalOptions{Format: k.Cfg.Format}

	config := sarama.NewConfig()
	config.ClientID = k.Cfg.Name
	config.Producer.Retry.Max = k.Cfg.MaxRetry
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Successes = true
	config.Producer.Timeout = k.Cfg.Timeout

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
