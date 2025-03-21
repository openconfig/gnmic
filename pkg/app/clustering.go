// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/lockers"
)

const (
	defaultClusterName = "default-cluster"
	retryTimer         = 10 * time.Second
	lockWaitTime       = 100 * time.Millisecond
	apiServiceName     = "gnmic-api"
	protocolTagName    = "__protocol"
	maxRebalanceLoop   = 100
)

var (
	errNoMoreSuitableServices = errors.New("no more suitable services for this target")
	errNotFound               = errors.New("not found")
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

func (a *App) leaderKey() string {
	return fmt.Sprintf("gnmic/%s/leader", a.Config.Clustering.ClusterName)
}

func (a *App) inCluster() bool {
	if a.Config == nil {
		return false
	}
	return !(a.Config.Clustering == nil)
}

func (a *App) apiServiceRegistration() {
	addr, port, _ := net.SplitHostPort(a.Config.APIServer.Address)
	p, _ := strconv.Atoi(port)

	tags := make([]string, 0, 2+len(a.Config.Clustering.Tags))
	tags = append(tags, fmt.Sprintf("cluster-name=%s", a.Config.Clustering.ClusterName))
	tags = append(tags, fmt.Sprintf("instance-name=%s", a.Config.Clustering.InstanceName))
	if a.Config.APIServer.TLS != nil {
		tags = append(tags, protocolTagName+"=https")
	} else {
		tags = append(tags, protocolTagName+"=http")
	}
	tags = append(tags, a.Config.Clustering.Tags...)

	serviceReg := &lockers.ServiceRegistration{
		ID:      a.Config.Clustering.InstanceName + "-api",
		Name:    fmt.Sprintf("%s-%s", a.Config.Clustering.ClusterName, apiServiceName),
		Address: a.Config.Clustering.ServiceAddress,
		Port:    p,
		Tags:    tags,
		TTL:     5 * time.Second,
	}
	if serviceReg.Address == "" {
		serviceReg.Address = addr
	}
	var err error
	a.Logger.Printf("registering service %+v", serviceReg)
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			err = a.locker.Register(a.ctx, serviceReg)
			if err != nil {
				a.Logger.Printf("api service registration failed: %v", err)
				time.Sleep(retryTimer)
				continue
			}
			return
		}
	}
}

func (a *App) startCluster() {
	if a.locker == nil || a.Config.Clustering == nil {
		return
	}

	// register api service
	go a.apiServiceRegistration()

	leaderKey := a.leaderKey()
	var err error
START:
	// acquire leader key lock
	for {
		a.isLeader = false
		err = nil
		a.isLeader, err = a.locker.Lock(a.ctx, leaderKey, []byte(a.Config.Clustering.InstanceName))
		if err != nil {
			a.Logger.Printf("failed to acquire leader lock: %v", err)
			time.Sleep(retryTimer)
			continue
		}
		if !a.isLeader {
			time.Sleep(retryTimer)
			continue
		}
		a.isLeader = true
		a.Logger.Printf("%q became the leader", a.Config.Clustering.InstanceName)
		break
	}
	ctx, cancel := context.WithCancel(a.ctx)
	defer cancel()
	go func() {
		go a.watchMembers(ctx)
		a.Logger.Printf("leader waiting %s before dispatching targets", a.Config.Clustering.LeaderWaitTimer)
		time.Sleep(a.Config.Clustering.LeaderWaitTimer)
		a.Logger.Printf("leader done waiting, starting loader and dispatching targets")
		go a.startLoader(ctx)
		go a.dispatchTargets(ctx)
	}()

	doneCh, errCh := a.locker.KeepLock(ctx, leaderKey)
	select {
	case <-doneCh:
		a.Logger.Printf("%q lost leader role", a.Config.Clustering.InstanceName)
		cancel()
		a.isLeader = false
		time.Sleep(retryTimer)
		goto START
	case err := <-errCh:
		a.Logger.Printf("%q failed to maintain the leader key: %v", a.Config.Clustering.InstanceName, err)
		cancel()
		a.isLeader = false
		time.Sleep(retryTimer)
		goto START
	case <-a.ctx.Done():
		return
	}
}

