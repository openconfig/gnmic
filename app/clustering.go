package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/karimra/gnmic/lockers"
)

const (
	retryTimer   = 2 * time.Second
	dispatchPace = 100 * time.Millisecond
)

func (a *App) InitLocker() error {
	if a.Config.Clustering == nil {
		return nil
	}
	if a.Config.Clustering.Locker == nil {
		return errors.New("missing locker config under clustering key")
	}

	if lockerType, ok := a.Config.Clustering.Locker["type"]; ok {
		a.Logger.Printf("starting locker type %q", lockerType)
		if initializer, ok := lockers.Lockers[lockerType.(string)]; ok {
			lock := initializer()
			err := lock.Init(a.ctx, a.Config.Clustering.Locker, lockers.WithLogger(a.Logger))
			if err != nil {
				return err
			}
			a.locker = lock
			return nil
		}
		return fmt.Errorf("unknown locker type %q", lockerType)
	}
	return errors.New("missing locker type field")
}

func (a *App) startCluster() {
	a.leaderElection()
}

func (a *App) leaderKey() string {
	return fmt.Sprintf("gnmic/%s/leader", a.Config.Clustering.ClusterName)
}

func (a *App) serviceRegistration() {
	addr, port, _ := net.SplitHostPort(a.Config.API)
	p, _ := strconv.Atoi(port)
	tags := make([]string, 0)
	if a.Config.Clustering.ClusterName != "" {
		tags = append(tags, fmt.Sprintf("cluster-name=%s", a.Config.Clustering.ClusterName))
	}
	if a.Config.Clustering.InstanceName != "" {
		tags = append(tags, fmt.Sprintf("instance-name=%s", a.Config.Clustering.InstanceName))
	}
	serviceReg := &lockers.ServiceRegistration{
		ID:      a.Config.Clustering.InstanceName + "-api",
		Name:    "gnmic-api",
		Address: addr,
		Port:    p,
		Tags:    tags,
		TTL:     5 * time.Second,
	}
	var err error
	a.Logger.Printf("registering service %+v", serviceReg)
	for {
		err = a.locker.Register(a.ctx, serviceReg)
		if err != nil {
			a.Logger.Printf("api service registration failed: %v", err)
			time.Sleep(retryTimer)
			continue
		}
		break
	}
}

func (a *App) leaderElection() {
	if a.locker == nil {
		return
	}
	leaderKey := a.leaderKey()
	var ok bool
	var err error
START:
	for {
		ok = false
		err = nil
		ok, err = a.locker.Lock(a.ctx, leaderKey, []byte(a.Config.Clustering.InstanceName))
		if err != nil {
			a.Logger.Printf("failed to acquire leader lock: %v", err)
			time.Sleep(retryTimer)
			continue
		}
		if !ok {
			time.Sleep(retryTimer)
			continue
		}
		a.Logger.Printf("%q is leader", a.Config.Clustering.InstanceName)
		break
	}

	ctx, cancel := context.WithCancel(a.ctx)
	go a.watchMembers(ctx)
	go a.dispatchTargets(ctx)

	doneCh, errCh := a.locker.KeepLock(a.ctx, leaderKey)
	select {
	case <-doneCh:
		cancel()
		goto START
	case err := <-errCh:
		a.Logger.Printf("failed to maintain leader key: %v", err)
		cancel()
		goto START
	}
}

func (a *App) watchMembers(ctx context.Context) {
START:
	select {
	case <-ctx.Done():
		return
	default:
		membersChan := make(chan []*lockers.Service)
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case srvs, ok := <-membersChan:
					if !ok {
						return
					}
					a.updateServices(srvs)
				}
			}
		}()
		err := a.locker.WatchServices(a.ctx, "gnmic-api", nil, membersChan, 5*time.Second)
		if err != nil {
			a.Logger.Printf("failed getting services: %v", err)
			time.Sleep(5 * time.Second)
			goto START
		}
	}
}

