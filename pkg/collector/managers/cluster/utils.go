package cluster_manager

import "strings"

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
