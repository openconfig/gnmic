package event_strings

import (
	"regexp"
	"strings"

	"github.com/karimra/gnmic/formatters"
)

// TODO

// Strings provides some of Golang's strings functions to transform: tags, tag keys, values and value keys
type Strings struct {
	Tags   []string `mapstructure:"tags,omitempty"`
	Values []string `mapstructure:"values,omitempty"`

	TagKeys   []string `mapstructure:"tag_keys,omitempty"`
	ValueKeys []string `mapstructure:"value_keys,omitempty"`

	Transforms []map[string]*transform `mapstructure:"transforms,omitempty"`

	tags      []*regexp.Regexp
	values    []*regexp.Regexp
	tagKeys   []*regexp.Regexp
	valueKeys []*regexp.Regexp
}

type transform struct {
	op string
	// apply the transformation on key or value
	On string `mapstructure:"on,omitempty"`
	// Keep the old value or not if the key changed
	Keep bool `mapstructure:"keep,omitempty"`
	// string to be replaced
	Old string `mapstructure:"old,omitempty"`
	// replacement string of Old
	New string `mapstructure:"new,omitempty"`
	// Prefix to be trimmed
	Prefix string `mapstructure:"prefix,omitempty"`
	// Suffix to be trimmed
	Suffix string `mapstructure:"suffix,omitempty"`
}

func init() {
	formatters.Register("event_strings", func() formatters.EventProcessor {
		return &Strings{}
	})
}

func (s *Strings) Init(cfg interface{}) error {
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
	// init tag keys regex
	s.tagKeys = make([]*regexp.Regexp, 0, len(s.TagKeys))
	for _, reg := range s.TagKeys {
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
	s.valueKeys = make([]*regexp.Regexp, 0, len(s.ValueKeys))
	for _, reg := range s.ValueKeys {
		re, err := regexp.Compile(reg)
		if err != nil {
			return err
		}
		s.valueKeys = append(s.valueKeys, re)
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
				if vs, ok := v.(string); ok {
					s.applyValueTransformations(e, k, vs)
				}
			}
		}
		for _, re := range s.values {
			if vs, ok := v.(string); ok {
				if re.MatchString(vs) {
					s.applyValueTransformations(e, k, vs)
				}
			}
		}
	}
	for k, v := range e.Tags {
		for _, re := range s.tagKeys {
			if re.MatchString(k) {
				s.applyTagTransformations(e, k, v)
			}
		}
		for _, re := range s.tags {
			if re.MatchString(v) {
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
	}
	return k, v
}

func (t *transform) replace(k, v string) (string, string) {
	switch t.On {
	case "key":
		k = strings.ReplaceAll(k, t.Old, t.New)
	case "value":
		v = strings.ReplaceAll(v, t.Old, t.New)
	}
	return k, v
}

func (t *transform) trimPrefix(k, v string) (string, string) {
	switch t.On {
	case "key":
		k = strings.TrimPrefix(k, t.Prefix)
	case "value":
		v = strings.TrimPrefix(v, t.Prefix)
	}
	return k, v
}

func (t *transform) trimSuffix(k, v string) (string, string) {
	switch t.On {
	case "key":
		k = strings.TrimSuffix(k, t.Suffix)
	case "value":
		v = strings.TrimSuffix(v, t.Suffix)
	}
	return k, v
}

func (t *transform) toTitle(k, v string) (string, string) {
	switch t.On {
	case "key":
		k = strings.Title(k)
	case "value":
		v = strings.Title(v)
	}
	return k, v
}

func (t *transform) toLower(k, v string) (string, string) {
	switch t.On {
	case "key":
		k = strings.ToLower(k)
	case "value":
		v = strings.ToLower(v)
	}
	return k, v
}

func (t *transform) toUpper(k, v string) (string, string) {
	switch t.On {
	case "key":
		k = strings.ToUpper(k)
	case "value":
		v = strings.ToUpper(v)
	}
	return k, v
}
