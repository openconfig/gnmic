package prometheus_output

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/karimra/gnmic/lockers"
)

const (
	defaultServiceRegistrationAddress = "localhost:8500"
	defaultRegistrationCheckInterval  = 5 * time.Second
	defaultMaxServiceFail             = 3
)

type serviceRegistration struct {
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
	UseLock          bool          `mapstructure:"use-lock,omitempty"`

	deregisterAfter  string
	id               string
	httpCheckAddress string
}

func (p *prometheusOutput) registerService(ctx context.Context) {
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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	doneCh := make(chan struct{})
	if p.Cfg.ServiceRegistration.UseLock {
		doneCh, err = p.acquireAndKeepLock(ctx, "gnmic/"+p.Cfg.clusterName+"/prometheus-output", []byte(p.Cfg.ServiceRegistration.id))
		if err != nil {
			p.logger.Printf("failed to acquire lock: %v", err)
			time.Sleep(1 * time.Second)
			goto INITCONSUL
		}
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
		return
	}

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
			goto INITCONSUL
		case <-doneCh:
			goto INITCONSUL
		}
	}
}

func (p *prometheusOutput) setServiceRegistrationDefaults() {
	if p.Cfg.ServiceRegistration == nil {
		return
	}
	if p.Cfg.ServiceRegistration.Address == "" {
		p.Cfg.ServiceRegistration.Address = defaultServiceRegistrationAddress
	}
	if p.Cfg.ServiceRegistration.CheckInterval <= 5*time.Second {
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

func (p *prometheusOutput) acquireLock(ctx context.Context, key string, val []byte) (string, error) {
	var err error
	var acquired = false
	writeOpts := new(api.WriteOptions)
	writeOpts = writeOpts.WithContext(ctx)
	kvPair := &api.KVPair{Key: key, Value: val}
	doneChan := make(chan struct{})
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-doneChan:
			return "", lockers.ErrCanceled
		default:
			acquired = false
			kvPair.Session, _, err = p.consulClient.Session().Create(
				&api.SessionEntry{
					Behavior:  "delete",
					TTL:       time.Duration(p.Cfg.ServiceRegistration.CheckInterval * 2).String(),
					LockDelay: 0,
				},
				writeOpts,
			)
			if err != nil {
				p.logger.Printf("failed creating session: %v", err)
				time.Sleep(time.Second)
				continue
			}
			acquired, _, err = p.consulClient.KV().Acquire(kvPair, writeOpts)
			if err != nil {
				p.logger.Printf("failed acquiring lock to %q: %v", kvPair.Key, err)
				time.Sleep(time.Second)
				continue
			}

			if acquired {
				return kvPair.Session, nil
			}
			if p.Cfg.Debug {
				p.logger.Printf("failed acquiring lock to %q: already locked", kvPair.Key)
			}
			time.Sleep(10 * time.Second)
		}
	}
}

func (p *prometheusOutput) keepLock(ctx context.Context, sessionID string) (chan struct{}, chan error) {
	writeOpts := new(api.WriteOptions)
	writeOpts = writeOpts.WithContext(ctx)
	doneChan := make(chan struct{})
	errChan := make(chan error)
	go func() {
		if sessionID == "" {
			errChan <- fmt.Errorf("unknown key")
			close(doneChan)
			return
		}
		err := p.consulClient.Session().RenewPeriodic(
			time.Duration(p.Cfg.ServiceRegistration.CheckInterval/2).String(),
			sessionID,
			writeOpts,
			doneChan,
		)
		if err != nil {
			errChan <- err
		}
	}()

	return doneChan, errChan
}

func (p *prometheusOutput) acquireAndKeepLock(ctx context.Context, key string, val []byte) (chan struct{}, error) {
	sessionID, err := p.acquireLock(ctx, key, val)
	if err != nil {
		p.logger.Printf("failed to acquire lock: %v", err)
		return nil, err
	}

	doneCh, errCh := p.keepLock(ctx, sessionID)
	go func() {
		for {
			select {
			case <-ctx.Done():
				close(doneCh)
				return
			case <-doneCh:
				return
			case err := <-errCh:
				p.logger.Printf("failed maintaining the lock: %v", err)
				close(doneCh)
			}
		}
	}()
	return doneCh, nil
}
