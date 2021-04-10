package gnmi_action

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

var errMalformedXPath = errors.New("malformed xpath")
var errMalformedXPathKey = errors.New("malformed xpath key")

var escapedBracketsReplacer = strings.NewReplacer(`\]`, `]`, `\[`, `[`)

// Get sends a gnmi.GetRequest to the target *t and returns a gnmi.GetResponse and an error
func (t *target) Get(ctx context.Context, req *gnmi.GetRequest) (*gnmi.GetResponse, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "username", *t.Config.Username, "password", *t.Config.Password)
	response, err := t.Client.Get(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed sending GetRequest to '%s': %v", t.Config.Address, err)
	}
	return response, nil
}

// Set sends a gnmi.SetRequest to the target *t and returns a gnmi.SetResponse and an error
func (t *target) Set(ctx context.Context, req *gnmi.SetRequest) (*gnmi.SetResponse, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "username", *t.Config.Username, "password", *t.Config.Password)
	response, err := t.Client.Set(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed sending SetRequest to '%s': %v", t.Config.Address, err)
	}
	return response, nil
}

// TargetConfig //
type targetConfig struct {
	Name          string        `mapstructure:"name,omitempty" json:"name,omitempty"`
	Address       string        `mapstructure:"address,omitempty" json:"address,omitempty"`
	Username      *string       `mapstructure:"username,omitempty" json:"username,omitempty"`
	Password      *string       `mapstructure:"password,omitempty" json:"password,omitempty"`
	Timeout       time.Duration `mapstructure:"timeout,omitempty" json:"timeout,omitempty"`
	Insecure      *bool         `mapstructure:"insecure,omitempty" json:"insecure,omitempty"`
	TLSCA         *string       `mapstructure:"tls-ca,omitempty" json:"tls-ca,omitempty"`
	TLSCert       *string       `mapstructure:"tls-cert,omitempty" json:"tls-cert,omitempty"`
	TLSKey        *string       `mapstructure:"tls-key,omitempty" json:"tls-key,omitempty"`
	SkipVerify    *bool         `mapstructure:"skip-verify,omitempty" json:"skip-verify,omitempty"`
	Subscriptions []string      `mapstructure:"subscriptions,omitempty" json:"subscriptions,omitempty"`
	Outputs       []string      `mapstructure:"outputs,omitempty" json:"outputs,omitempty"`
	BufferSize    uint          `mapstructure:"buffer-size,omitempty" json:"buffer-size,omitempty"`
	RetryTimer    time.Duration `mapstructure:"retry,omitempty" json:"retry-timer,omitempty"`
	TLSMinVersion string        `mapstructure:"tls-min-version,omitempty" json:"tls-min-version,omitempty"`
	TLSMaxVersion string        `mapstructure:"tls-max-version,omitempty" json:"tls-max-version,omitempty"`
	TLSVersion    string        `mapstructure:"tls-version,omitempty" json:"tls-version,omitempty"`
	ProtoFiles    []string      `mapstructure:"proto-files,omitempty" json:"proto-files,omitempty"`
	ProtoDirs     []string      `mapstructure:"proto-dirs,omitempty" json:"proto-dirs,omitempty"`
	Tags          []string      `mapstructure:"tags,omitempty" json:"tags,omitempty"`
	Gzip          *bool         `mapstructure:"gzip,omitempty" json:"gzip,omitempty"`
}

// Target represents a gNMI enabled box
type target struct {
	Config *targetConfig   `json:"config,omitempty"`
	Client gnmi.GNMIClient `json:"-"`
}

// NewTarget //
func newTarget(c *targetConfig) *target {
	t := &target{
		Config: c,
	}
	return t
}

// CreateGNMIClient //
func (t *target) createGNMIClient(ctx context.Context, opts ...grpc.DialOption) error {

	timeoutCtx, cancel := context.WithTimeout(ctx, t.Config.Timeout)
	defer cancel()
	conn, err := grpc.DialContext(timeoutCtx, t.Config.Address, opts...)
	if err != nil {
		return err
	}
	t.Client = gnmi.NewGNMIClient(conn)
	return nil
}

