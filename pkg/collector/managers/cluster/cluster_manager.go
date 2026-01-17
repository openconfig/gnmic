package cluster_manager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"path"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"

	apiconst "github.com/openconfig/gnmic/pkg/collector/api/const"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/lockers"
	"github.com/openconfig/gnmic/pkg/logging"
	"github.com/zestor-dev/zestor/store"
)

const (
	protocolTagName          = "__protocol"
	retryRegistrationBackoff = 2 * time.Second
)

type ClusterManager struct {
	store            store.Store[any]
	clusteringConfig *config.Clustering
	apiConfig        *config.APIServer

	locker lockers.Locker

	election           Election
	recampaignCooldown atomic.Int64

	membership       Membership
	assigner         Assigner
	lockCheckLimiter chan struct{}

	// semaphore to limit the number of concurrent rebalancing operations (to 1)
	rebalancingSem *semaphore.Weighted

	apiClient *http.Client

	mm      *sync.RWMutex
	members map[string]*Member

	logger *slog.Logger
	wg     *sync.WaitGroup
	cfn    context.CancelFunc
}

func NewClusterManager(store store.Store[any]) *ClusterManager {
	return &ClusterManager{
		store:            store,
		mm:               new(sync.RWMutex),
		members:          make(map[string]*Member),
		locker:           nil,
		lockCheckLimiter: make(chan struct{}, 64), // TODO: make this configurable
		rebalancingSem:   semaphore.NewWeighted(1),
		apiClient:        &http.Client{Timeout: 10 * time.Second}, // TODO:
	}
}

func (c *ClusterManager) Start(ctx context.Context, locker lockers.Locker, wg *sync.WaitGroup) error {
	c.locker = locker
	c.logger = logging.NewLogger(c.store, "component", "cluster-manager")
	ctx, cfn := context.WithCancel(ctx)
	c.cfn = cfn
	c.wg = wg
	//get clustring config from store
	clusteringConfig, ok, err := c.store.Get("clustering", "clustering")
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
	apiConfig, ok, err := c.store.Get("api-server", "api-server")
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

	c.election, err = NewElection(c.locker, clustering, c.logger)
	if err != nil {
		return err
	}
	c.membership = NewMembership(c.locker, clustering, c.logger)
	c.assigner = NewAssigner(c.store)

	// start registration to register the api service
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			if ctx.Err() != nil {
				return
			}
			if err := c.startRegistration(ctx); err != nil {
				c.logger.Error("registration failed", "err", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(retryRegistrationBackoff):
					continue
				}
			}
			// startRegistration should block until ctx.Done(); when it returns, exit.
			return
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

		// Cooldown after an API triggered withdraw
		if wait := c.recampaignCooldown.Load(); wait > 0 {
			c.logger.Info("waiting for cooldown", "cooldown", time.Duration(wait))
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(time.Duration(wait)):
			}
			// reset
			c.recampaignCooldown.Store(0)
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
			ID:      c.clusteringConfig.InstanceName,
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

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(c.clusteringConfig.LeaderWaitTimer):
		break
	}

	// watch membership (other nodes joining/leaving)
	membersCh, cancelMembers, err := c.membership.Watch(ctx)
	if err != nil {
		return fmt.Errorf("failed to start watching membership: %w", err)
	}
	defer cancelMembers()

	// watch targets
	targetsCh, cancelTargets, err := c.store.Watch("targets") // no initial replay
	if err != nil {
		return fmt.Errorf("failed to watch targets: %w", err)
	}
	defer cancelTargets()

	// ticker for periodic reconciliation of target assignments
	targetsWatchTicker := time.NewTicker(c.clusteringConfig.TargetsWatchTimer)
	defer targetsWatchTicker.Stop()

	// intial membership sync
	members, err := c.membership.GetMembers(ctx)
	if err != nil {
		c.logger.Error("failed to get members", "error", err) //log error but continue
	} else {
		// initial reconcile
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
			// periodic reconciliation of target assignments
			members := c.snapshotMembers()
			if len(members) == 0 {
				c.logger.Warn("no members, skipping reconciliation")
				continue
			}
			if err := c.reconcileAssignments(ctx, members); err != nil {
				c.logger.Error("reconcile assignments failed", "error", err)
			}
		}
	}
}

func (c *ClusterManager) WithdrawLeader(ctx context.Context, cooldown time.Duration) error {
	c.recampaignCooldown.Store(cooldown.Nanoseconds())
	return c.election.Withdraw()
}

func (c *ClusterManager) IsLeader(ctx context.Context) (bool, error) {
	leader, err := c.GetLeaderName(ctx)
	if err != nil {
		return false, err
	}
	return leader == c.clusteringConfig.InstanceName, nil
}

func (c *ClusterManager) snapshotMembers() map[string]*Member {
	c.mm.RLock()
	defer c.mm.RUnlock()
	members := make(map[string]*Member, len(c.members))
	maps.Copy(members, c.members)
	return members
}