func (a *App) watchMembers(ctx context.Context) {
	serviceName := fmt.Sprintf("%s-%s", a.Config.Clustering.ClusterName, apiServiceName)
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
		err := a.locker.WatchServices(ctx, serviceName, []string{"cluster-name=" + a.Config.Clustering.ClusterName}, membersChan, a.Config.Clustering.ServicesWatchTimer)
		if err != nil {
			a.Logger.Printf("failed getting services: %v", err)
			time.Sleep(retryTimer)
			goto START
		}
	}
}

func (a *App) updateServices(srvs []*lockers.Service) {
	a.configLock.Lock()
	defer a.configLock.Unlock()

	numNewSrv := len(srvs)
	numCurrentSrv := len(a.apiServices)

	a.Logger.Printf("received service update with %d service(s)", numNewSrv)
	// no new services and no current services, continue
	if numNewSrv == 0 && numCurrentSrv == 0 {
		return
	}

	// no new services and having some services, delete all
	if numNewSrv == 0 && numCurrentSrv != 0 {
		a.Logger.Printf("deleting all services")
		a.apiServices = make(map[string]*lockers.Service)
		return
	}
	// no current services, add all new services
	if numCurrentSrv == 0 {
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
				a.Logger.Printf("no services found, waiting...")
				time.Sleep(a.Config.Clustering.TargetsWatchTimer)
				continue
			}
			a.dispatchLock.Lock()
			a.dispatchTargetsOnce(ctx)
			a.dispatchLock.Unlock()
			select {
			case <-ctx.Done():
				return
			default:
				time.Sleep(a.Config.Clustering.TargetsWatchTimer)
			}
		}
	}
}

func (a *App) dispatchTargetsOnce(ctx context.Context) {
	dctx, cancel := context.WithTimeout(ctx, a.Config.Clustering.TargetsWatchTimer)
	defer cancel()
	var limiter *time.Ticker
	if a.Config.LocalFlags.SubscribeBackoff > 0 {
		limiter = time.NewTicker(a.Config.LocalFlags.SubscribeBackoff)
	}
	for _, tc := range a.Config.Targets {
		err := a.dispatchTarget(dctx, tc)
		if err != nil {
			a.Logger.Printf("failed to dispatch target %q: %v", tc.Name, err)
		}
		if err == errNotFound {
			// no registered services,
			// no need to continue with other targets,
			// break from the targets loop
			break
		}
		if err == errNoMoreSuitableServices {
			// target has no suitable matching services,
			// continue to next target without wait
			continue
		}
		if limiter != nil {
			<-limiter.C
		}
	}
	if limiter != nil {
		limiter.Stop()
	}
}

func (a *App) dispatchTarget(ctx context.Context, tc *types.TargetConfig, denied ...string) error {
	if a.Config.Debug {
		a.Logger.Printf("checking if %q is locked", tc.Name)
	}
	key := fmt.Sprintf("gnmic/%s/targets/%s", a.Config.Clustering.ClusterName, tc.Name)
	locked, err := a.locker.IsLocked(ctx, key)
	if err != nil {
		return err
	}
	if a.Config.Debug {
		a.Logger.Printf("target %q is locked: %v", tc.Name, locked)
	}
	if locked {
		return nil
	}
	a.Logger.Printf("dispatching target %q", tc.Name)
	if denied == nil {
		denied = make([]string, 0)
	}
SELECTSERVICE:
	service, err := a.selectService(tc.Tags, denied...)
	if err != nil {
		return err
	}
	if service == nil {
		goto SELECTSERVICE
	}
	a.Logger.Printf("selected service %+v", service)
	// assign target to selected service
	err = a.assignTarget(ctx, tc, service)
	if err != nil {
		// add service to denied list and reselect
		a.Logger.Printf("failed assigning target %q to service %q: %v", tc.Name, service.ID, err)
		denied = append(denied, service.ID)
		goto SELECTSERVICE
	}
	// wait for lock to be acquired
	instanceName := ""
	for _, tag := range service.Tags {
		splitTag := strings.Split(tag, "=")
		if len(splitTag) == 2 && splitTag[0] == "instance-name" {
			instanceName = splitTag[1]
		}
	}
	a.Logger.Printf("[cluster-leader] waiting for lock %q to be acquired by %q", key, instanceName)
	retries := 0
WAIT:
	values, err := a.locker.List(ctx, key)
	if err != nil {
		a.Logger.Printf("failed getting value of %q: %v", key, err)
		time.Sleep(lockWaitTime)
		goto WAIT
	}
	if len(values) == 0 {
		retries++
		if (retries+1)*int(lockWaitTime) >= int(a.Config.Clustering.TargetAssignmentTimeout) {
			a.Logger.Printf("[cluster-leader] max retries reached for target %q and service %q, reselecting...", tc.Name, service.ID)
			err = a.unassignTarget(ctx, tc.Name, service.ID)
			if err != nil {
				a.Logger.Printf("failed to unassign target %q from %q", tc.Name, service.ID)
			}
			goto SELECTSERVICE
		}
		time.Sleep(lockWaitTime)
		goto WAIT
	}
	if instance, ok := values[key]; ok {
		if instance == instanceName {
			a.Logger.Printf("[cluster-leader] lock %q acquired by %q", key, instanceName)
			return nil
		}
	}
	retries++
	if (retries+1)*int(lockWaitTime) >= int(a.Config.Clustering.TargetAssignmentTimeout) {
		a.Logger.Printf("[cluster-leader] max retries reached for target %q and service %q, reselecting...", tc.Name, service.ID)
		err = a.unassignTarget(ctx, tc.Name, service.ID)
		if err != nil {
			a.Logger.Printf("failed to unassign target %q from %q", tc.Name, service.ID)
		}
		goto SELECTSERVICE
	}
	time.Sleep(lockWaitTime)
	goto WAIT
}