// ParsePath creates a gnmi.Path out of a p string, check if the first element is prefixed by an origin,
// removes it from the xpath and adds it to the returned gnmiPath
func parsePath(p string) (*gnmi.Path, error) {
	lp := len(p)
	if lp == 0 {
		return &gnmi.Path{}, nil
	}
	var origin string

	idx := strings.Index(p, ":")
	if idx >= 0 && p[0] != '/' && !strings.Contains(p[:idx], "/") &&
		// path == origin:/ || path == origin:
		((idx+1 < lp && p[idx+1] == '/') || (lp == idx+1)) {
		origin = p[:idx]
		p = p[idx+1:]
	}

	pes, err := toPathElems(p)
	if err != nil {
		return nil, err
	}
	return &gnmi.Path{
		Origin: origin,
		Elem:   pes,
	}, nil
}

// toPathElems parses a xpath and returns a list of path elements
func toPathElems(p string) ([]*gnmi.PathElem, error) {
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	buffer := make([]rune, 0)
	null := rune(0)
	prevC := rune(0)
	// track if the loop is traversing a key
	inKey := false
	for _, r := range p {
		switch r {
		case '[':
			if inKey && prevC != '\\' {
				return nil, errMalformedXPath
			}
			if prevC != '\\' {
				inKey = true
			}
		case ']':
			if !inKey && prevC != '\\' {
				return nil, errMalformedXPath
			}
			if prevC != '\\' {
				inKey = false
			}
		case '/':
			if !inKey {
				buffer = append(buffer, null)
				prevC = r
				continue
			}
		}
		buffer = append(buffer, r)
		prevC = r
	}
	if inKey {
		return nil, errMalformedXPath
	}
	stringElems := strings.Split(string(buffer), string(null))
	pElems := make([]*gnmi.PathElem, 0, len(stringElems))
	for _, s := range stringElems {
		if s == "" {
			continue
		}
		pe, err := toPathElem(s)
		if err != nil {
			return nil, err
		}
		pElems = append(pElems, pe)
	}
	return pElems, nil
}

// toPathElem take a xpath formatted path element such as "elem1[k=v]" and returns the corresponding gnmi.PathElem
func toPathElem(s string) (*gnmi.PathElem, error) {
	idx := -1
	prevC := rune(0)
	for i, r := range s {
		if r == '[' && prevC != '\\' {
			idx = i
			break
		}
		prevC = r
	}
	var kvs map[string]string
	if idx > 0 {
		var err error
		kvs, err = parseXPathKeys(s[idx:])
		if err != nil {
			return nil, err
		}
		s = s[:idx]
	}
	return &gnmi.PathElem{Name: s, Key: kvs}, nil
}

// parseXPathKeys takes keys definition from an xpath, e.g [k1=v1][k2=v2] and return the keys and values as a map[string]string
func parseXPathKeys(s string) (map[string]string, error) {
	if len(s) == 0 {
		return nil, nil
	}
	kvs := make(map[string]string)
	inKey := false
	start := 0
	prevRune := rune(0)
	for i, r := range s {
		switch r {
		case '[':
			if prevRune == '\\' {
				prevRune = r
				continue
			}
			if inKey {
				return nil, errMalformedXPathKey
			}
			inKey = true
			start = i + 1
		case ']':
			if prevRune == '\\' {
				prevRune = r
				continue
			}
			if !inKey {
				return nil, errMalformedXPathKey
			}
			eq := strings.Index(s[start:i], "=")
			if eq < 0 {
				return nil, errMalformedXPathKey
			}
			k, v := s[start:i][:eq], s[start:i][eq+1:]
			if len(k) == 0 || len(v) == 0 {
				return nil, errMalformedXPathKey
			}
			kvs[escapedBracketsReplacer.Replace(k)] = escapedBracketsReplacer.Replace(v)
			inKey = false
		}
		prevRune = r
	}
	if inKey {
		return nil, errMalformedXPathKey
	}
	return kvs, nil
}
