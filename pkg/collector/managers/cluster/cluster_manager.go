package cluster_manager

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"path"
	"sort"
	"strconv"
	"sync"
	"time"

	apiconst "github.com/openconfig/gnmic/pkg/collector/api/const"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/config/store"
	"github.com/openconfig/gnmic/pkg/lockers"
)

const (
	protocolTagName          = "__protocol"
	retryRegistrationBackoff = 2 * time.Second
)

type ClusterManager struct {
	configStore      store.Store[any]
	clusteringConfig *config.Clustering
	apiConfig        *config.APIServer

	locker lockers.Locker

	election   Election
	membership Membership
	placer     Placer
	assigner   Assigner

	apiClient *http.Client

	mm      *sync.RWMutex
	members map[string]Member

	logger *slog.Logger
	wg     *sync.WaitGroup
	cfn    context.CancelFunc
}

func NewClusterManager(configStore store.Store[any]) *ClusterManager {
	return &ClusterManager{
		configStore: configStore,
		mm:          new(sync.RWMutex),
		members:     make(map[string]Member),
		locker:      nil,
		logger:      slog.With("component", "cluster-manager"),
		apiClient:   &http.Client{Timeout: 10 * time.Second}, // TODO:
	}
}

func (c *ClusterManager) Start(ctx context.Context, locker lockers.Locker, wg *sync.WaitGroup) error {
	c.locker = locker
	ctx, cfn := context.WithCancel(ctx)
	c.cfn = cfn
	c.wg = wg
	//get clustring config from store
	clusteringConfig, ok, err := c.configStore.Get("clustering", "clustering")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	clustering, ok := clusteringConfig.(*config.Clustering)
	if !ok {
		return nil
	}
	if clustering == nil {
		return nil
	}
	c.clusteringConfig = clustering
	apiConfig, ok, err := c.configStore.Get("api-server", "api-server")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	api, ok := apiConfig.(*config.APIServer)
	if !ok {
		return errors.New("missing api-server config when clustring is enabled")
	}
	if api == nil {
		return errors.New("missing api-server config when clustring is enabled")
	}
	c.apiConfig = api
	c.logger.Info("starting cluster manager")

	c.membership = NewMembership(c.locker, c.logger, clustering.ClusterName)
	c.election = NewElection(c.locker, clustering.ClusterName, clustering.InstanceName, clustering.TargetsWatchTimer, clustering.TargetsWatchTimer/2, c.logger)
	c.placer = NewPlacer(c.locker, c.logger)
	c.assigner = NewAssigner(c.configStore)

	// start registration to register the api service
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				err := c.startRegistration(ctx)
				if err != nil {
					c.logger.Error("startRegistration exited with error", "error", err)
					select {
					case <-ctx.Done():
						return
					case <-time.After(retryRegistrationBackoff):
						continue
					}
				}
			}
		}
	}()

	wg.Add(1)
	// run election campaign to grab the leader lock
	// when grabbed, start leader duties
	go func() {
		defer wg.Done()
		err := c.runCampaign(ctx)
		if err != nil {
			c.logger.Error("runCampaign exited with error", "error", err)
		}
	}()

	return nil
}

func (c *ClusterManager) Stop() error {
	if c.cfn != nil {
		c.cfn()
	}
	return nil
}

func (c *ClusterManager) runCampaign(ctx context.Context) error {
	backoff := time.Second

	for {
		if ctx.Err() != nil {
			return nil
		}

		term, err := c.election.Campaign(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			c.logger.Error("failed to campaign", "error", err)
			time.Sleep(backoff)
			continue
		}

		c.logger.Info("became leader", "term", term, "node", c.clusteringConfig.InstanceName, "cluster", c.clusteringConfig.ClusterName)
		// Leader session context
		leaderCtx, leaderCancel := context.WithCancel(ctx)
		cancelLeader := func() {
			leaderCancel()
			// TODO: any extra cleanups?
		}

		// Start leader duties
		go func() {
			if err := c.runLeader(leaderCtx); err != nil && leaderCtx.Err() == nil {
				c.logger.Error("runLeader exited with error", "err", err)
			}
		}()

		// this blocks until leadership is lost or we’re shutting down.
		lost := c.election.Observe(ctx)
		if lost != nil {
			select {
			case <-ctx.Done():
				cancelLeader()
				return nil
			case <-lost:
				c.logger.Warn("leadership lost", "term", term)
				cancelLeader()
			}
		} else {
			// Shouldn't happen
			c.logger.Warn("Observe returned nil channel; cancelling leader")
			cancelLeader()
		}

		time.Sleep(backoff) // small backoff before campaigning again
	}
}

