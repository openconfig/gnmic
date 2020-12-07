package event_strings

import (
	"io/ioutil"
	"log"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/karimra/gnmic/formatters"
)

// TODO

// Strings provides some of Golang's strings functions to transform: tags, tag names, values and value names
type Strings struct {
	Tags       []string                `mapstructure:"tags,omitempty"`
	Values     []string                `mapstructure:"values,omitempty"`
	TagNames   []string                `mapstructure:"tag_names,omitempty"`
	ValueNames []string                `mapstructure:"value_names,omitempty"`
	Debug      bool                    `mapstructure:"debug,omitempty"`
	Transforms []map[string]*transform `mapstructure:"transforms,omitempty"`

	tags      []*regexp.Regexp
	values    []*regexp.Regexp
	tagKeys   []*regexp.Regexp
	valueKeys []*regexp.Regexp

	logger *log.Logger
}

type transform struct {
	op string
	// apply the transformation on name or value
	On string `mapstructure:"on,omitempty"`
	// Keep the old value or not if the name changed
	Keep bool `mapstructure:"keep,omitempty"`
	// string to be replaced
	Old string `mapstructure:"old,omitempty"`
	// replacement string of Old
	New string `mapstructure:"new,omitempty"`
	// Prefix to be trimmed
	Prefix string `mapstructure:"prefix,omitempty"`
	// Suffix to be trimmed
	Suffix string `mapstructure:"suffix,omitempty"`
	// charachter to split on
	SplitOn string `mapstructure:"split_on,omitempty"`
	// charachter to join with
	JoinWith string `mapstructure:"join_with,omitempty"`
	// number of first items to ignore when joining
	IgnoreFirst int `mapstructure:"ignore_first,omitempty"`
	// number of last items to ignore when joining
	IgnoreLast int `mapstructure:"ignore_last,omitempty"`
}

func init() {
	formatters.Register("event_strings", func() formatters.EventProcessor {
		return &Strings{}
	})
}

func (s *Strings) Init(cfg interface{}, logger *log.Logger) error {
	err := formatters.DecodeConfig(cfg, s)
	if err != nil {
		return err
	}
	for i := range s.Transforms {
		for k := range s.Transforms[i] {
			s.Transforms[i][k].op = k
		}
	}
	// init tags regex
	s.tags = make([]*regexp.Regexp, 0, len(s.Tags))
	for _, reg := range s.Tags {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		s.tags = append(s.tags, re)
	}
	// init tag names regex
	s.tagKeys = make([]*regexp.Regexp, 0, len(s.TagNames))
	for _, reg := range s.TagNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		s.tagKeys = append(s.tagKeys, re)
	}
	// init values regex
	s.values = make([]*regexp.Regexp, 0, len(s.Values))
	for _, reg := range s.Values {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		s.values = append(s.values, re)
	}
	// init value Keys regex
	s.valueKeys = make([]*regexp.Regexp, 0, len(s.ValueNames))
	for _, reg := range s.ValueNames {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		s.valueKeys = append(s.valueKeys, re)
	}
	if s.Debug {
		s.logger = log.New(logger.Writer(), "event_strings ", logger.Flags())
	} else {
		s.logger = log.New(ioutil.Discard, "", 0)
	}
	return nil
}

func (s *Strings) Apply(e *formatters.EventMsg) {
	if e == nil {
		return
	}
	for k, v := range e.Values {
		for _, re := range s.valueKeys {
			if re.MatchString(k) {
				s.logger.Printf("value name '%s' matched regex '%s'", k, re.String())
				if vs, ok := v.(string); ok {
					s.applyValueTransformations(e, k, vs)
				}
			}
		}
		for _, re := range s.values {
			if vs, ok := v.(string); ok {
				if re.MatchString(vs) {
					s.logger.Printf("value '%s' matched regex '%s'", vs, re.String())
					s.applyValueTransformations(e, k, vs)
				}
			}
		}
	}
	for k, v := range e.Tags {
		for _, re := range s.tagKeys {
			if re.MatchString(k) {
				s.logger.Printf("tag name '%s' matched regex '%s'", k, re.String())
				s.applyTagTransformations(e, k, v)
			}
		}
		for _, re := range s.tags {
			if re.MatchString(v) {
				s.logger.Printf("tag '%s' matched regex '%s'", k, re.String())
				s.applyTagTransformations(e, k, v)
			}
		}
	}
}

