package app

// Shutdown stops application work and releases leased resources.
// Calls are serialized and idempotent.
func (a *App) Shutdown() error {
	a.shutdownOnce.Do(func() {
		a.CleanupPlugins()
		if a.Cfn != nil {
			a.Cfn()
		}
		if a.locker != nil {
			a.shutdownErr = a.locker.Stop()
		}
	})
	return a.shutdownErr
}
