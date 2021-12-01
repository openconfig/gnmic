package gnmi_action

import (
	"log"
	"os"

	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
)

func (g *gnmiAction) WithTargets(tcs map[string]*types.TargetConfig) {
	if tcs == nil {
		return
	}
	g.targetsConfigs = tcs
}

func (g *gnmiAction) WithLogger(logger *log.Logger) {
	if g.Debug && logger != nil {
		g.logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
	} else if g.Debug {
		g.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}
