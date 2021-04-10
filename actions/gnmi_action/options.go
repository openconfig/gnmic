package gnmi_action

import (
	"log"
	"os"

	"github.com/karimra/gnmic/actions"
)

func (g *gnmiAction) WithTargets(tcs map[string]interface{}) {
	var err error
	for n, tc := range tcs {
		ltc := new(targetConfig)
		err = actions.DecodeConfig(tc, ltc)
		if err != nil {
			g.logger.Printf("failed to decode targets config: %v", err)
		}
		g.targetsConfigs[n] = ltc
	}
}

func (g *gnmiAction) WithLogger(logger *log.Logger) {
	if g.Debug && logger != nil {
		g.logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
	} else if g.Debug {
		g.logger = log.New(os.Stderr, loggingPrefix, log.LstdFlags|log.Lmicroseconds)
	}
}