func (c *ClusterManager) startRegistration(ctx context.Context) error {
	c.logger.Info("starting registration", "address", c.apiConfig.Address)
	addr, port, _ := net.SplitHostPort(c.apiConfig.Address)
	p, _ := strconv.Atoi(port)

	tags := make([]string, 0, 2+len(c.clusteringConfig.Tags))
	tags = append(tags, fmt.Sprintf("cluster-name=%s", c.clusteringConfig.ClusterName))
	tags = append(tags, fmt.Sprintf("instance-name=%s", c.clusteringConfig.InstanceName))
	if c.apiConfig.TLS != nil {
		tags = append(tags, protocolTagName+"=https")
	} else {
		tags = append(tags, protocolTagName+"=http")
	}
	tags = append(tags, c.clusteringConfig.Tags...)

	address := c.clusteringConfig.ServiceAddress
	if address == "" {
		address = addr
	}
	deregister, err := c.membership.Register(ctx, c.clusteringConfig.ClusterName,
		&Registration{
			ID:      c.clusteringConfig.InstanceName + serviceInstanceSuffix,
			Address: address,
			Port:    p,
			Labels:  tags,
		})
	defer deregister()
	if err != nil {
		return err
	}
	return nil
}

// runLeader executes as long as this node is the elected leader.
// It continuously reconciles cluster state: assigns targets to nodes,
// verifies ownership via locks, and updates assignments in the store.
func (c *ClusterManager) runLeader(ctx context.Context) error {
	c.logger.Info("starting leader duties")

	// watch membership (other nodes joining/leaving)
	membersCh, cancelMembers, err := c.membership.Watch(ctx)
	if err != nil {
		return fmt.Errorf("failed to start watching membership: %w", err)
	}
	defer cancelMembers()

	// watch targets
	targetsCh, cancelTargets, err := c.configStore.Watch("targets") // no initial replay
	if err != nil {
		return fmt.Errorf("failed to watch targets: %w", err)
	}
	defer cancelTargets()

	// ticker for periodic reconciliation of target assignments
	targetsWatchTicker := time.NewTicker(c.clusteringConfig.TargetsWatchTimer)
	defer targetsWatchTicker.Stop()

	members, err := c.membership.GetMembers(ctx)
	if err != nil {
		c.logger.Error("failed to get members", "error", err) //log error but continue
	} else {
		c.mm.Lock()
		c.members = members
		c.mm.Unlock()
		if err := c.reconcileAssignments(ctx, members); err != nil {
			c.logger.Error("reconcile assignments failed", "error", err)
		}
	}
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("stopping leader duties")
			return nil
		case members, ok := <-membersCh:
			if !ok {
				c.logger.Warn("membership watcher closed") // happens only when explicitly closed
				return nil
			}
			c.logger.Info("membership update", "members", members)
			c.mm.Lock()
			c.members = members
			c.mm.Unlock()
		case targets, ok := <-targetsCh:
			if !ok {
				c.logger.Warn("targets watcher closed")
				return nil
			}
			c.logger.Info("targets update", "targets", targets)
			switch targets.EventType {
			case store.EventTypeCreate:
				err := c.handleTargetCreate(ctx, targets.Name)
				if err != nil {
					c.logger.Error("failed to handle target create", "target", targets.Name, "error", err)
				}
			// case store.EventTypeUpdate:
			// 	c.handleTarget(ctx, targets.Name)
			case store.EventTypeDelete:
				err := c.handleTargetDelete(ctx, targets.Name)
				if err != nil {
					c.logger.Error("failed to handle target delete", "target", targets.Name, "error", err)
				}
			}
		case <-targetsWatchTicker.C:
			// snapshot members
			if len(c.members) == 0 {
				c.logger.Warn("no members, skipping reconciliation")
				continue
			}
			members := make(map[string]Member, len(c.members))
			c.mm.RLock()
			maps.Copy(members, c.members)
			c.mm.RUnlock()
			if err := c.reconcileAssignments(ctx, members); err != nil {
				c.logger.Error("reconcile assignments failed", "error", err)
			}
		}
	}
}

