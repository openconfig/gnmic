package tcp_output

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/outputs"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
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

	conn     *net.TCPConn
	cancelFn context.CancelFunc
	buffer   chan []byte
	limiter  *time.Ticker
	logger   *log.Logger
	mo       *collector.MarshalOptions
}

type Config struct {
	Address    string        `mapstructure:"address,omitempty"` // ip:port
	Rate       time.Duration `mapstructure:"rate,omitempty"`
	BufferSize uint          `mapstructure:"buffer-size,omitempty"`
	Format     string        `mapstructure:"format,omitempty"`
	KeepAlive  time.Duration `mapstructure:"keep-alive,omitempty"`
}

func (t *TCPOutput) Init(cfg map[string]interface{}, logger *log.Logger) error {
	decoder, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{
			DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
			Result:     t.Cfg,
		},
	)
	if err != nil {
		return err
	}
	err = decoder.Decode(cfg)
	if err != nil {
		return err
	}
	_, _, err = net.SplitHostPort(t.Cfg.Address)
	if err != nil {
		return fmt.Errorf("wrong address format: %v", err)
	}
	t.logger = log.New(os.Stderr, "tcp_output ", log.LstdFlags|log.Lmicroseconds)
	if logger != nil {
		t.logger.SetOutput(logger.Writer())
		t.logger.SetFlags(logger.Flags())
	}
	t.buffer = make(chan []byte, t.Cfg.BufferSize)
	if t.Cfg.Rate > 0 {
		t.limiter = time.NewTicker(t.Cfg.Rate)
	}
	var ctx context.Context
	ctx, t.cancelFn = context.WithCancel(context.Background())
	t.mo = &collector.MarshalOptions{Format: t.Cfg.Format}
	tcpAddr, err := net.ResolveTCPAddr("tcp", t.Cfg.Address)
	if err != nil {
		return err
	}
	t.conn, err = net.DialTCP("tcp", nil, tcpAddr)
	if err != nil {
		return err
	}
	if t.Cfg.KeepAlive > 0 {
		t.conn.SetKeepAlive(true)
		t.conn.SetKeepAlivePeriod(t.Cfg.KeepAlive)
	}

	go t.start(ctx)
	return nil
}
func (t *TCPOutput) Write(m proto.Message, meta outputs.Meta) {
	if m == nil {
		return
	}
	b, err := t.mo.Marshal(m, meta)
	if err != nil {
		t.logger.Printf("failed marshaling proto msg: %v", err)
		return
	}
	t.buffer <- b
}
func (t *TCPOutput) Close() error {
	t.cancelFn()
	t.limiter.Stop()
	close(t.buffer)
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
func (t *TCPOutput) start(ctx context.Context) {
	var err error
	defer t.Close()
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-t.buffer:
			if t.limiter != nil {
				<-t.limiter.C
			}
			_, err = t.conn.Write(b)
			if err != nil {
				t.logger.Printf("failed sending tcp bytes: %v", err)
			}
		}
	}
}
