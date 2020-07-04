package kafka_output

import (
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	"github.com/google/uuid"
	"github.com/karimra/gnmic/outputs"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	defaultKafkaMaxRetry = 2
	defaultKafkaTimeout  = 5
	defaultKafkaTopic    = "telemetry"
)

func init() {
	outputs.Register("kafka", func() outputs.Output {
		return &KafkaOutput{
			Cfg: &Config{},
		}
	})
}

// KafkaOutput //
type KafkaOutput struct {
	Cfg      *Config
	producer sarama.SyncProducer
	metrics  []prometheus.Collector
	logger   sarama.StdLogger
}

// Config //
type Config struct {
	Address  string
	Topic    string
	Name     string
	MaxRetry int
	Timeout  int
}

func (k *KafkaOutput) String() string {
	b, err := json.Marshal(k)
	if err != nil {
		return ""
	}
	return string(b)
}

// Init /
func (k *KafkaOutput) Init(cfg map[string]interface{}, logger *log.Logger) error {
	err := mapstructure.Decode(cfg, k.Cfg)
	if err != nil {
		return err
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
	if logger != nil {
		sarama.Logger = log.New(logger.Writer(), "kafka_output ", logger.Flags())
	} else {
		sarama.Logger = log.New(os.Stderr, "kafka_output ", log.LstdFlags|log.Lmicroseconds)
	}
	k.logger = sarama.Logger
	config := sarama.NewConfig()
	if k.Cfg.Name != "" {
		config.ClientID = k.Cfg.Name
	} else {
		config.ClientID = "gnmic-" + uuid.New().String()
	}

	config.Producer.Retry.Max = k.Cfg.MaxRetry
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Successes = true
	config.Producer.Timeout = time.Duration(k.Cfg.Timeout) * time.Second
	k.producer, err = sarama.NewSyncProducer(strings.Split(k.Cfg.Address, ","), config)
	if err != nil {
		return err
	}
	k.logger.Printf("initialized kafka producer: %s", k.String())
	return err
}

// Write //
func (k *KafkaOutput) Write(b []byte, meta outputs.Meta) {
	if len(b) == 0 {
		return
	}
	msg := &sarama.ProducerMessage{
		Topic: k.Cfg.Topic,
		Value: sarama.ByteEncoder(b),
	}
	_, _, err := k.producer.SendMessage(msg)
	if err != nil {
		k.logger.Printf("failed to send a kafka msg to topic '%s': %v", k.Cfg.Topic, err)
	}
	// 	k.logger.Printf("wrote %d bytes to kafka_topic=%s", len(b), k.Cfg.Topic)
}

// Close //
func (k *KafkaOutput) Close() error {
	return k.producer.Close()
}

// Metrics //
func (k *KafkaOutput) Metrics() []prometheus.Collector { return k.metrics }