func (c *ClusterManager) reconcileAssignments(ctx context.Context, members map[string]Member) error {
	// 1. List all known targets
	targets, err := c.configStore.List("targets")
	if err != nil {
		return err
	}
	// 2. get current assignments from locker
	// target -> holder
	currentAssignments, err := c.getAssignments(ctx)
	if err != nil {
		return err
	}
	c.logger.Info("current assignments", "assignments", currentAssignments)
	// 3. Decide assignments (simple hash-based or round robin)
	assignments := make(map[string]*Member) // targetName -> member

	for tName := range targets {
		currentHolder, ok := currentAssignments[tName]
		if ok {
			if m, ok := members[currentHolder.ID]; ok {
				assignments[tName] = &m
				continue
			} else {
				c.logger.Warn("target lock holder not found", "target", tName, "holder", currentHolder)
			}
		}
		assigned := c.pickAssignee(tName, members)
		if assigned == nil {
			c.logger.Warn("no assignee found for target", "target", tName)
			continue
		}
		assignments[tName] = assigned
	}

	// 4. Publish assignments with assigner
	err = c.assigner.Assign(ctx, assignments)
	if err != nil {
		c.logger.Error("failed to push assignments", "count", len(assignments), "error", err)
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(500 * time.Millisecond): // debug backoff
	}
	// 5. Optionally verify active locks on assigned targets
	for tName, member := range assignments {
		locked, err := c.locker.IsLocked(ctx, targetLockKey(tName, c.clusteringConfig.ClusterName))
		if err != nil {
			c.logger.Warn("lock check failed", "target", tName, "error", err)
			continue
		}
		_ = locked
		_ = member
		// c.logger.Info("lock check", "target", tName, "locked", locked, "member", member.ID)
	}

	return nil
}

func (c *ClusterManager) handleTargetCreate(ctx context.Context, target string) error {
	// 1. get current assignemnts
	currentAssignments, err := c.getAssignments(ctx)
	if err != nil {
		return err
	}
	c.logger.Info("current assignments", "assignments", currentAssignments)
	// 2. find least loaded member
	leastLoadedMember := c.getLeastLoadedMember(currentAssignments)
	if leastLoadedMember == nil {
		return fmt.Errorf("no least loaded member found")
	}
	// 3. assign target to member
	err = c.assigner.Assign(ctx, map[string]*Member{target: leastLoadedMember})
	if err != nil {
		c.logger.Error("failed to push assignment", "target", target, "error", err)
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(100 * time.Millisecond):
	}
	// 4. check locker
	locked, err := c.locker.IsLocked(ctx, targetLockKey(target, c.clusteringConfig.ClusterName))
	if err != nil {
		c.logger.Error("failed to check locker", "target", target, "error", err)
		return err
	}
	if !locked {
		c.logger.Error("target is not locked", "target", target)
		return fmt.Errorf("target is not locked")
	}
	return nil
}

func (c *ClusterManager) getAssignments(ctx context.Context) (map[string]Member, error) {
	currentAssignments, err := c.locker.List(ctx, targetsLockPrefix(c.clusteringConfig.ClusterName))
	if err != nil {
		return nil, err
	}
	res := make(map[string]Member)
	c.mm.RLock()
	for tName, memberName := range currentAssignments {
		// normalize targetName
		targetName := path.Base(tName)
		if m, ok := c.members[memberName+serviceInstanceSuffix]; ok {
			res[targetName] = m
		} else {
			// TODO: unknwon member ?
		}
	}
	c.mm.RUnlock()

	// calcualte load
	for tName, member := range res {
		member.Load = int64(len(currentAssignments[tName]))
	}
	return res, nil
}

func (c *ClusterManager) getAssignment(ctx context.Context, target string) (*Member, error) {
	targetKey := targetLockKey(target, c.clusteringConfig.ClusterName)
	holder, err := c.locker.List(ctx, targetKey)
	if err != nil {
		return nil, err
	}
	fmt.Println("holder", holder)
	if len(holder) == 0 {
		return nil, nil
	}

	c.mm.RLock()
	fmt.Println("member", c.members)
	member, ok := c.members[holder[targetKey]+serviceInstanceSuffix]
	c.mm.RUnlock()
	if !ok {
		return nil, nil
	}
	if member.ID == "" {
		return nil, nil
	}
	return &member, nil
}

