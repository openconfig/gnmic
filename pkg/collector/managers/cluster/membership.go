package cluster_manager

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/openconfig/gnmic/pkg/lockers"
)

const (
	apiServiceName = "gnmic-api"
	serviceInstanceSuffix = "-api"
)

type Membership interface {
	Register(ctx context.Context, clusterName string, self *Registration) (func() error, error)
	GetMembers(ctx context.Context) (map[string]Member, error)
	Watch(ctx context.Context) (<-chan map[string]Member, func(), error)
}

type Registration struct {
	ID      string   // instance ID
	Name    string   // service Name
	Address string   // service Address
	Port    int      //service port
	Labels  []string // labels/tags list
}

type Member struct {
	ID      string
	Address string
	Labels  []string
	Load    int64 // populated by the cluster manager based on lock count
}

type membership struct {
	locker      lockers.Locker
	logger      *slog.Logger
	clusterName string
}

func NewMembership(locker lockers.Locker, logger *slog.Logger, clusterName string) Membership {
	return &membership{locker: locker, logger: logger, clusterName: clusterName}
}

func (m *membership) GetMembers(ctx context.Context) (map[string]Member, error) {
	members := make(map[string]Member)
	srvs, err := m.locker.GetServices(ctx, m.serviceName(), nil)
	if err != nil {
		return nil, err
	}
	for _, srv := range srvs {
		members[srv.ID] = Member{ID: srv.ID, Address: srv.Address, Labels: srv.Tags}
	}
	return members, nil
}

func (m *membership) Watch(ctx context.Context) (<-chan map[string]Member, func(), error) {
	lockerCh := make(chan []*lockers.Service)
	ctx, cancel := context.WithCancel(ctx)
	serviceName := m.serviceName()
	m.logger.Info("watching services", "serviceName", serviceName)
	go m.locker.WatchServices(ctx, serviceName, []string{}, lockerCh, 10*time.Second)

	ch := make(chan map[string]Member)

	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case srvs, ok := <-lockerCh:
				if !ok {
					return
				}
				members := make(map[string]Member)
				for _, srv := range srvs {
					members[srv.ID] = Member{ID: srv.ID, Address: srv.Address, Labels: srv.Tags}
				}
				select {
				case <-ctx.Done():
					return
				case ch <- members:
				}
			}
		}
	}()
	return ch, func() {
		cancel()
		close(ch)
	}, nil
}

func (m *membership) Register(ctx context.Context, clusterName string, self *Registration) (func() error, error) {
	ctx, cancel := context.WithCancel(ctx)
	err := m.locker.Register(ctx, &lockers.ServiceRegistration{
		ID:      self.ID,
		Name:    fmt.Sprintf("%s-%s", clusterName, apiServiceName),
		Address: self.Address,
		Port:    self.Port,
		Tags:    self.Labels,
		TTL:     10 * time.Second, // TODO: make this configurable
	})
	return func() error {
		cancel()
		return m.locker.Deregister(self.ID)
	}, err
}

func (m *membership) serviceName() string {
	return fmt.Sprintf("%s-%s", m.clusterName, apiServiceName)
}
