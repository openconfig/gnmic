package kafka_output

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/google/uuid"
	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/outputs"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
)

const (
	defaultKafkaMaxRetry    = 2
	defaultKafkaTimeout     = 5 * time.Second
	defaultKafkaTopic       = "telemetry"
	numWorkers              = 1
	defaultFormat           = "json"
	defaultRecoveryWaitTime = 10 * time.Second
)

type protoMsg struct {
	m    proto.Message
	meta outputs.Meta
}

func init() {
	outputs.Register("kafka", func() outputs.Output {
		return &KafkaOutput{
			Cfg:     &Config{},
			msgChan: make(chan *protoMsg),
			wg:      new(sync.WaitGroup),
		}
	})
}

// KafkaOutput //
type KafkaOutput struct {
	Cfg      *Config
	metrics  []prometheus.Collector
	logger   sarama.StdLogger
	mo       *collector.MarshalOptions
	cancelFn context.CancelFunc
	msgChan  chan *protoMsg
	wg       *sync.WaitGroup
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
}

func (k *KafkaOutput) String() string {
	b, err := json.Marshal(k)
	if err != nil {
		return ""
	}
	return string(b)
}

// Init /
func (k *KafkaOutput) Init(ctx context.Context, cfg map[string]interface{}, logger *log.Logger) error {
	err := outputs.DecodeConfig(cfg, k.Cfg)
	if err != nil {
		logger.Printf("kafka output config decode failed: %v", err)
		return err
	}
	if k.Cfg.Format == "" {
		k.Cfg.Format = defaultFormat
	}
	if !(k.Cfg.Format == "event" || k.Cfg.Format == "protojson" || k.Cfg.Format == "proto" || k.Cfg.Format == "json") {
		return fmt.Errorf("unsupported output format '%s' for output type kafka", k.Cfg.Format)
	}
	if k.Cfg.Topic == "" {
		k.Cfg.Topic = defaultKafkaTopic
	}
	if k.Cfg.MaxRetry == 0 {
		k.Cfg.MaxRetry = defaultKafkaMaxRetry
	}
	if k.Cfg.Timeout == 0 {
		k.Cfg.Timeout = defaultKafkaTimeout
	}
	if k.Cfg.RecoveryWaitTime == 0 {
		k.Cfg.RecoveryWaitTime = defaultRecoveryWaitTime
	}
	if logger != nil {
		sarama.Logger = log.New(logger.Writer(), "kafka_output ", logger.Flags())
	} else {
		sarama.Logger = log.New(os.Stderr, "kafka_output ", log.LstdFlags|log.Lmicroseconds)
	}
	k.logger = sarama.Logger
	k.mo = &collector.MarshalOptions{Format: k.Cfg.Format}

	config := sarama.NewConfig()
	if k.Cfg.Name != "" {
		config.ClientID = k.Cfg.Name
	} else {
		config.ClientID = "gnmic-" + uuid.New().String()
	}

	config.Producer.Retry.Max = k.Cfg.MaxRetry
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Successes = true
	config.Producer.Timeout = k.Cfg.Timeout

	ctx, k.cancelFn = context.WithCancel(ctx)
	k.wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go k.worker(ctx, i, config)
	}
	go func() {
		<-ctx.Done()
		k.Close()
	}()
	return nil
}

// Write //
func (k *KafkaOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	case k.msgChan <- &protoMsg{m: rsp, meta: meta}:
	case <-time.After(k.Cfg.Timeout):
		log.Printf("writing expired after %s, Kafka output might not be initialized", k.Cfg.Timeout)
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
func (k *KafkaOutput) Metrics() []prometheus.Collector { return k.metrics }

func (k *KafkaOutput) worker(ctx context.Context, idx int, config *sarama.Config) {
	var producer sarama.SyncProducer
	var err error
	defer k.wg.Done()
	k.logger.Printf("starting worker id=%d", idx)
CRPROD:
	producer, err = sarama.NewSyncProducer(strings.Split(k.Cfg.Address, ","), config)
	if err != nil {
		sarama.Logger.Printf("worker-%d failed to create kafka producer: %v", idx, err)
		time.Sleep(k.Cfg.RecoveryWaitTime)
		goto CRPROD
	}
	defer producer.Close()
	k.logger.Printf("worker-%d initialized kafka producer: %s", idx, k.String())
	for {
		select {
		case <-ctx.Done():
			k.logger.Printf("worker-%d shutting down", idx)
			return
		case m := <-k.msgChan:
			b, err := k.mo.Marshal(m.m, m.meta)
			if err != nil {
				k.logger.Printf("worker-%d failed marshaling proto msg: %v", idx, err)
				continue
			}
			msg := &sarama.ProducerMessage{
				Topic: k.Cfg.Topic,
				Value: sarama.ByteEncoder(b),
			}
			_, _, err = producer.SendMessage(msg)
			if err != nil {
				k.logger.Printf("worker-%d failed to send a kafka msg to topic '%s': %v", idx, k.Cfg.Topic, err)
				producer.Close()
				time.Sleep(k.Cfg.RecoveryWaitTime)
				goto CRPROD
			}
		}
	}
}
