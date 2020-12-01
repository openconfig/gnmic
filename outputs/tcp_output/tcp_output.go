package tcp_output

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/outputs"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
)

const (
	defaultRetryTimer = 2 * time.Second
	defaultNumWorkers = 1
)

func init() {
	outputs.Register("tcp", func() outputs.Output {
		return &TCPOutput{
			Cfg: &Config{},
		}
	})
}

type TCPOutput struct {
	Cfg *Config

	cancelFn context.CancelFunc
	buffer   chan []byte
	limiter  *time.Ticker
	logger   *log.Logger
	mo       *formatters.MarshalOptions
}

type Config struct {
	Address       string        `mapstructure:"address,omitempty"` // ip:port
	Rate          time.Duration `mapstructure:"rate,omitempty"`
	BufferSize    uint          `mapstructure:"buffer-size,omitempty"`
	Format        string        `mapstructure:"format,omitempty"`
	KeepAlive     time.Duration `mapstructure:"keep-alive,omitempty"`
	RetryInterval time.Duration `mapstructure:"retry-interval,omitempty"`
	NumWorkers    int           `mapstructure:"num-workers,omitempty"`
}

func (t *TCPOutput) SetLogger(logger *log.Logger) {
	if logger != nil {
		t.logger = log.New(logger.Writer(), "tcp_output ", logger.Flags())
		return
	}
	t.logger = log.New(os.Stderr, "tcp_output ", log.LstdFlags|log.Lmicroseconds)
}

func (t *TCPOutput) SetEventProcessors(ps map[string]map[string]interface{}) {

}

func (t *TCPOutput) Init(ctx context.Context, cfg map[string]interface{}, opts ...outputs.Option) error {
	for _, opt := range opts {
		opt(t)
	}
	err := outputs.DecodeConfig(cfg, t.Cfg)
	if err != nil {
		t.logger.Printf("tcp output config decode failed: %v", err)
		return err
	}
	_, _, err = net.SplitHostPort(t.Cfg.Address)
	if err != nil {
		t.logger.Printf("tcp output config validation failed: %v", err)
		return fmt.Errorf("wrong address format: %v", err)
	}
	t.buffer = make(chan []byte, t.Cfg.BufferSize)
	if t.Cfg.Rate > 0 {
		t.limiter = time.NewTicker(t.Cfg.Rate)
	}
	if t.Cfg.RetryInterval == 0 {
		t.Cfg.RetryInterval = defaultRetryTimer
	}
	if t.Cfg.NumWorkers < 1 {
		t.Cfg.NumWorkers = defaultNumWorkers
	}
	t.mo = &formatters.MarshalOptions{Format: t.Cfg.Format}
	go func() {
		<-ctx.Done()
		t.Close()
	}()

	ctx, t.cancelFn = context.WithCancel(ctx)
	for i := 0; i < t.Cfg.NumWorkers; i++ {
		go t.start(ctx, i)
	}
	return nil
}
func (t *TCPOutput) Write(ctx context.Context, m proto.Message, meta outputs.Meta) {
	if m == nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
		b, err := t.mo.Marshal(m, meta)
		if err != nil {
			t.logger.Printf("failed marshaling proto msg: %v", err)
			return
		}
		t.buffer <- b
	}
}
func (t *TCPOutput) Close() error {
	t.cancelFn()
	if t.limiter != nil {
		t.limiter.Stop()
	}
	return nil
}
func (t *TCPOutput) Metrics() []prometheus.Collector { return nil }
func (t *TCPOutput) String() string {
	b, err := json.Marshal(t)
	if err != nil {
		return ""
	}
	return string(b)
}
func (t *TCPOutput) start(ctx context.Context, idx int) {
	workerLogPrefix := fmt.Sprintf("worker-%d", idx)
START:
	tcpAddr, err := net.ResolveTCPAddr("tcp", t.Cfg.Address)
	if err != nil {
		t.logger.Printf("%s failed to resolve address: %v", workerLogPrefix, err)
		time.Sleep(t.Cfg.RetryInterval)
		goto START
	}
	conn, err := net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		t.logger.Printf("%s failed to dial TCP: %v", workerLogPrefix, err)
		time.Sleep(t.Cfg.RetryInterval)
		goto START
	}
	defer conn.Close()
	if t.Cfg.KeepAlive > 0 {
		conn.SetKeepAlive(true)
		conn.SetKeepAlivePeriod(t.Cfg.KeepAlive)
	}
	defer t.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-t.buffer:
			if t.limiter != nil {
				<-t.limiter.C
			}
			_, err = conn.Write(b)
			if err != nil {
				t.logger.Printf("%s failed sending tcp bytes: %v", workerLogPrefix, err)
				conn.Close()
				time.Sleep(t.Cfg.RetryInterval)
				goto START
			}
		}
	}
}
