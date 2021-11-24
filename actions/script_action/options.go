package script_action

import (
	"log"
	"os"

	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
)

func (s *scriptAction) WithTargets(map[string]*types.TargetConfig) {}

func (s *scriptAction) WithLogger(logger *log.Logger) {
	if s.Debug && logger != nil {
		s.logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
	} else if s.Debug {
		s.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}
