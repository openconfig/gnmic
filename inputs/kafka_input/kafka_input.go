package kafka_input

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/Shopify/sarama"
	"github.com/damiannolan/sasl/oauthbearer"
	"github.com/google/uuid"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/inputs"
	"github.com/karimra/gnmic/outputs"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
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

func init() {
	inputs.Register("kafka", func() inputs.Input {
		return &KafkaInput{
			Cfg:    &Config{},
			logger: log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
			wg:     new(sync.WaitGroup),
		}
	})
}

// KafkaInput //
type KafkaInput struct {
	Cfg     *Config
	cfn     context.CancelFunc
	logger  sarama.StdLogger
	wg      *sync.WaitGroup
	outputs []outputs.Output
	evps    []formatters.EventProcessor
}

// Config //
type Config struct {
	Name              string        `mapstructure:"name,omitempty"`
	Address           string        `mapstructure:"address,omitempty"`
	Topics            string        `mapstructure:"topics,omitempty"`
	SASL              *sasl         `mapstructure:"sasl,omitempty"`
	GroupID           string        `mapstructure:"group-id,omitempty"`
	SessionTimeout    time.Duration `mapstructure:"session-timeout,omitempty"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeat-interval,omitempty"`
	RecoveryWaitTime  time.Duration `mapstructure:"recovery-wait-time,omitempty"`
	Version           string        `mapstructure:"version,omitempty"`
	Format            string        `mapstructure:"format,omitempty"`
	Debug             bool          `mapstructure:"debug,omitempty"`
	NumWorkers        int           `mapstructure:"num-workers,omitempty"`
	Outputs           []string      `mapstructure:"outputs,omitempty"`
	EventProcessors   []string      `mapstructure:"event-processors,omitempty"`

	kafkaVersion sarama.KafkaVersion
}

type sasl struct {
	User      string `mapstructure:"user,omitempty"`
	Password  string `mapstructure:"password,omitempty"`
	Mechanism string `mapstructure:"mechanism,omitempty"`
	TokenURL  string `mapstructure:"token-url,omitempty"`
}

func (k *KafkaInput) Start(ctx context.Context, name string, cfg map[string]interface{}, opts ...inputs.Option) error {
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
	k.wg.Add(k.Cfg.NumWorkers)
	for i := 0; i < k.Cfg.NumWorkers; i++ {
		go k.worker(ctx, i)
	}
	return nil
}

func (k *KafkaInput) worker(ctx context.Context, idx int) {
	defer k.wg.Done()

	config := k.createConfig(idx)
	workerLogPrefix := fmt.Sprintf("worker-%d", idx)
START:
	k.logger.Printf("%s starting consumer group %s", workerLogPrefix, k.Cfg.GroupID)
	consumerGrp, err := sarama.NewConsumerGroup(strings.Split(k.Cfg.Address, ","), k.Cfg.GroupID, config)
	if err != nil {
		k.logger.Printf("%s failed to create consumer group: %v", workerLogPrefix, err)
		time.Sleep(k.Cfg.RecoveryWaitTime)
		goto START
	}
	k.logger.Printf("%s started consumer group %s", workerLogPrefix, k.Cfg.GroupID)
	defer consumerGrp.Close()
	cons := &consumer{
		ready:   make(chan bool),
		msgChan: make(chan *sarama.ConsumerMessage),
	}
	go func() {
		var err error
		for {
			if ctx.Err() != nil {
				return
			}
			err = consumerGrp.Consume(ctx, strings.Split(k.Cfg.Topics, ","), cons)
			if err != nil {
				if k.Cfg.Debug {
					k.logger.Printf("%s failed to start consumer, topics=%q, group=%q : %v", workerLogPrefix, k.Cfg.Topics, k.Cfg.GroupID, err)
				}
				continue
			}
			cons.ready = make(chan bool)
		}
	}()
	<-cons.ready
	k.logger.Printf("%s kafka consumer ready", workerLogPrefix)
	for {
		select {
		case <-ctx.Done():
			return
		case m := <-cons.msgChan:
			if len(m.Value) == 0 {
				continue
			}
			if k.Cfg.Debug {
				k.logger.Printf("%s client=%s received msg, topic=%s, partition=%d, key=%q, length=%d, value=%s", workerLogPrefix, config.ClientID, m.Topic, m.Partition, string(m.Key), len(m.Value), string(m.Value))
			}
			switch k.Cfg.Format {
			case "event":
				evMsgs := make([]*formatters.EventMsg, 1)
				err = json.Unmarshal(m.Value, &evMsgs)
				if err != nil {
					if k.Cfg.Debug {
						k.logger.Printf("%s failed to unmarshal event msg: %v", workerLogPrefix, err)
					}
					continue
				}

				for _, p := range k.evps {
					evMsgs = p.Apply(evMsgs...)
				}

				go func() {
					for _, o := range k.outputs {
						for _, ev := range evMsgs {
							o.WriteEvent(ctx, ev)
						}
					}
				}()
			case "proto":
				var protoMsg proto.Message
				err = proto.Unmarshal(m.Value, protoMsg)
				if err != nil {
					if k.Cfg.Debug {
						k.logger.Printf("%s failed to unmarshal proto msg: %v", workerLogPrefix, err)
					}
					continue
				}
				meta := outputs.Meta{}
				go func() {
					for _, o := range k.outputs {
						o.Write(ctx, protoMsg, meta)
					}
				}()
			}
		case err := <-consumerGrp.Errors():
			k.logger.Printf("%s client=%s, consumer-group=%s error: %v", workerLogPrefix, config.ClientID, k.Cfg.GroupID, err)
			time.Sleep(k.Cfg.RecoveryWaitTime)
			goto START
		}
	}
}

