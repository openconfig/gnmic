package collector

import (
	"strings"

	"github.com/google/gnxi/utils/xpath"
	"github.com/openconfig/gnmi/proto/gnmi"
)

// ParsePath creates a gnmi.Path out of a p string, check if the first element is prefixed by an origin,
// removes it from the xpath and adds it to the returned gnmiPath
func ParsePath(p string) (*gnmi.Path, error) {
	var origin string
	elems := strings.Split(p, "/")
	if len(elems) > 0 {
		f := strings.Split(elems[0], ":")
		if len(f) > 1 {
			origin = f[0]
			elems[0] = strings.Join(f[1:], ":")
		}
	}
	gnmiPath, err := xpath.ToGNMIPath(strings.Join(elems, "/"))
	if err != nil {
		return nil, err
	}
	gnmiPath.Origin = origin
	return gnmiPath, nil
}
