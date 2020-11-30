package udp_output

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
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
)

const defaultRetryTimer = 2 * time.Second

func init() {
	outputs.Register("udp", func() outputs.Output {
		return &UDPSock{
			Cfg: &Config{},
		}
	})
}

type UDPSock struct {
	Cfg *Config

	conn     *net.UDPConn
	cancelFn context.CancelFunc
	buffer   chan []byte
	limiter  *time.Ticker
	logger   *log.Logger
	mo       *collector.MarshalOptions
}

type Config struct {
	Address       string        `mapstructure:"address,omitempty"` // ip:port
	Rate          time.Duration `mapstructure:"rate,omitempty"`
	BufferSize    uint          `mapstructure:"buffer-size,omitempty"`
	Format        string        `mapstructure:"format,omitempty"`
	RetryInterval time.Duration `mapstructure:"retry-interval,omitempty"`
}

func (u *UDPSock) SetLogger(logger *log.Logger) {
	if logger != nil {
		u.logger = log.New(logger.Writer(), "udp_output ", logger.Flags())
	} else {
		u.logger = log.New(os.Stderr, "udp_output ", log.LstdFlags|log.Lmicroseconds)
	}
}

func (u *UDPSock) Init(ctx context.Context, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, u.Cfg)
	if err != nil {
		u.logger.Printf("udp output config decode failed: %v", err)
		return err
	}
	_, _, err = net.SplitHostPort(u.Cfg.Address)
	if err != nil {
		u.logger.Printf("udp output config validation failed: %v", err)
		return fmt.Errorf("wrong address format: %v", err)
	}
	if u.Cfg.RetryInterval == 0 {
		u.Cfg.RetryInterval = defaultRetryTimer
	}

	u.buffer = make(chan []byte, u.Cfg.BufferSize)
	if u.Cfg.Rate > 0 {
		u.limiter = time.NewTicker(u.Cfg.Rate)
	}
	go func() {
		<-ctx.Done()
		u.Close()
	}()
	ctx, u.cancelFn = context.WithCancel(ctx)
	u.mo = &collector.MarshalOptions{Format: u.Cfg.Format}
	go u.start(ctx)
	return nil
}

func (u *UDPSock) Write(ctx context.Context, m proto.Message, meta outputs.Meta) {
	if m == nil {
		return
	}
	b, err := u.mo.Marshal(m, meta)
	if err != nil {
		u.logger.Printf("failed marshaling proto msg: %v", err)
		return
	}
	u.buffer <- b
}

func (u *UDPSock) Close() error {
	u.cancelFn()
	if u.limiter != nil {
		u.limiter.Stop()
	}
	return nil
}
func (u *UDPSock) Metrics() []prometheus.Collector { return nil }

func (u *UDPSock) String() string {
	b, err := json.Marshal(u)
	if err != nil {
		return ""
	}
	return string(b)
}

func (u *UDPSock) start(ctx context.Context) {
	var udpAddr *net.UDPAddr
	var err error
	defer u.Close()
DIAL:
	if ctx.Err() != nil {
		u.logger.Printf("context error: %v", ctx.Err())
		return
	}
	udpAddr, err = net.ResolveUDPAddr("udp", u.Cfg.Address)
	if err != nil {
		u.logger.Printf("failed to dial udp: %v", err)
		time.Sleep(u.Cfg.RetryInterval)
		goto DIAL
	}
	u.conn, err = net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		u.logger.Printf("failed to dial udp: %v", err)
		time.Sleep(u.Cfg.RetryInterval)
		goto DIAL
	}
	for {
		select {
		case <-ctx.Done():
			return
		case b := <-u.buffer:
			if u.limiter != nil {
				<-u.limiter.C
			}
			_, err = u.conn.Write(b)
			if err != nil {
				u.logger.Printf("failed sending udp bytes: %v", err)
				time.Sleep(u.Cfg.RetryInterval)
				goto DIAL
			}
		}
	}
}
