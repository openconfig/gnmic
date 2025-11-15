package cluster_manager

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"time"
)

func (c *ClusterManager) RebalanceTargets(ctx context.Context) error {
	members, err := c.GetMembers(ctx)
	if err != nil {
		return err
	}
	if len(members) < 2 {
		return fmt.Errorf("no members or only one member found")
	}
	c.logger.Debug("members", "members", members)
	// get most loaded and least loaded
	mostLoadedMember := c.getMostLoadedMember(members)
	if mostLoadedMember == nil {
		return fmt.Errorf("count not determine most loaded member")
	}
	leastLoadedMember := c.getLeastLoadedMember(members)
	if leastLoadedMember == nil {
		return fmt.Errorf("count not determine least loaded member")
	}

	c.logger.Debug("mostLoadedMember", "mostLoadedMember", mostLoadedMember)
	c.logger.Debug("leastLoadedMember", "leastLoadedMember", leastLoadedMember)
	// decide if rebalancing is needed
	// if not, return
	diff := mostLoadedMember.Load - leastLoadedMember.Load
	if diff < 2 {
		c.logger.Info("rebalancing is not needed",
			"mostLoadedMember", mostLoadedMember.ID,
			"mostLoadedMemberLoad", mostLoadedMember.Load,
			"leastLoadedMember", leastLoadedMember.ID,
			"leastLoadedMemberLoad", leastLoadedMember.Load,
		)
		return nil
	}
	c.logger.Info("rebalancing is needed",
		"mostLoadedMember", mostLoadedMember.ID,
		"mostLoadedMemberLoad", mostLoadedMember.Load,
		"leastLoadedMember", leastLoadedMember.ID,
		"leastLoadedMemberLoad", leastLoadedMember.Load,
	)
	// determine the set of targets to move
	moveCount := diff / 2 // TODO: add cap
	moveCount = max(moveCount, leastLoadedMember.Load)
	candidates := append([]string{}, mostLoadedMember.Targets...)
	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})
	targetsToMove := candidates[:moveCount]
	assignments := make(map[string]*Member)
	// unassign the target from the most loaded member
	for _, t := range targetsToMove {
		c.logger.Info("unassigning target", "target", t, "member", mostLoadedMember.ID)
		err = c.assigner.Unassign(ctx, mostLoadedMember, t)
		if err != nil {
			c.logger.Error("failed to unassign target", "target", t, "member", mostLoadedMember.ID, "error", err)
			continue
		}
		assignments[t] = leastLoadedMember
	}
	c.logger.Info("assignment set", "assignments", assignments, "member", leastLoadedMember.ID)
	err = c.assigner.Assign(ctx, assignments)
	if err != nil {
		return err
	}
	for _, t := range targetsToMove {
		c.asyncVerifyLock(ctx, t, leastLoadedMember.ID, time.Now().Add(5*time.Second))
	}
	return nil
}

func (c *ClusterManager) RebalanceTargetsV2() error {
	if ok := c.rebalancingSem.TryAcquire(1); !ok {
		return fmt.Errorf("rebalancing already in progress")
	}
	go func() {
		defer c.rebalancingSem.Release(1)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second) // TODO: configurable
		defer cancel()
		members, err := c.GetMembers(ctx)
		if err != nil {
			c.logger.Error("failed to get members", "error", err)
		}
		rebalancePlan := rebalance(members)
		c.logger.Info("rebalance plan", "rebalancePlan", rebalancePlan)
		if rebalancePlan == nil {
			c.logger.Info("cluster is already balanced")
			return
		}

		if len(rebalancePlan) == 0 {
			c.logger.Info("cluster is already balanced")
			return
		}
		// per member deltas
		removeBySrc := map[string][]string{}
		addByDst := map[string][]string{}
		// keep target -> current member mapping
		owner := make(map[string]string)
		for id, m := range members {
			for _, t := range m.Targets {
				owner[t] = id
			}
		}
		for t, dst := range rebalancePlan {
			if srcID, ok := owner[t]; ok && srcID != dst.ID {
				removeBySrc[srcID] = append(removeBySrc[srcID], t)
				addByDst[dst.ID] = append(addByDst[dst.ID], t)
			}
		}
		c.logger.Info("removing targets", "removeBySrc", removeBySrc)
		for srcID, ts := range removeBySrc {
			err = c.assigner.Unassign(ctx, members[srcID], ts...)
			if err != nil {
				c.logger.Error("failed to unassign targets", "targets", ts, "member", srcID, "error", err)
				continue
			}
		}
		c.logger.Info("adding targets", "addByDst", addByDst)
		for dstID, ts := range addByDst {
			asg := make(map[string]*Member, len(ts))
			for _, t := range ts {
				asg[t] = members[dstID]
			}
			err = c.assigner.Assign(ctx, asg)
			if err != nil {
				c.logger.Error("assign batch failed", "member", dstID, "err", err)
			}
		}
	}()
	return nil
}

