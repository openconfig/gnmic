package app

import "fmt"

func (a *App) targetLockKey(s string) string {
	if a.Config.Clustering == nil {
		return s
	}
	if s == "" {
		return s
	}
	return fmt.Sprintf("gnmic/%s/targets/%s", a.Config.Clustering.ClusterName, s)
}
