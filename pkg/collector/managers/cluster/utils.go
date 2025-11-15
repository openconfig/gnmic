package cluster_manager

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

func targetsLockPrefix(clusterName string) string {
	return fmt.Sprintf("gnmic/%s/targets", clusterName)
}

func targetLockKey(target, clusterName string) string {
	return fmt.Sprintf("gnmic/%s/targets/%s", clusterName, target)
}

func GetAPIScheme(member *Member) string {
	if member == nil {
		return httpScheme
	}
	for _, lb := range member.Labels {
		parts := strings.SplitN(lb, "=", 2)
		if len(parts) == 2 && parts[0] == protocolLabel {
			if parts[1] == "https" {
				return httpsScheme
			} else {
				return httpScheme
			}
		}
	}
	return httpScheme
}

func getMemberAddress(member *Member) string {
	return GetAPIScheme(member) + "://" + member.Address
}

func jittered(d time.Duration) time.Duration {
	j := time.Duration(rand.Int63n(int64(float64(d) * recampaignJitterRatio)))
	return d + j
}
