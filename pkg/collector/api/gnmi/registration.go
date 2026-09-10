// SPDX-License-Identifier: Apache-2.0

package gnmiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/hashicorp/consul/api"

	"github.com/openconfig/gnmic/pkg/logging"
)

const consulRetryTimer = time.Second

// registerService registers the gNMI server in Consul and maintains its TTL
// check until ctx is done, at which point the service is deregistered.
func (s *Server) registerService(ctx context.Context, defaultTags ...string) {
	if s.cfg.ServiceRegistration == nil {
		return
	}
	clientConfig := &api.Config{
		Address:    s.cfg.ServiceRegistration.Address,
		Scheme:     "http",
		Datacenter: s.cfg.ServiceRegistration.Datacenter,
		Token:      s.cfg.ServiceRegistration.Token,
	}
	if s.cfg.ServiceRegistration.Username != "" && s.cfg.ServiceRegistration.Password != "" {
		clientConfig.HttpAuth = &api.HttpBasicAuth{
			Username: s.cfg.ServiceRegistration.Username,
			Password: s.cfg.ServiceRegistration.Password,
		}
	}

	service, err := s.buildServiceRegistration(defaultTags)
	if err != nil {
		s.logger.Error("failed to build service registration", "error", err)
		return
	}
	ttlCheckID := "service:" + service.ID
	b, _ := json.Marshal(service)
	s.logger.Info("registering gNMI server service", "service", string(b))

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		consulClient, err := api.NewClient(clientConfig)
		if err != nil {
			logging.LogErrUnlessCanceled(s.logger, err, "failed to create consul client")
			time.Sleep(consulRetryTimer)
			continue
		}
		if _, err = consulClient.Agent().Self(); err != nil {
			logging.LogErrUnlessCanceled(s.logger, err, "failed to connect to consul")
			time.Sleep(consulRetryTimer)
			continue
		}
		err = consulClient.Agent().ServiceRegister(service)
		if err != nil {
			logging.LogErrUnlessCanceled(s.logger, err, "failed to register service in consul")
			time.Sleep(consulRetryTimer)
			continue
		}
		err = consulClient.Agent().UpdateTTL(ttlCheckID, "", api.HealthPassing)
		if err != nil {
			logging.LogErrUnlessCanceled(s.logger, err, "failed to update TTL check to Passing")
		}
		// maintain the TTL check, return true when ctx is done.
		if s.maintainTTL(ctx, consulClient, ttlCheckID) {
			err = consulClient.Agent().ServiceDeregister(service.ID)
			if err != nil {
				s.logger.Error("failed to deregister service from consul", "error", err)
			}
			return
		}
		// TTL update failed, reconnect and re-register.
	}
}

// maintainTTL periodically refreshes the service TTL check.
// It returns true if ctx is done and false if the TTL update failed
// and the service should be re-registered.
func (s *Server) maintainTTL(ctx context.Context, consulClient *api.Client, ttlCheckID string) bool {
	ticker := time.NewTicker(s.cfg.ServiceRegistration.CheckInterval / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := consulClient.Agent().UpdateTTL(ttlCheckID, "", api.HealthPassing)
			if err != nil {
				logging.LogErrUnlessCanceled(s.logger, err, "failed to update TTL check to Passing")
				return false
			}
		case <-ctx.Done():
			return true
		}
	}
}

func (s *Server) buildServiceRegistration(defaultTags []string) (*api.AgentServiceRegistration, error) {
	h, p, err := net.SplitHostPort(s.cfg.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to split host and port from gNMI server address %q: %w", s.cfg.Address, err)
	}
	pi, _ := strconv.Atoi(p)
	service := &api.AgentServiceRegistration{
		ID:      s.instanceName(),
		Name:    s.cfg.ServiceRegistration.Name,
		Address: h,
		Port:    pi,
		Tags:    append(defaultTags, s.cfg.ServiceRegistration.Tags...),
		Checks: api.AgentServiceChecks{
			{
				TTL:                            s.cfg.ServiceRegistration.CheckInterval.String(),
				DeregisterCriticalServiceAfter: s.cfg.ServiceRegistration.DeregisterAfter,
			},
		},
	}
	if clustering := s.getClusteringConfig(); clustering != nil {
		if clustering.InstanceName != "" {
			service.ID = clustering.InstanceName
		}
		service.Name = clustering.ClusterName + "-gnmi-server"
		if service.Tags == nil {
			service.Tags = make([]string, 0)
		}
		service.Tags = append(service.Tags, fmt.Sprintf("cluster-name=%s", clustering.ClusterName))
	}
	if service.ID == "" {
		service.ID = service.Name
	}
	service.Tags = append(service.Tags, fmt.Sprintf("instance-name=%s", service.ID))
	return service, nil
}
