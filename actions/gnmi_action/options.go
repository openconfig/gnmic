package gnmi_action

import (
	"log"
	"os"

	"github.com/karimra/gnmic/collector"
)

func (g *gnmiAction) WithTargets(tcs map[string]interface{}) {
	for n, tc := range tcs {
		switch tc := tc.(type) {
		case *collector.TargetConfig:
			g.targetsConfig[n] = tc
		}
	}
}

func (g *gnmiAction) WithLogger(logger *log.Logger) {
	if g.Debug && logger != nil {
		g.logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
	} else if g.Debug {
		g.logger = log.New(os.Stderr, loggingPrefix, log.LstdFlags|log.Lmicroseconds)
	}
}
