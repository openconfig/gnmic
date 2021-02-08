package consul_locker

import (
	"context"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/karimra/gnmic/lockers"
)

func (c *ConsulLocker) Register(ctx context.Context, s *lockers.ServiceRegistration) error {
	service := &api.AgentServiceRegistration{
		ID:      s.ID,
		Name:    s.Name,
		Address: s.Address,
		Port:    s.Port,
		Tags:    s.Tags,
		Checks: api.AgentServiceChecks{
			{
				TTL:                            s.TTL.String(),
				DeregisterCriticalServiceAfter: "5s",
			},
		},
	}
	sctx, cancel := context.WithCancel(ctx)
	c.m.Lock()
	c.services[s.ID] = cancel
	c.m.Unlock()
	ttlCheckID := "service:" + s.ID
	err := c.client.Agent().ServiceRegister(service)
	if err != nil {
		return err
	}
	// keep service with ttl
	err = c.client.Agent().UpdateTTL(ttlCheckID, "", api.HealthPassing)
	if err != nil {
		return err
	}
	ticker := time.NewTicker(s.TTL / 2)
	for {
		select {
		case <-ticker.C:
			err = c.client.Agent().UpdateTTL(ttlCheckID, "", api.HealthPassing)
			if err != nil {
				return err
			}
		case <-sctx.Done():
			c.client.Agent().UpdateTTL(ttlCheckID, ctx.Err().Error(), api.HealthCritical)
			ticker.Stop()
			return nil
		}
	}
}

func (c *ConsulLocker) Deregister(s string) error {
	c.m.Lock()
	if cfn, ok := c.services[s]; ok {
		cfn()
	}
	c.m.Unlock()
	return c.client.Agent().ServiceDeregister(s)
}