// rebalance computes a one-shot plan: target -> newOwner (only moved ones).
// It never proposes moving the same target twice, and tries to fill each receiver to its quota.
func rebalance(members map[string]*Member) map[string]*Member {
	// calculate total load
	total := int64(0)
	for _, m := range members {
		total += m.Load
	}
	// determine the quota for each member
	q := calculateMembersQuota(members, total)
	if len(q) == 0 {
		return nil // already balanced
	}

	donors := make([]*donor, 0)
	receivers := make([]*receiver, 0)

	// deterministic member order
	ids := make([]string, 0, len(members))
	for id := range members {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	// determine "want" and "have" for each member
	for _, id := range ids {
		m := members[id]
		want := q[id]
		have := m.Load
		switch {
		case have > want:
			// copy targets & randomize to avoid bias; or choose oldest/cheapest to move
			pool := append([]string(nil), m.Targets...)
			rand.Shuffle(len(pool), func(i, j int) { pool[i], pool[j] = pool[j], pool[i] })
			donors = append(donors, &donor{
				id: id,
				// m:       m,
				surplus: have - want,
				pool:    pool,
			})
		case have < want:
			receivers = append(receivers, &receiver{
				id: id,
				// m: m,
				need: want - have,
			})
		}
	}

	if len(donors) == 0 || len(receivers) == 0 {
		return nil // already balanced
	}

	moves := make(map[string]*Member)
	// determine the best targets to move from each donor
	for _, d := range donors {
		for d.surplus > 0 && len(receivers) > 0 {
			r := receivers[0]
			if r.need == 0 { // receiver needs no more
				receivers = receivers[1:]
				continue
			}
			// find up to k targets eligible for r
			k := min(d.surplus, r.need)
			taken := int64(0)
			// scan donor pool, pick targets, compact pool as we consume
			w := 0
			for _, t := range d.pool {
				if taken < k {
					moves[t] = members[r.id] // move target to receiver
					taken++
					// skip copying this one (removed)
					continue
				}
				// keep in pool
				d.pool[w] = t
				w++
			}
			d.pool = d.pool[:w] // compact pool
			d.surplus -= taken  // update surplus
			r.need -= taken     // update need

			// if this receiver still needs more, keep it at index 0, otherwise pop it
			if r.need == 0 {
				receivers = receivers[1:]
			} else {
				// rotate receiver to the back to give others a chance (optional)
				receivers = append(receivers[1:], r)
			}

			// If donor ran out of eligible candidates before satisfying k, break to next receiver
			if taken == 0 {
				// no eligible targets for this receiver, try with next receiver
				receivers = append(receivers[1:], r) // rotate
				if len(receivers) == 1 {
					// only one receiver left, but no eligible targets => donor is stuck
					break
				}
			}
		}
	}
	return moves
}

// calculateMembersQuota calculates the quota for each member based on the total load
// the quota is the average load per member
// the remainder is distributed evenly among the members
func calculateMembersQuota(members map[string]*Member, total int64) map[string]int64 {
	if total == 0 {
		// no load, no quota
		res := make(map[string]int64, len(members))
		for id := range members {
			res[id] = 0
		}
		return res
	}
	if len(members) == 0 {
		return nil
	}
	n := int64(len(members))
	base := total / n
	rem := total % n
	ids := make([]string, 0, n)
	for id := range members {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	quota := make(map[string]int64)
	for i, id := range ids {
		if i < int(rem) {
			quota[id] = base + 1
		} else {
			quota[id] = base
		}
	}
	return quota
}

type donor struct {
	// member id
	id string
	// surplus load
	surplus int64
	// copy of targets
	pool []string
}

type receiver struct {
	// member id
	id string
	// need load
	need int64
}
