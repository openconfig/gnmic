package outputs

import "github.com/karimra/gnmiClient/outputs/file"

type Output interface {
	Write() error
	Close() error
}

// NewOutput //
func NewOutput(cfg interface{}) (Output, error) {
	switch cfg.(type) {
	case *file.Config:
		o, err := file.NewOutput(cfg.(*file.Config))
		if err != nil {
			return nil, err
		}
		return o, nil
	}
	return nil, nil
}