func (c *ClusterManager) reconcileAssignments(ctx context.Context, members map[string]*Member) error {
	// 1. List all known targets
	targets, err := c.store.Keys("targets")
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		c.logger.Info("no targets, skipping reconciliation")
		return nil
	}
	// 2. get current assignments from locker
	// target -> holder
	currentAssignments, err := c.getAssignments(ctx)
	if err != nil {
		return err
	}
	c.logger.Debug("current assignments", "assignments", currentAssignments)

	membersLoad, err := c.getMembersLoad(ctx)
	if err != nil {
		return err
	}

	c.logger.Debug("reconcile assignments members with load", "members", members)
	quotas := calculateMembersQuota(members, int64(len(targets)))
	c.logger.Debug("reconcile assignments quotas", "quotas", quotas)

	// 3. Decide assignments
	assignments := make(map[string]*Member) // targetName -> member

	for _, tName := range targets {
		c.logger.Info("reconciling target", "target", tName)
		currentHolder, ok := currentAssignments[tName]
		if ok {
			if m, ok := members[currentHolder.ID]; ok {
				c.logger.Info("target already assigned to member", "target", tName, "member", m.ID)
				assignments[tName] = m
				continue
			} else {
				c.logger.Warn("target lock holder not found", "target", tName, "holder", currentHolder)
			}
		}
		assigned := pickAssignee(tName, members, quotas, membersLoad)
		if assigned == nil {
			c.logger.Warn("no assignee found for target", "target", tName)
			continue
		}
		c.logger.Info("assigning target", "target", tName, "assignee", assigned.ID)
		assignments[tName] = assigned
	}

	// 4. Publish assignments with assigner
	err = c.assigner.Assign(ctx, assignments)
	if err != nil {
		c.logger.Error("failed to push assignments", "count", len(assignments), "error", err)
	}

	// 5. Optionally verify active locks on assigned targets
	for tName, member := range assignments {
		c.asyncVerifyLock(ctx, tName, member.ID, time.Now().Add(5*time.Second))
	}

	return nil
}

func (c *ClusterManager) handleTargetCreate(ctx context.Context, target string, deniedMembers ...string) error {
	// 1. get current members with Load populated
	currentMembers, err := c.GetMembers(ctx)
	if err != nil {
		return err
	}
	c.logger.Debug("current members", "currentMembers", currentMembers)

	for _, m := range deniedMembers {
		delete(currentMembers, m)
	}
	return c.assignTarget(ctx, target, currentMembers)
}

// assignTarget assigns a target to the least loaded member.
// This is used when a new target is created or when a member is drained from its targets.
func (c *ClusterManager) assignTarget(ctx context.Context, target string, currentMembers map[string]*Member) error {
	// 2. find least loaded member
	leastLoadedMember := c.getLeastLoadedMember(currentMembers)
	if leastLoadedMember == nil {
		return fmt.Errorf("no least loaded member found")
	}
	// 3. assign target to member
	err := c.assigner.Assign(ctx, map[string]*Member{target: leastLoadedMember})
	if err != nil {
		c.logger.Error("failed to push assignment", "target", target, "error", err)
		return err
	}
	leastLoadedMember.Load++
	c.asyncVerifyLock(context.Background(), target, leastLoadedMember.ID, time.Now().Add(5*time.Second))
	return nil
}

func (c *ClusterManager) getAssignments(ctx context.Context) (map[string]*Member, error) {
	currentAssignments, err := c.locker.List(ctx, targetsLockPrefix(c.clusteringConfig.ClusterName))
	if err != nil {
		return nil, err
	}
	res := make(map[string]*Member)
	c.mm.RLock()
	defer c.mm.RUnlock()

	for tName, memberName := range currentAssignments {
		// normalize targetName
		targetName := path.Base(tName)
		if m, ok := c.members[memberName]; ok {
			res[targetName] = m
		} else {
			// TODO: unknwon member ?
			c.logger.Warn("found unknown member in current assignments", "member", memberName)
		}
	}

	return res, nil
}

func (c *ClusterManager) getAssignment(ctx context.Context, target string) *Member {
	member, ok := c.targetLockHolder(ctx, target)
	if !ok {
		return nil
	}

	return member
}

// GetMembers returns all members in the cluster.
// Populates the Load field with the number of locked targets.
func (c *ClusterManager) GetMembers(ctx context.Context) (map[string]*Member, error) {
	currentMembers := c.snapshotMembers()
	if len(currentMembers) == 0 {
		return nil, fmt.Errorf("no members found")
	}
	return c.populateMemberLoad(ctx, currentMembers)

}

func (c *ClusterManager) populateMemberLoad(ctx context.Context, members map[string]*Member) (map[string]*Member, error) {
	currentAssignments, err := c.locker.List(ctx, targetsLockPrefix(c.clusteringConfig.ClusterName))
	if err != nil {
		return nil, err
	}
	res := make(map[string]*Member)
	// seed res with current members
	for _, m := range members {
		res[m.ID] = &Member{
			ID:      m.ID,
			Address: m.Address,
			Labels:  m.Labels,
			Load:    0,
			Targets: nil,
		}
	}
	for tName, memberName := range currentAssignments {
		// normalize targetName
		targetName := path.Base(tName)
		if m, ok := members[memberName]; ok {
			if am, ok := res[memberName]; ok {
				am.Load++
				am.Targets = append(am.Targets, targetName)
			} else {
				am := &Member{
					ID:      m.ID,
					Address: m.Address,
					Labels:  m.Labels,
					Load:    1,
					Targets: []string{targetName},
				}
				res[memberName] = am
			}
		} else {
			c.logger.Warn("found unknown member in current assignments", "member", memberName)
		}
	}
	return res, nil
}

