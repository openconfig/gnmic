package kafka_output

import (
	"log"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	"github.com/karimra/gnmiClient/outputs"
	"github.com/mitchellh/mapstructure"
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
	stopChan chan struct{}
}

// Config //
type Config struct {
	Address  string
	Topic    string
	MaxRetry int
	Timeout  int
}

// Initialize /
func (k *KafkaOutput) Initialize(cfg map[string]interface{}) error {
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
	config := sarama.NewConfig()
	config.Producer.Retry.Max = k.Cfg.MaxRetry
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Return.Successes = true
	config.Producer.Timeout = time.Duration(k.Cfg.Timeout) * time.Second

	k.producer, err = sarama.NewSyncProducer(strings.Split(k.Cfg.Address, ","), config)
	return err
}

// Write //
func (k *KafkaOutput) Write(b []byte) {
	if len(b) == 0 {
		return
	}
	msg := &sarama.ProducerMessage{
		Topic: k.Cfg.Topic,
		Value: sarama.ByteEncoder(b),
	}
	_, _, err := k.producer.SendMessage(msg)
	if err != nil {
		log.Printf("failed to send a kafka msg to topic '%s': %v", k.Cfg.Topic, err)
	}
}

// Close //
func (k *KafkaOutput) Close() error {
	close(k.stopChan)
	return k.producer.Close()
}
