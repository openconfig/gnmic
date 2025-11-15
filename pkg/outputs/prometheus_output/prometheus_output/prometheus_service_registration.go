// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package prometheus_output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/consul/api"

	"github.com/openconfig/gnmic/pkg/lockers"
)

const (
	defaultServiceRegistrationAddress = "localhost:8500"
	defaultRegistrationCheckInterval  = 5 * time.Second
	defaultMaxServiceFail             = 3
)

type serviceRegistration struct {
	Address    string `mapstructure:"address,omitempty" json:"address,omitempty"`
	Datacenter string `mapstructure:"datacenter,omitempty" json:"datacenter,omitempty"`
	Username   string `mapstructure:"username,omitempty" json:"username,omitempty"`
	Password   string `mapstructure:"password,omitempty" json:"password,omitempty"`
	Token      string `mapstructure:"token,omitempty" json:"token,omitempty"`

	Name             string        `mapstructure:"name,omitempty" json:"name,omitempty"`
	CheckInterval    time.Duration `mapstructure:"check-interval,omitempty" json:"check-interval,omitempty"`
	MaxFail          int           `mapstructure:"max-fail,omitempty" json:"max-fail,omitempty"`
	Tags             []string      `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	EnableHTTPCheck  bool          `mapstructure:"enable-http-check,omitempty" json:"enable-http-check,omitempty"`
	HTTPCheckAddress string        `mapstructure:"http-check-address,omitempty" json:"http-check-address,omitempty"`
	UseLock          bool          `mapstructure:"use-lock,omitempty" json:"use-lock,omitempty"`
	ServiceAddress   string        `mapstructure:"service-address,omitempty" json:"service-address,omitempty"`

	deregisterAfter  string
	id               string
	httpCheckAddress string
}

func (p *prometheusOutput) registerService(ctx context.Context) {
	cfg := p.cfg.Load()
	if cfg == nil {
		return
	}
	if cfg.ServiceRegistration == nil {
		return
	}
	defer func() {
		p.logger.Printf("deregistering service: %s", cfg.ServiceRegistration.Name)
	}()
	p.logger.Printf("registering service: %s", cfg.ServiceRegistration.Name)
	var err error
	clientConfig := &api.Config{
		Address:    cfg.ServiceRegistration.Address,
		Scheme:     "http",
		Datacenter: cfg.ServiceRegistration.Datacenter,
		Token:      cfg.ServiceRegistration.Token,
	}
	if cfg.ServiceRegistration.Username != "" && cfg.ServiceRegistration.Password != "" {
		clientConfig.HttpAuth = &api.HttpBasicAuth{
			Username: cfg.ServiceRegistration.Username,
			Password: cfg.ServiceRegistration.Password,
		}
	}
	doneCh := make(chan struct{})
INITCONSUL:
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			p.logger.Printf("context canceled: %v", ctx.Err())
			close(doneCh)
			if p.consulClient != nil {
				err = p.consulClient.Agent().ServiceDeregister(cfg.ServiceRegistration.id)
				if err != nil {
					p.logger.Printf("failed to deregister service in consul: %v", err)
				}
			}
			return
		}
	}
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

	if cfg.ServiceRegistration.UseLock {
		doneCh, err = p.acquireAndKeepLock(ctx, "gnmic/"+cfg.clusterName+"/prometheus-output", []byte(cfg.ServiceRegistration.id))
		if err != nil {
			p.logger.Printf("failed to acquire lock: %v", err)
			time.Sleep(1 * time.Second)
			goto INITCONSUL
		}
	}

	ttlCheckID := "ttl:" + cfg.ServiceRegistration.id
	service := &api.AgentServiceRegistration{
		ID:      cfg.ServiceRegistration.id,
		Name:    cfg.ServiceRegistration.Name,
		Address: cfg.address,
		Port:    cfg.port,
		Tags:    cfg.ServiceRegistration.Tags,
		Checks: api.AgentServiceChecks{
			{
				CheckID:                        ttlCheckID,
				TTL:                            cfg.ServiceRegistration.CheckInterval.String(),
				DeregisterCriticalServiceAfter: cfg.ServiceRegistration.deregisterAfter,
			},
		},
	}
	if cfg.ServiceRegistration.EnableHTTPCheck {
		service.Checks = append(service.Checks, &api.AgentServiceCheck{
			CheckID:                        "http:" + cfg.ServiceRegistration.id,
			HTTP:                           cfg.ServiceRegistration.httpCheckAddress,
			Method:                         "GET",
			Interval:                       cfg.ServiceRegistration.CheckInterval.String(),
			TLSSkipVerify:                  true,
			DeregisterCriticalServiceAfter: cfg.ServiceRegistration.deregisterAfter,
		})
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
	ticker := time.NewTicker(cfg.ServiceRegistration.CheckInterval / 2)
	for {
		select {
		case <-ticker.C:
			err = p.consulClient.Agent().UpdateTTL(ttlCheckID, "", api.HealthPassing)
			if err != nil {
				p.logger.Printf("failed to update TTL check to Passing: %v", err)
			}
		case <-ctx.Done():
			err = p.consulClient.Agent().UpdateTTL(ttlCheckID, ctx.Err().Error(), api.HealthCritical)
			if err != nil {
				p.logger.Printf("failed to update TTL check to Critical: %v", err)
			}
			ticker.Stop()
			goto INITCONSUL
		case <-doneCh:
			ticker.Stop()
			goto INITCONSUL
		}
	}
}

func (p *prometheusOutput) setServiceRegistrationDefaults(c *config) {
	if c.ServiceRegistration.Address == "" {
		c.ServiceRegistration.Address = defaultServiceRegistrationAddress
	}
	if c.ServiceRegistration.CheckInterval <= 5*time.Second {
		c.ServiceRegistration.CheckInterval = defaultRegistrationCheckInterval
	}
	if c.ServiceRegistration.MaxFail <= 0 {
		c.ServiceRegistration.MaxFail = defaultMaxServiceFail
	}
	deregisterTimer := c.ServiceRegistration.CheckInterval * time.Duration(c.ServiceRegistration.MaxFail)
	c.ServiceRegistration.deregisterAfter = deregisterTimer.String()

	if !c.ServiceRegistration.EnableHTTPCheck {
		return
	}
	c.ServiceRegistration.httpCheckAddress = c.ServiceRegistration.HTTPCheckAddress
	if c.ServiceRegistration.httpCheckAddress != "" {
		c.ServiceRegistration.httpCheckAddress = filepath.Join(c.ServiceRegistration.httpCheckAddress, c.Path)
		if !strings.HasPrefix(c.ServiceRegistration.httpCheckAddress, "http") {
			c.ServiceRegistration.httpCheckAddress = "http://" + c.ServiceRegistration.httpCheckAddress
		}
		return
	}
	c.ServiceRegistration.httpCheckAddress = filepath.Join(c.Listen, c.Path)
	if !strings.HasPrefix(c.ServiceRegistration.httpCheckAddress, "http") {
		c.ServiceRegistration.httpCheckAddress = "http://" + c.ServiceRegistration.httpCheckAddress
	}
}

func (p *prometheusOutput) acquireLock(ctx context.Context, key string, val []byte) (string, error) {
	cfg := p.cfg.Load()
	if cfg == nil {
		return "", fmt.Errorf("config not found")
	}
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
					TTL:       time.Duration(cfg.ServiceRegistration.CheckInterval * 2).String(),
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
			if cfg.Debug {
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
		cfg := p.cfg.Load()
		if cfg == nil {
			errChan <- fmt.Errorf("config not found")
			close(doneChan)
			return
		}
		err := p.consulClient.Session().RenewPeriodic(
			time.Duration(cfg.ServiceRegistration.CheckInterval/2).String(),
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
				_, err := p.consulClient.KV().Delete(key, nil)
				if err != nil {
					p.logger.Printf("failed to delete lock from consul: %v", err)
				}
				_, err = p.consulClient.Session().Destroy(sessionID, nil)
				if err != nil {
					p.logger.Printf("failed to destroy session in consul: %v", err)
				}
				return
			case err := <-errCh:
				p.logger.Printf("failed maintaining the lock: %v", err)
				close(doneCh)
			}
		}
	}()
	return doneCh, nil
}