func (s *Strings) applyValueTransformations(e *formatters.EventMsg, k, v string) {
	for _, trans := range s.Transforms {
		for _, t := range trans {
			if !t.Keep {
				delete(e.Values, k)
			}
			k, v = t.apply(k, v)
			e.Values[k] = v
		}
	}
}

func (s *Strings) applyTagTransformations(e *formatters.EventMsg, k, v string) {
	for _, trans := range s.Transforms {
		for _, t := range trans {
			if !t.Keep {
				delete(e.Tags, k)
			}
			k, v = t.apply(k, v)
			e.Tags[k] = v
		}
	}
}

func (t *transform) apply(k, v string) (string, string) {
	switch t.op {
	case "replace":
		return t.replace(k, v)
	case "trim_prefix":
		return t.trimPrefix(k, v)
	case "trim_suffix":
		return t.trimSuffix(k, v)
	case "title":
		return t.toTitle(k, v)
	case "to_lower":
		return t.toLower(k, v)
	case "to_upper":
		return t.toUpper(k, v)
	case "split":
		return t.split(k, v)
	case "path_base":
		return t.pathBase(k, v)
	}
	return k, v
}

func (t *transform) replace(k, v string) (string, string) {
	switch t.On {
	case "name":
		k = strings.ReplaceAll(k, t.Old, t.New)
	case "value":
		v = strings.ReplaceAll(v, t.Old, t.New)
	}
	return k, v
}

func (t *transform) trimPrefix(k, v string) (string, string) {
	switch t.On {
	case "name":
		k = strings.TrimPrefix(k, t.Prefix)
	case "value":
		v = strings.TrimPrefix(v, t.Prefix)
	}
	return k, v
}

func (t *transform) trimSuffix(k, v string) (string, string) {
	switch t.On {
	case "name":
		k = strings.TrimSuffix(k, t.Suffix)
	case "value":
		v = strings.TrimSuffix(v, t.Suffix)
	}
	return k, v
}

func (t *transform) toTitle(k, v string) (string, string) {
	switch t.On {
	case "name":
		k = strings.Title(k)
	case "value":
		v = strings.Title(v)
	}
	return k, v
}

func (t *transform) toLower(k, v string) (string, string) {
	switch t.On {
	case "name":
		k = strings.ToLower(k)
	case "value":
		v = strings.ToLower(v)
	}
	return k, v
}

func (t *transform) toUpper(k, v string) (string, string) {
	switch t.On {
	case "name":
		k = strings.ToUpper(k)
	case "value":
		v = strings.ToUpper(v)
	}
	return k, v
}

func (t *transform) split(k, v string) (string, string) {
	switch t.On {
	case "name":
		items := strings.Split(k, t.SplitOn)
		numItems := len(items)
		if numItems <= t.IgnoreFirst || numItems <= t.IgnoreLast || t.IgnoreFirst >= numItems-t.IgnoreLast {
			return "", v
		}
		k = strings.Join(items[t.IgnoreFirst:numItems-t.IgnoreLast], t.JoinWith)
	case "value":
		items := strings.Split(v, t.SplitOn)
		numItems := len(items)
		if numItems <= t.IgnoreFirst || numItems <= t.IgnoreLast || t.IgnoreFirst >= numItems-t.IgnoreLast {
			return k, ""
		}
		v = strings.Join(items[t.IgnoreFirst:numItems-t.IgnoreLast], t.JoinWith)
	}
	return k, v
}

func (t *transform) pathBase(k, v string) (string, string) {
	switch t.On {
	case "name":
		k = filepath.Base(k)
	case "value":
		v = filepath.Base(v)
	}
	return k, v
}
