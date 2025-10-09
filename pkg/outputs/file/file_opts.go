package file

import (
	"log"

	"github.com/openconfig/gnmic/pkg/formatters"
	gutils "github.com/openconfig/gnmic/pkg/utils"
)

func (f *File) setEventProcessors(logger *log.Logger) error {
	tcs, ps, acts, err := gutils.GetConfigMaps(f.store)
	if err != nil {
		return err
	}

	f.evps, err = formatters.MakeEventProcessors(
		logger,
		f.cfg.EventProcessors,
		ps,
		tcs,
		acts,
	)
	if err != nil {
		return err
	}
	return nil
}

func (f *File) setLogger(logger *log.Logger) {
	if logger != nil && f.logger != nil {
		f.logger.SetOutput(logger.Writer())
		f.logger.SetFlags(logger.Flags())
	}
}