func (a *App) selectService(tags []string, denied ...string) (*lockers.Service, error) {
	numServices := len(a.apiServices)
	switch numServices {
	case 0:
		return nil, errNotFound
	case 1:
		for _, s := range a.apiServices {
			return s, nil
		}
	default:
		// select instance by tags
		matchingInstances := make([]string, 0)
		tagCount := a.getInstancesTagsMatches(tags)
		if len(tagCount) > 0 {
			matchingInstances = a.getHighestTagsMatches(tagCount)
			a.Logger.Printf("current instances with tags=%v: %+v", tags, matchingInstances)
		} else {
			for n := range a.apiServices {
				matchingInstances = append(matchingInstances, strings.TrimSuffix(n, "-api"))
			}
		}
		if len(matchingInstances) == 1 {
			return a.apiServices[fmt.Sprintf("%s-api", matchingInstances[0])], nil
		}
		// select instance by load
		load, err := a.getInstancesLoad(matchingInstances...)
		if err != nil {
			return nil, err
		}
		a.Logger.Printf("current instances load: %+v", load)
		// if there are no locks in place, return a random service
		if len(load) == 0 {
			for _, n := range matchingInstances {
				a.Logger.Printf("selected service name: %s", n)
				return a.apiServices[fmt.Sprintf("%s-api", n)], nil
			}
		}
		for _, d := range denied {
			delete(load, strings.TrimSuffix(d, "-api"))
		}
		a.Logger.Printf("current instances load after filtering: %+v", load)
		// all services were denied
		if len(load) == 0 {
			return nil, errNoMoreSuitableServices
		}
		ss := a.getLowLoadInstance(load)
		a.Logger.Printf("selected service name: %s", ss)
		if srv, ok := a.apiServices[fmt.Sprintf("%s-api", ss)]; ok {
			return srv, nil
		}
		return a.apiServices[ss], nil
	}
	return nil, errNotFound
}

func (a *App) getInstancesLoad(instances ...string) (map[string]int, error) {
	// read all current locks held by the cluster
	locks, err := a.locker.List(a.ctx, fmt.Sprintf("gnmic/%s/targets", a.Config.Clustering.ClusterName))
	if err != nil {
		return nil, err
	}
	if a.Config.Debug {
		a.Logger.Println("current locks:", locks)
	}
	load := make(map[string]int)
	// using the read locks, calculate the number of targets each instance has locked
	for _, instance := range locks {
		if _, ok := load[instance]; !ok {
			load[instance] = 0
		}
		load[instance]++
	}
	// for instances that are registered but do not have any lock,
	// add a "0" load
	for _, s := range a.apiServices {
		instance := strings.TrimSuffix(s.ID, "-api")
		if _, ok := load[instance]; !ok {
			load[instance] = 0
		}
	}
	if len(instances) > 0 {
		filteredLoad := make(map[string]int)
		for _, instance := range instances {
			if l, ok := load[instance]; ok {
				filteredLoad[instance] = l
			} else {
				filteredLoad[instance] = 0
			}
		}
		return filteredLoad, nil
	}
	return load, nil
}