func (a *App) updateServices(srvs []*lockers.Service) {
	a.m.Lock()
	defer a.m.Unlock()

	numNewSrv := len(srvs)
	numCurrSrv := len(a.apiServices)

	a.Logger.Printf("service update with %d service(s)", numNewSrv)
	// no new services and no current services, continue
	if numNewSrv == 0 && numCurrSrv == 0 {
		return
	}

	// no new services and having some services, delete all
	if numNewSrv == 0 && numCurrSrv != 0 {
		a.Logger.Printf("deleting all services")
		a.apiServices = make(map[string]*lockers.Service)
		return
	}
	// no current services, add all new services
	if numCurrSrv == 0 {
		for _, s := range srvs {
			a.Logger.Printf("adding service id %q", s.ID)
			a.apiServices[s.ID] = s
		}
		return
	}
	//
	newSrvs := make(map[string]*lockers.Service)
	for _, s := range srvs {
		newSrvs[s.ID] = s
	}
	// delete removed services
	for n := range a.apiServices {
		if _, ok := newSrvs[n]; !ok {
			a.Logger.Printf("deleting service id %q", n)
			delete(a.apiServices, n)
		}
	}
	// add new services
	for n, s := range newSrvs {
		a.Logger.Printf("adding service id %q", n)
		a.apiServices[n] = s
	}
}

func (a *App) dispatchTargets(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if len(a.apiServices) == 0 {
				time.Sleep(retryTimer)
				continue
			}
			for _, tc := range a.Config.Targets {
				locked, err := a.locker.IsLocked(ctx, fmt.Sprintf("gnmic/%s/targets/%s", a.Config.Clustering.ClusterName, tc.Name))
				if err != nil {
					a.Logger.Printf("failed to check if target %q is locked: %v", tc.Name, err)
					continue
				}
				if a.Config.Debug {
					a.Logger.Printf("target %q is locked: %v", tc.Name, locked)
				}
				if locked {
					continue
				}
			SELECTSERVICE:
				service, err := a.selectService()
				if err != nil {
					a.Logger.Printf("failed selecting a service: %v", err)
					goto SELECTSERVICE
				}
				a.Logger.Printf("selected service %+v", service)
				resp, err := a.httpClient.Post("http://"+service.Address+"/targets/"+tc.Name, "", new(bytes.Buffer))
				if err != nil {
					a.Logger.Printf("failed to send targets assignment to %s: %v", service.Address, err)
					goto SELECTSERVICE
				}
				a.Logger.Printf("got response code=%d for target %q assignment", resp.StatusCode, tc.Name)
				if resp.StatusCode > 200 {
					goto SELECTSERVICE
				}
				time.Sleep(dispatchPace)
			}

			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(retryTimer)
			}
		}
	}
}

func (a *App) selectService() (*lockers.Service, error) {
	numServices := len(a.apiServices)
	switch numServices {
	case 0:
		return nil, errors.New("not found")
	case 1:
		for _, s := range a.apiServices {
			return s, nil
		}
	default:
		load, err := a.getInstancesLoad()
		if err != nil {
			return nil, err
		}
		a.Logger.Printf("current instances load: %v", load)
		var ss string
		var low = -1
		for s, l := range load {
			if low < 0 || l < low {
				ss = s
				low = l
			}
		}
		// there are no locks in place, return a random service
		if ss == "" {
			for _, s := range a.apiServices {
				a.Logger.Printf("selected service: %s", s.ID)
				return s, nil
			}
		}
		a.Logger.Printf("selected service: %s", ss)
		a.m.Lock()
		defer a.m.Unlock()
		return a.apiServices[ss+"-api"], nil
	}
	return nil, nil
}

func (a *App) getInstancesLoad() (map[string]int, error) {
	locks, err := a.locker.List(a.ctx, fmt.Sprintf("gnmic/%s/targets", a.Config.Clustering.ClusterName))
	if err != nil {
		return nil, err
	}
	if a.Config.Debug {
		a.Logger.Println("current locks:", locks)
	}
	load := make(map[string]int)
	for _, instance := range locks {
		if _, ok := load[instance]; !ok {
			load[instance] = 0
		}
		load[instance]++
	}
	for _, s := range a.apiServices {
		instanceName := strings.TrimSuffix(s.ID, "-api")
		if _, ok := load[instanceName]; !ok {
			load[instanceName] = 0
		}
	}
	if a.Config.Debug {
		a.Logger.Printf("calculated instances load: %+v", load)
	}
	return load, nil
}
