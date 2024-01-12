package app

import "github.com/openconfig/gnmic/pkg/formatters/plugin_manager"

func (a *App) initPluginManager() error {
	pc, err := a.Config.GetPluginsConfig()
	if err != nil {
		return err
	}
	if pc == nil {
		return nil
	}
	a.pm = plugin_manager.New(pc, a.Logger.Writer())
	return a.pm.Load()
}

func (a *App) CleanupPlugins() {
	if a.pm == nil {
		return
	}
	a.pm.Cleanup()
}