func (c *ClusterManager) getLeastLoadedMember(assignments map[string]Member) *Member {
	var leastLoadedMember *Member
	for _, member := range assignments {
		if leastLoadedMember == nil {
			leastLoadedMember = &member
			continue
		}
		if member.Load < leastLoadedMember.Load {
			leastLoadedMember = &member
		}
	}
	return leastLoadedMember
}

func (c *ClusterManager) handleTargetDelete(ctx context.Context, target string) error {
	// find target assignment
	assignedTo, err := c.getAssignment(ctx, target)
	if err != nil {
		return err
	}
	if assignedTo == nil {
		return fmt.Errorf("target is not assigned to any member")
	}
	err = c.assigner.Unassign(ctx, target, assignedTo)
	if err != nil {
		c.logger.Error("failed to unassign target", "target", target, "error", err)
		return err
	}
	// delete from other instances
	c.mm.RLock()
	members := make(map[string]Member, len(c.members))
	maps.Copy(members, c.members)
	c.mm.RUnlock()
	// delete self from members since the initial trigger for this function is the target delete event
	delete(members, assignedTo.ID)
	err = c.deleteTargetFromMembers(ctx, target, members)
	if err != nil {
		c.logger.Error("failed to delete target from members", "target", target, "error", err)
		return err
	}
	// TODO: verify ?
	return nil
}

func (c *ClusterManager) deleteTargetFromMembers(ctx context.Context, target string, members map[string]Member) error {
	for _, member := range members {
		address := getMemberAddress(&member)
		url := address + apiconst.TargetsConfigAPIv1URL + "/" + target
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
		if err != nil {
			c.logger.Error("failed to create request", "error", err)
			return err
		}
		resp, err := c.apiClient.Do(req)
		if err != nil {
			c.logger.Error("failed to delete target", "error", err)
			return err
		}
		resp.Body.Close()
		if resp.StatusCode/100 != 2 {
			c.logger.Error("failed to delete target", "error", resp.Status)
			return fmt.Errorf("failed to delete target: %s", resp.Status)
		}
	}
	return nil
}

func (c *ClusterManager) pickAssignee(target string, members map[string]Member) *Member {
	if len(members) == 0 {
		return nil
	}
	// trivial deterministic hash
	keys := make([]string, 0, len(members))
	for id := range members {
		keys = append(keys, id)
	}
	sort.Strings(keys)
	h := fnv.New32a()
	_, _ = h.Write([]byte(target))
	idx := int(h.Sum32()) % len(keys)
	m, ok := members[keys[idx]]
	if ok {
		return &m
	}
	return nil
}

func targetsLockPrefix(clusterName string) string {
	return fmt.Sprintf("gnmic/%s/targets", clusterName)
}

func targetLockKey(target, clusterName string) string {
	return fmt.Sprintf("gnmic/%s/targets/%s", clusterName, target)
}

func targetLockHolder(ctx context.Context, lk lockers.Locker, target, clusterName string) (string, bool) {
	values, err := lk.List(ctx, targetLockKey(target, clusterName))
	if err != nil {
		return "", false
	}
	if len(values) == 0 {
		return "", false
	}
	h, ok := values[targetLockKey(target, clusterName)]
	if ok {
		return h, true
	}

	return "", false
}

func (c *ClusterManager) GetInstanceToTargetsMapping(ctx context.Context) (map[string][]string, error) {
	locks, err := c.locker.List(ctx, fmt.Sprintf("gnmic/%s/targets", c.clusteringConfig.ClusterName))
	if err != nil {
		return nil, err
	}
	rs := make(map[string][]string)
	for k, v := range locks {
		if _, ok := rs[v]; !ok {
			rs[v] = make([]string, 0)
		}
		rs[v] = append(rs[v], path.Base(k))
	}
	for _, ls := range rs {
		sort.Strings(ls)
	}
	return rs, nil
}

func (c *ClusterManager) GetLeaderName(ctx context.Context) (string, error) {
	leaderKey := fmt.Sprintf("gnmic/%s/leader", c.clusteringConfig.ClusterName)
	leader, err := c.locker.List(ctx, leaderKey)
	if err != nil {
		return "", err
	}
	if len(leader) == 0 {
		return "", nil
	}
	return leader[leaderKey], nil
}
