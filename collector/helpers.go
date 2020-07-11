package collector

import (
	"github.com/google/gnxi/utils/xpath"
	"github.com/openconfig/gnmi/proto/gnmi"
)

func CreatePrefix(prefix, target string) (*gnmi.Path, error) {
	if len(prefix)+len(target) == 0 {
		return nil, nil
	}
	p := new(gnmi.Path)
	var err error
	p, err = xpath.ToGNMIPath(prefix)
	if err != nil {
		return nil, err
	}
	if target != "" {
		p.Target = target
	}
	return p, nil
}
