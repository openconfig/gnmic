package prometheus_output

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
)

const (
	defaultServiceRegistrationAddress = "localhost:8500"
	defaultRegistrationCheckInterval  = 5 * time.Second
	defaultMaxServiceFail             = 3
)

type ServiceRegistration struct {
	Address    string `mapstructure:"address,omitempty"`
	Datacenter string `mapstructure:"datacenter,omitempty"`
	Username   string `mapstructure:"username,omitempty"`
	Password   string `mapstructure:"password,omitempty"`
	Token      string `mapstructure:"token,omitempty"`

	Name             string        `mapstructure:"name,omitempty"`
	CheckInterval    time.Duration `mapstructure:"check-interval,omitempty"`
	MaxFail          int           `mapstructure:"max-fail,omitempty"`
	Tags             []string      `mapstructure:"tags,omitempty"`
	EnableHTTPCheck  bool          `mapstructure:"enable-http-check,omitempty"`
	HTTPCheckAddress string        `mapstructure:"http-check-address,omitempty"`

	deregisterAfter  string
	id               string
	httpCheckAddress string
}

func (p *PrometheusOutput) registerService(ctx context.Context) {
	if p.Cfg.ServiceRegistration == nil {
		return
	}
	var err error
	clientConfig := &api.Config{
		Address:    p.Cfg.ServiceRegistration.Address,
		Scheme:     "http",
		Datacenter: p.Cfg.ServiceRegistration.Datacenter,
		Token:      p.Cfg.ServiceRegistration.Token,
	}
	if p.Cfg.ServiceRegistration.Username != "" && p.Cfg.ServiceRegistration.Password != "" {
		clientConfig.HttpAuth = &api.HttpBasicAuth{
			Username: p.Cfg.ServiceRegistration.Username,
			Password: p.Cfg.ServiceRegistration.Password,
		}
	}
INITCONSUL:
	p.consulClient, err = api.NewClient(clientConfig)
	if err != nil {
		p.logger.Printf("failed to connect to consul: %v", err)
		time.Sleep(1 * time.Second)
		goto INITCONSUL
	}
	self, err := p.consulClient.Agent().Self()
	if err != nil {
		p.logger.Printf("failed to connect to consul: %v", err)
		time.Sleep(1 * time.Second)
		goto INITCONSUL
	}
	if cfg, ok := self["Config"]; ok {
		b, _ := json.Marshal(cfg)
		p.logger.Printf("consul agent config: %s", string(b))
	}

	service := &api.AgentServiceRegistration{
		ID:      p.Cfg.ServiceRegistration.id,
		Name:    p.Cfg.ServiceRegistration.Name,
		Address: p.Cfg.address,
		Port:    p.Cfg.port,
		Tags:    p.Cfg.ServiceRegistration.Tags,
		Checks: api.AgentServiceChecks{
			{
				TTL:                            p.Cfg.ServiceRegistration.CheckInterval.String(),
				DeregisterCriticalServiceAfter: p.Cfg.ServiceRegistration.deregisterAfter,
			},
		},
	}
	ttlCheckID := "service:" + p.Cfg.ServiceRegistration.id
	if p.Cfg.ServiceRegistration.EnableHTTPCheck {
		service.Checks = append(service.Checks, &api.AgentServiceCheck{
			HTTP:                           p.Cfg.ServiceRegistration.httpCheckAddress,
			Method:                         "GET",
			Interval:                       p.Cfg.ServiceRegistration.CheckInterval.String(),
			TLSSkipVerify:                  true,
			DeregisterCriticalServiceAfter: p.Cfg.ServiceRegistration.deregisterAfter,
		})
		ttlCheckID = ttlCheckID + ":1"
	}
	b, _ := json.Marshal(service)
	p.logger.Printf("registering service: %s", string(b))
	err = p.consulClient.Agent().ServiceRegister(service)
	if err != nil {
		p.logger.Printf("failed to register service in consul: %v", err)
		time.Sleep(time.Second)
		goto INITCONSUL
	}
	go func() {
		err = p.consulClient.Agent().UpdateTTL(ttlCheckID, "", api.HealthPassing)
		if err != nil {
			p.logger.Printf("failed to pass TTL check: %v", err)
		}
		ticker := time.NewTicker(p.Cfg.ServiceRegistration.CheckInterval / 2)
		for {
			select {
			case <-ticker.C:
				err = p.consulClient.Agent().UpdateTTL(ttlCheckID, "", api.HealthPassing)
				if err != nil {
					p.logger.Printf("failed to pass TTL check: %v", err)
				}
			case <-ctx.Done():
				p.consulClient.Agent().UpdateTTL(ttlCheckID, ctx.Err().Error(), api.HealthCritical)
				ticker.Stop()
				return
			}
		}
	}()
}

func (p *PrometheusOutput) setServiceRegistrationDefaults() {
	if p.Cfg.ServiceRegistration == nil {
		return
	}
	if p.Cfg.ServiceRegistration.Address == "" {
		p.Cfg.ServiceRegistration.Address = defaultServiceRegistrationAddress
	}
	if p.Cfg.ServiceRegistration.CheckInterval <= 0 {
		p.Cfg.ServiceRegistration.CheckInterval = defaultRegistrationCheckInterval
	}
	if p.Cfg.ServiceRegistration.MaxFail <= 0 {
		p.Cfg.ServiceRegistration.MaxFail = defaultMaxServiceFail
	}
	deregisterTimer := p.Cfg.ServiceRegistration.CheckInterval * time.Duration(p.Cfg.ServiceRegistration.MaxFail)
	p.Cfg.ServiceRegistration.deregisterAfter = deregisterTimer.String()

	if !p.Cfg.ServiceRegistration.EnableHTTPCheck {
		return
	}
	p.Cfg.ServiceRegistration.httpCheckAddress = p.Cfg.ServiceRegistration.HTTPCheckAddress
	if p.Cfg.ServiceRegistration.httpCheckAddress != "" {
		p.Cfg.ServiceRegistration.httpCheckAddress = filepath.Join(p.Cfg.ServiceRegistration.httpCheckAddress, p.Cfg.Path)
		if !strings.HasPrefix(p.Cfg.ServiceRegistration.httpCheckAddress, "http") {
			p.Cfg.ServiceRegistration.httpCheckAddress = "http://" + p.Cfg.ServiceRegistration.httpCheckAddress
		}
		return
	}
	p.Cfg.ServiceRegistration.httpCheckAddress = filepath.Join(p.Cfg.Listen, p.Cfg.Path)
	if !strings.HasPrefix(p.Cfg.ServiceRegistration.httpCheckAddress, "http") {
		p.Cfg.ServiceRegistration.httpCheckAddress = "http://" + p.Cfg.ServiceRegistration.httpCheckAddress
	}
}
