package udp_output

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/outputs"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"
)

func init() {
	outputs.Register("udp", func() outputs.Output {
		return &UDPSock{
			Cfg: &Config{},
		}
	})
}

type UDPSock struct {
	Cfg    *Config
	logger *log.Logger
	mo     *collector.MarshalOptions
}

type Config struct {
	Address string // ip:port
	Format  string
}

func (u *UDPSock) Init(cfg map[string]interface{}, logger *log.Logger) error {
	err := mapstructure.Decode(cfg, u.Cfg)
	if err != nil {
		return err
	}
	_, _, err = net.SplitHostPort(u.Cfg.Address)
	if err != nil {
		return fmt.Errorf("wrong address format: %v", err)
	}
	u.logger = log.New(os.Stderr, "udp_output ", log.LstdFlags|log.Lmicroseconds)
	if logger != nil {
		u.logger.SetOutput(logger.Writer())
		u.logger.SetFlags(logger.Flags())
	}
	u.mo = &collector.MarshalOptions{Format: u.Cfg.Format}
	return nil
}
func (u *UDPSock) Write(m proto.Message, meta outputs.Meta) {
	if m == nil {
		return
	}
	udpAddr, err := net.ResolveUDPAddr("udp", u.Cfg.Address)
	if err != nil {
		u.logger.Printf("failed resolving UDP Address: %v", err)
		return
	}
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		u.logger.Printf("failed UDP dial: %v", err)
		return
	}
	b, err := u.mo.Marshal(m, meta)
	if err != nil {
		u.logger.Printf("failed marshaling proto msg: %v", err)
		return
	}
	_, err = conn.Write(b)
	if err != nil {
		u.logger.Printf("failed writing to udp socket: %v", err)
		return
	}
}
func (u *UDPSock) Close() error                    { return nil }
func (u *UDPSock) Metrics() []prometheus.Collector { return nil }
func (u *UDPSock) String() string {
	b, err := json.Marshal(u)
	if err != nil {
		return ""
	}
	return string(b)
}
