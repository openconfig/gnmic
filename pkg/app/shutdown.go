package app

// Shutdown stops application work and releases resources that must not be left
// to lease expiry. It is safe to call more than once.
func (a *App) Shutdown() error {
	a.shutdownOnce.Do(func() {
		if a.Cfn != nil {
			a.Cfn()
		}
		if a.locker != nil {
			a.shutdownErr = a.locker.Stop()
		}
		a.CleanupPlugins()
	})
	return a.shutdownErr
}
