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
	if p.cfg.ServiceRegistration == nil {
		return
	}
	var err error
	clientConfig := &api.Config{
		Address:    p.cfg.ServiceRegistration.Address,
		Scheme:     "http",
		Datacenter: p.cfg.ServiceRegistration.Datacenter,
		Token:      p.cfg.ServiceRegistration.Token,
	}
	if p.cfg.ServiceRegistration.Username != "" && p.cfg.ServiceRegistration.Password != "" {
		clientConfig.HttpAuth = &api.HttpBasicAuth{
			Username: p.cfg.ServiceRegistration.Username,
			Password: p.cfg.ServiceRegistration.Password,
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
	if p.cfg.ServiceRegistration.UseLock {
		doneCh, err = p.acquireAndKeepLock(ctx, "gnmic/"+p.cfg.clusterName+"/prometheus-output", []byte(p.cfg.ServiceRegistration.id))
		if err != nil {
			p.logger.Printf("failed to acquire lock: %v", err)
			time.Sleep(1 * time.Second)
			goto INITCONSUL
		}
	}

	service := &api.AgentServiceRegistration{
		ID:      p.cfg.ServiceRegistration.id,
		Name:    p.cfg.ServiceRegistration.Name,
		Address: p.cfg.address,
		Port:    p.cfg.port,
		Tags:    p.cfg.ServiceRegistration.Tags,
		Checks: api.AgentServiceChecks{
			{
				TTL:                            p.cfg.ServiceRegistration.CheckInterval.String(),
				DeregisterCriticalServiceAfter: p.cfg.ServiceRegistration.deregisterAfter,
			},
		},
	}
	ttlCheckID := "service:" + p.cfg.ServiceRegistration.id
	if p.cfg.ServiceRegistration.EnableHTTPCheck {
		service.Checks = append(service.Checks, &api.AgentServiceCheck{
			HTTP:                           p.cfg.ServiceRegistration.httpCheckAddress,
			Method:                         "GET",
			Interval:                       p.cfg.ServiceRegistration.CheckInterval.String(),
			TLSSkipVerify:                  true,
			DeregisterCriticalServiceAfter: p.cfg.ServiceRegistration.deregisterAfter,
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
	ticker := time.NewTicker(p.cfg.ServiceRegistration.CheckInterval / 2)
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
	if p.cfg.ServiceRegistration.Address == "" {
		p.cfg.ServiceRegistration.Address = defaultServiceRegistrationAddress
	}
	if p.cfg.ServiceRegistration.CheckInterval <= 5*time.Second {
		p.cfg.ServiceRegistration.CheckInterval = defaultRegistrationCheckInterval
	}
	if p.cfg.ServiceRegistration.MaxFail <= 0 {
		p.cfg.ServiceRegistration.MaxFail = defaultMaxServiceFail
	}
	deregisterTimer := p.cfg.ServiceRegistration.CheckInterval * time.Duration(p.cfg.ServiceRegistration.MaxFail)
	p.cfg.ServiceRegistration.deregisterAfter = deregisterTimer.String()

	if !p.cfg.ServiceRegistration.EnableHTTPCheck {
		return
	}
	p.cfg.ServiceRegistration.httpCheckAddress = p.cfg.ServiceRegistration.HTTPCheckAddress
	if p.cfg.ServiceRegistration.httpCheckAddress != "" {
		p.cfg.ServiceRegistration.httpCheckAddress = filepath.Join(p.cfg.ServiceRegistration.httpCheckAddress, p.cfg.Path)
		if !strings.HasPrefix(p.cfg.ServiceRegistration.httpCheckAddress, "http") {
			p.cfg.ServiceRegistration.httpCheckAddress = "http://" + p.cfg.ServiceRegistration.httpCheckAddress
		}
		return
	}
	p.cfg.ServiceRegistration.httpCheckAddress = filepath.Join(p.cfg.Listen, p.cfg.Path)
	if !strings.HasPrefix(p.cfg.ServiceRegistration.httpCheckAddress, "http") {
		p.cfg.ServiceRegistration.httpCheckAddress = "http://" + p.cfg.ServiceRegistration.httpCheckAddress
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
					TTL:       time.Duration(p.cfg.ServiceRegistration.CheckInterval * 2).String(),
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
			if p.cfg.Debug {
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
			time.Duration(p.cfg.ServiceRegistration.CheckInterval/2).String(),
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
