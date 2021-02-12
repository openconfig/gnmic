package consul_locker

import (
	"context"
	"net"
	"strconv"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/karimra/gnmic/lockers"
)

const defaultWatchTimeout = 1 * time.Minute

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

func (c *ConsulLocker) WatchServices(ctx context.Context, serviceName string, tags []string, sChan chan<- []*lockers.Service, watchTimeout time.Duration) error {
	if watchTimeout <= 0 {
		watchTimeout = defaultWatchTimeout
	}
	qOpts := &api.QueryOptions{
		WaitIndex: 0,
		WaitTime:  watchTimeout,
	}
	var err error
	var index uint64
	// long blocking watch
	ticker := time.NewTicker(watchTimeout)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			c.logger.Printf("(re)starting watch service=%q, index=%d", serviceName, qOpts.WaitIndex)
			index, err = c.watch(qOpts.WithContext(ctx), serviceName, tags, sChan)
			if err != nil {
				c.logger.Printf("service %q watch failed: %v", serviceName, err)
			}
			if index > qOpts.WaitIndex {
				qOpts.WaitIndex = index
			}
			<-ticker.C
		}
	}
}

func (c *ConsulLocker) watch(qOpts *api.QueryOptions, serviceName string, tags []string, sChan chan<- []*lockers.Service) (uint64, error) {
	se, meta, err := c.client.Health().ServiceMultipleTags(serviceName, tags, true, qOpts)
	if err != nil {
		return meta.LastIndex, err
	}
	if meta.LastIndex == qOpts.WaitIndex {
		c.logger.Printf("service=%q did not change", serviceName)
		return meta.LastIndex, nil
	}
	if err != nil {
		return meta.LastIndex, err
	}
	newSrvs := make([]*lockers.Service, 0)
	for _, srv := range se {
		addr := srv.Service.Address
		if addr == "" {
			addr = srv.Node.Address
		}
		newSrvs = append(newSrvs, &lockers.Service{
			ID:      srv.Service.ID,
			Address: net.JoinHostPort(addr, strconv.Itoa(srv.Service.Port)),
			Tags:    srv.Service.Tags,
		})
	}
	sChan <- newSrvs
	return meta.LastIndex, nil
}

func (c *ConsulLocker) IsLocked(ctx context.Context, k string) (bool, error) {
	qOpts := &api.QueryOptions{}
	kv, _, err := c.client.KV().Get(k, qOpts.WithContext(ctx))
	if err != nil {
		return false, err
	}
	if kv == nil {
		return false, nil
	}
	return kv.LockIndex > 0, nil
}

func (c *ConsulLocker) List(ctx context.Context, prefix string) (map[string]string, error) {
	qOpts := &api.QueryOptions{}
	kvs, _, err := c.client.KV().List(prefix, qOpts.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	if kvs == nil {
		return nil, err
	}
	rs := make(map[string]string)
	for _, kv := range kvs {
		rs[kv.Key] = string(kv.Value)
	}
	return rs, nil
}