func (c *ClusterManager) getMembersLoad(ctx context.Context) (map[string]int64, error) {
	currentAssignments, err := c.locker.List(ctx, targetsLockPrefix(c.clusteringConfig.ClusterName))
	if err != nil {
		return nil, err
	}
	res := make(map[string]int64)

	for _, memberName := range currentAssignments {
		_, ok := res[memberName]
		if !ok {
			res[memberName] = 0
		}
		res[memberName]++
	}
	return res, nil
}

func (c *ClusterManager) getLeastLoadedMember(assignments map[string]*Member) *Member {
	var leastLoadedMember *Member
	for _, member := range assignments {
		if leastLoadedMember == nil {
			leastLoadedMember = member
			continue
		}
		if member.Load < leastLoadedMember.Load {
			leastLoadedMember = member
		}
	}
	return leastLoadedMember
}

func (c *ClusterManager) getMostLoadedMember(members map[string]*Member) *Member {
	var mostLoadedMember *Member
	for _, member := range members {
		if mostLoadedMember == nil {
			mostLoadedMember = member
		}
		if member.Load > mostLoadedMember.Load {
			mostLoadedMember = member
		}
	}
	return mostLoadedMember
}

func (c *ClusterManager) handleTargetDelete(ctx context.Context, target string) error {
	// find target assignment
	assignedTo := c.getAssignment(ctx, target)
	if assignedTo == nil {
		return fmt.Errorf("target is not assigned to any member")
	}
	err := c.assigner.Unassign(ctx, assignedTo, target)
	if err != nil {
		c.logger.Error("failed to unassign target", "target", target, "error", err)
		return err
	}
	// delete from other instances
	c.mm.RLock()
	members := make(map[string]*Member, len(c.members))
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

func (c *ClusterManager) deleteTargetFromMembers(ctx context.Context, target string, members map[string]*Member) error {
	for _, member := range members {
		address := getMemberAddress(member)
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

func (c *ClusterManager) targetLockHolder(ctx context.Context, target string) (*Member, bool) {
	holder, ok := holder(ctx, c.locker, targetLockKey(target, c.clusteringConfig.ClusterName))
	if !ok || holder == "" {
		return nil, false
	}
	c.mm.RLock()
	defer c.mm.RUnlock()
	member, ok := c.members[holder]
	if ok {
		return member, true
	}
	return nil, false
}

func (c *ClusterManager) GetInstanceToTargetsMapping(ctx context.Context) (map[string][]string, error) {
	locks, err := c.locker.List(ctx, targetsLockPrefix(c.clusteringConfig.ClusterName))
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

func (c *ClusterManager) DrainMember(ctx context.Context, toBeDrained string) error {
	members, err := c.GetMembers(ctx)
	if err != nil {
		return err
	}
	c.logger.Info("members", "members", members)
	memberToDrain, ok := members[toBeDrained]
	if !ok {
		return fmt.Errorf("member to drain not found")
	}

	if memberToDrain == nil {
		return fmt.Errorf("member to drain not found")
	}
	if len(memberToDrain.Targets) == 0 {
		c.logger.Info("member has no targets", "member", toBeDrained)
		return nil
	}
	c.logger.Info("draining member", "member", toBeDrained)
	c.logger.Info("unassigning targets", "targets", memberToDrain.Targets)
	err = c.assigner.Unassign(ctx, memberToDrain, memberToDrain.Targets...)
	if err != nil {
		c.logger.Error("failed to unassign targets", "member", toBeDrained, "error", err)
		return err
	}
	c.logger.Info("unassigned targets", "targets", memberToDrain.Targets)
	c.logger.Info("deleting member", "member", toBeDrained)
	delete(members, toBeDrained)
	for _, t := range memberToDrain.Targets {
		c.assignTarget(ctx, t, members)
	}
	return nil
}

func (c *ClusterManager) asyncVerifyLock(ctx context.Context, target, expectHolder string, deadline time.Time) {
	select {
	case c.lockCheckLimiter <- struct{}{}: // acquire semaphore
	case <-ctx.Done():
		return
	}
	go func() {
		defer func() {
			<-c.lockCheckLimiter // release semaphore
		}()
		key := targetLockKey(target, c.clusteringConfig.ClusterName)
		for {
			if ctx.Err() != nil || time.Now().After(deadline) {
				c.logger.Info("lock not observed before deadline",
					"target", target, "expect", expectHolder)
				return
			}
			holder, ok := holder(ctx, c.locker, key)
			if ok && holder == expectHolder {
				c.logger.Info("lock observed",
					"target", target, "holder", holder)
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(200 * time.Millisecond):
			}
		}
	}()
}