// loop through the current cluster load
// find the instance with the lowest load
func (a *App) getLowLoadInstance(load map[string]int) string {
	var ss string
	var low = -1
	for s, l := range load {
		if low < 0 || l < low {
			ss = s
			low = l
		}
	}
	return ss
}

// loop through the current cluster load
// find the instance(s) with the highest and lowest load
func (a *App) getHighAndLowInstance(load map[string]int) (string, string) {
	var highIns, lowIns string
	var high = -1
	var low = -1
	for s, l := range load {
		if high < 0 || l > high {
			highIns = s
			high = l
		}
		if low < 0 || l < low {
			lowIns = s
			low = l
		}
	}
	return highIns, lowIns
}

func (a *App) getTargetToInstanceMapping(ctx context.Context) (map[string]string, error) {
	locks, err := a.locker.List(ctx, fmt.Sprintf("gnmic/%s/targets", a.Config.Clustering.ClusterName))
	if err != nil {
		return nil, err
	}
	if a.Config.Debug {
		a.Logger.Println("current locks:", locks)
	}
	for k, v := range locks {
		delete(locks, k)
		locks[filepath.Base(k)] = v
	}
	return locks, nil
}

func (a *App) getInstanceToTargetsMapping(ctx context.Context) (map[string][]string, error) {
	locks, err := a.locker.List(ctx, fmt.Sprintf("gnmic/%s/targets", a.Config.Clustering.ClusterName))
	if err != nil {
		return nil, err
	}
	if a.Config.Debug {
		a.Logger.Println("current locks:", locks)
	}
	rs := make(map[string][]string)
	for k, v := range locks {
		if _, ok := rs[v]; !ok {
			rs[v] = make([]string, 0)
		}
		rs[v] = append(rs[v], filepath.Base(k))
	}
	for _, ls := range rs {
		sort.Strings(ls)
	}
	return rs, nil
}

func (a *App) getInstancesTagsMatches(tags []string) map[string]int {
	maxMatch := make(map[string]int)
	numTags := len(tags)
	if numTags == 0 {
		return maxMatch
	}
	for name, s := range a.apiServices {
		name = strings.TrimSuffix(name, "-api")
		maxMatch[name] = 0
		for i, tag := range s.Tags {
			if i+1 > numTags {
				break
			}
			if tag == tags[i] {
				maxMatch[name]++
				continue
			}
			break
		}
	}
	return maxMatch
}

func (a *App) getHighestTagsMatches(tagsCount map[string]int) []string {
	var ss = make([]string, 0)
	var high = -1
	for s, c := range tagsCount {
		if high < 0 || c > high {
			ss = []string{strings.TrimSuffix(s, "-api")}
			high = c
			continue
		}
		if high == c {
			ss = append(ss, strings.TrimSuffix(s, "-api"))
		}
	}
	return ss
}

func (a *App) deleteTarget(ctx context.Context, name string) error {
	err := a.createAPIClient()
	if err != nil {
		return err
	}
	errs := make([]error, 0, len(a.apiServices))
	for _, s := range a.apiServices {
		scheme := a.getServiceScheme(s)
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		url := fmt.Sprintf("%s://%s/api/v1/config/targets/%s", scheme, s.Address, name)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		if err != nil {
			a.Logger.Printf("failed to create a delete request: %v", err)
			errs = append(errs, err)
			continue
		}

		rsp, err := a.clusteringClient.Do(req)
		if err != nil {
			rsp.Body.Close()
			a.Logger.Printf("failed deleting target %q: %v", name, err)
			errs = append(errs, err)
			continue
		}
		rsp.Body.Close()
		a.Logger.Printf("received response code=%d, for DELETE %s", rsp.StatusCode, url)
	}
	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("there was %d error(s) while deleting target %q", len(errs), name)
}

