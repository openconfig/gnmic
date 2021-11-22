package template_action

import (
	"log"
	"os"

	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
)

func (t *templateAction) WithTargets(map[string]*types.TargetConfig) {}

func (t *templateAction) WithLogger(logger *log.Logger) {
	if t.Debug && logger != nil {
		t.logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
	} else if t.Debug {
		t.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}
