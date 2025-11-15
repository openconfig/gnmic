package cluster_manager

import (
	"hash/fnv"
)

// pickAssignee selects the best Member to assign a target to, based on current quota and load.
// It chooses the member with the most available quota (quota - load), and uses tieBreak for deterministic selection when tied.
// Updates the membersLoad map to reflect the new assignment and returns the selected Member.
func pickAssignee(
	targetName string,
	members map[string]*Member,
	quotas map[string]int64,
	membersLoad map[string]int64,
) *Member {
	if len(members) == 0 {
		return nil
	}
	var pick *Member
	highestFreeQuota := int64(-1 << 62)
	for _, m := range members {
		s := quotas[m.ID] - membersLoad[m.ID]
		if s > highestFreeQuota || (s == highestFreeQuota && tieBreak(targetName, m.ID, pick.ID)) {
			pick = m
			highestFreeQuota = s
		}
	}
	membersLoad[pick.ID]++
	return pick
}

// tieBreak deterministically chooses between two members for assignment by hashing the combination of targetName and memberID using FNV-1a.
// If the hashes are equal, it resorts to lexicographical comparison of the member IDs.
func tieBreak(targetName, memberA, memberB string) bool {

	if memberB == "" {
		return true
	}
	ha := fnv64(targetName + memberA)
	hb := fnv64(targetName + memberB)
	if ha == hb {
		return memberA < memberB
	}
	return ha < hb
}

// fnv64 computes the FNV-1a 64-bit hash for a given string.
func fnv64(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}