func (a *App) assignTarget(ctx context.Context, tc *types.TargetConfig, service *lockers.Service) error {
	// encode target config
	buffer := new(bytes.Buffer)
	err := json.NewEncoder(buffer).Encode(tc)
	if err != nil {
		return err
	}
	err = a.createAPIClient()
	if err != nil {
		return err
	}
	scheme := a.getServiceScheme(service)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s://%s/api/v1/config/targets", scheme, service.Address), buffer)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.clusteringClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	a.Logger.Printf("got response code=%d for target %q config add from %q", resp.StatusCode, tc.Name, service.Address)
	if resp.StatusCode > 200 {
		return fmt.Errorf("status code=%d", resp.StatusCode)
	}
	// send target start
	req, err = http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s://%s/api/v1/targets/%s", scheme, service.Address, tc.Name), new(bytes.Buffer))
	if err != nil {
		return err
	}
	resp, err = a.clusteringClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	a.Logger.Printf("got response code=%d for target %q assignment from %q", resp.StatusCode, tc.Name, service.Address)
	if resp.StatusCode > 200 {
		return fmt.Errorf("status code=%d", resp.StatusCode)
	}
	return nil
}

func (a *App) unassignTarget(ctx context.Context, name string, serviceID string) error {
	err := a.createAPIClient()
	if err != nil {
		return err
	}
	if s, ok := a.apiServices[serviceID]; ok {
		scheme := a.getServiceScheme(s)
		url := fmt.Sprintf("%s://%s/api/v1/targets/%s", scheme, s.Address, name)
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		if err != nil {
			return err
		}
		rsp, err := a.clusteringClient.Do(req)
		if err != nil {
			return err
		}
		a.Logger.Printf("received response code=%d, for DELETE %s", rsp.StatusCode, url)
	}
	return nil
}

func (a *App) getServiceScheme(service *lockers.Service) string {
	scheme := "http"
	for _, t := range service.Tags {
		if strings.HasPrefix(t, protocolTagName+"=") {
			scheme = strings.Split(t, "=")[1]
			break
		}
	}
	return scheme
}

func (a *App) createAPIClient() error {
	if a.clusteringClient != nil {
		return nil
	}
	// no certs
	if a.Config.Clustering.TLS == nil {
		a.clusteringClient = &http.Client{
			Timeout: defaultHTTPClientTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}
		return nil
	}
	// with certs
	tlsConfig, err := utils.NewTLSConfig(
		a.Config.Clustering.TLS.CaFile,
		a.Config.Clustering.TLS.CertFile,
		a.Config.Clustering.TLS.KeyFile, "",
		a.Config.Clustering.TLS.SkipVerify,
		false)
	if err != nil {
		return err
	}
	a.clusteringClient = &http.Client{
		Timeout: defaultHTTPClientTimeout,
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
	}
	return nil
}

func (a *App) clusterRebalanceTargets() error {
	a.dispatchLock.Lock()
	defer a.dispatchLock.Unlock()

	rebalanceCount := 0 // counts the number of iterations
	maxIter := -1       // stores the maximum expected number of iterations
	for {
		// get most loaded and least loaded
		load, err := a.getInstancesLoad()
		if err != nil {
			return err
		}
		highest, lowest := a.getHighAndLowInstance(load)
		lowLoad := load[lowest]
		highLoad := load[highest]
		delta := highLoad - lowLoad
		if maxIter < 0 { // set max number of iteration to delta/2
			maxIter = delta / 2
			if maxIter > maxRebalanceLoop {
				maxIter = maxRebalanceLoop
			}
		}
		a.Logger.Printf("rebalancing: high instance: %s=%d, low instance %s=%d", highest, highLoad, lowest, lowLoad)
		// nothing to do
		if delta < 2 {
			return nil
		}
		if rebalanceCount >= maxIter {
			return nil
		}
		// there is some work to do
		// get highest load instance targets
		highInstanceTargets, err := a.getInstanceTargets(a.ctx, highest)
		if err != nil {
			return err
		}
		if len(highInstanceTargets) == 0 {
			return nil
		}
		// pick one and move it to the lowest load instance
		err = a.unassignTarget(a.ctx, highInstanceTargets[0], highest+"-api")
		if err != nil {
			return err
		}
		tc, ok := a.Config.Targets[highInstanceTargets[0]]
		if !ok {
			return fmt.Errorf("could not find target %s config", highInstanceTargets[0])
		}
		err = a.dispatchTarget(a.ctx, tc)
		if err != nil {
			return err
		}
		rebalanceCount++
	}
}