func (k *KafkaInput) Close() error {
	k.cfn()
	k.wg.Wait()
	return nil
}

func (k *KafkaInput) SetLogger(logger *log.Logger) {
	if logger != nil {
		sarama.Logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
		k.logger = sarama.Logger
	}
}

func (k *KafkaInput) SetOutputs(outs map[string]outputs.Output) {
	if len(k.Cfg.Outputs) == 0 {
		for _, o := range outs {
			k.outputs = append(k.outputs, o)
		}
		return
	}
	for _, name := range k.Cfg.Outputs {
		if o, ok := outs[name]; ok {
			k.outputs = append(k.outputs, o)
		}
	}
}

func (k *KafkaInput) SetName(name string) {
	sb := strings.Builder{}
	if name != "" {
		sb.WriteString(name)
		sb.WriteString("-")
	}
	sb.WriteString(k.Cfg.Name)
	sb.WriteString("-kafka-cons")
	k.Cfg.Name = sb.String()
}

func (k *KafkaInput) SetEventProcessors(ps map[string]map[string]interface{}, logger *log.Logger, tcs map[string]*types.TargetConfig) {
	for _, epName := range k.Cfg.EventProcessors {
		if epCfg, ok := ps[epName]; ok {
			epType := ""
			for k := range epCfg {
				epType = k
				break
			}
			if in, ok := formatters.EventProcessors[epType]; ok {
				ep := in()
				err := ep.Init(epCfg[epType], formatters.WithLogger(logger), formatters.WithTargets(tcs))
				if err != nil {
					k.logger.Printf("failed initializing event processor %q of type=%q: %v", epName, epType, err)
					continue
				}
				k.evps = append(k.evps, ep)
				k.logger.Printf("added event processor %q of type=%q to kafka input", epName, epType)
			}
		}
	}
}

// helper funcs

func (k *KafkaInput) setDefaults() error {
	var err error
	if k.Cfg.Version != "" {
		k.Cfg.kafkaVersion, err = sarama.ParseKafkaVersion(k.Cfg.Version)
		if err != nil {
			return err
		}
	} else {
		k.Cfg.kafkaVersion = defaultVersion

	}
	if k.Cfg.Format == "" {
		k.Cfg.Format = defaultFormat
	}
	if !(strings.ToLower(k.Cfg.Format) == "event" || strings.ToLower(k.Cfg.Format) == "proto") {
		return fmt.Errorf("unsupported input format")
	}
	if k.Cfg.Topics == "" {
		k.Cfg.Topics = defaultTopic
	}
	if k.Cfg.Address == "" {
		k.Cfg.Address = defaultAddress
	}
	if k.Cfg.NumWorkers <= 0 {
		k.Cfg.NumWorkers = defaultNumWorkers
	}
	if k.Cfg.SessionTimeout <= 2*time.Millisecond {
		k.Cfg.SessionTimeout = defaultSessionTimeout
	}
	if k.Cfg.HeartbeatInterval <= 1*time.Millisecond {
		k.Cfg.HeartbeatInterval = defaultHeartbeatInterval
	}
	if k.Cfg.GroupID == "" {
		k.Cfg.GroupID = defaultGroupID
	}
	if k.Cfg.RecoveryWaitTime <= 0 {
		k.Cfg.RecoveryWaitTime = defaultRecoveryWaitTime
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

func (k *KafkaInput) createConfig(idx int) *sarama.Config {
	cfg := sarama.NewConfig()
	cfg.Version = k.Cfg.kafkaVersion
	cfg.ClientID = fmt.Sprintf("%s-%d", k.Cfg.Name, idx)
	cfg.Consumer.Return.Errors = true
	cfg.Consumer.Group.Session.Timeout = k.Cfg.SessionTimeout
	cfg.Consumer.Group.Heartbeat.Interval = k.Cfg.HeartbeatInterval
	cfg.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRange
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
	return cfg
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
