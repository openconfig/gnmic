// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_starlark

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"go.starlark.net/starlark"

	"github.com/openconfig/gnmic/pkg/formatters"
)

type event struct {
	ev     *formatters.EventMsg
	frozen bool
}

func fromEvent(ev *formatters.EventMsg) *event {
	return &event{
		ev: ev,
	}
}

func toEvent(sev *event) *formatters.EventMsg {
	if sev == nil {
		return nil
	}
	return sev.ev
}

// *event implements starlark.Value
func (s *event) String() string {
	b, _ := json.Marshal(s.ev)
	return string(b)
}

// *event implements starlark.Value
func (s *event) Type() string { return "Event" }

// *event implements starlark.Value
func (s *event) Freeze() { s.frozen = true }

// *event implements starlark.Value
func (s *event) Truth() starlark.Bool { return starlark.True }

// *event implements starlark.Value
func (s *event) Hash() (uint32, error) { return 0, errors.New("not hashable") }

// *event implements the starlark.HasAttrs interface.
func (s *event) AttrNames() []string {
	return []string{"name", "timestamp", "tags", "values", "deletes"}
}

// *event implements the starlark.HasAttrs interface.
func (s *event) Attr(name string) (starlark.Value, error) {
	switch name {
	case "name":
		return starlark.String(s.ev.Name), nil
	case "timestamp":
		return starlark.MakeInt64(s.ev.Timestamp), nil
	case "tags":
		return s.Tags(), nil
	case "values":
		return s.Values(), nil
	case "deletes":
		return s.Deletes(), nil
	default:
		// Returning nil, nil indicates "no such field or method"
		return nil, nil
	}
}

// *event implements the starlark.HasSetField interface.
func (s *event) SetField(name string, value starlark.Value) error {
	if s.frozen {
		return fmt.Errorf("cannot modify frozen event struct")
	}

	switch name {
	case "name":
		return s.SetName(value)
	case "timestamp":
		return s.SetTimestamp(value)
	case "tags":
		return s.SetTags(value)
	case "values":
		return s.SetValues(value)
	case "deletes":
		return s.SetDeletes(value)
	default:
		return starlark.NoSuchAttrError(
			fmt.Sprintf("cannot assign to field %q", name))
	}
}

func (s *event) SetName(name starlark.Value) error {
	if name, ok := name.(starlark.String); ok {
		s.ev.Name = name.GoString()
		return nil
	}
	return fmt.Errorf("name not a string, %T", name)
}

func (s *event) Tags() starlark.Value {
	return newDict("Tags", s.ev.Tags)
}

func (s *event) Values() starlark.Value {
	return newDict("Values", s.ev.Values)
}

func (s *event) Deletes() starlark.Value {
	if len(s.ev.Deletes) == 0 {
		return &starlark.List{}
	}
	result := &starlark.List{}
	for _, s := range s.ev.Deletes {
		v, _ := toStarlarkValue(s)
		result.Append(v)
	}
	return result
}

func (s *event) Timestamp() starlark.Int {
	return starlark.MakeInt64(s.ev.Timestamp)
}

func (s *event) SetTimestamp(value starlark.Value) error {
	switch v := value.(type) {
	case starlark.Int:
		ns, ok := v.Int64()
		if !ok {
			return errors.New("type error: expected int64 timestamp")
		}
		s.ev.Timestamp = ns
		return nil
	default:
		return fmt.Errorf("type error: got %T", v)
	}
}

func (s *event) SetTags(value starlark.Value) error {
	tags, err := toTags(value)
	if err != nil {
		return err
	}
	s.ev.Tags = tags
	return nil
}

func (s *event) SetValues(value starlark.Value) error {
	vals, err := toValues(value)
	if err != nil {
		return err
	}
	s.ev.Values = vals
	return nil
}

func (s *event) SetDeletes(value starlark.Value) error {
	dels, err := toDeletes(value)
	if err != nil {
		return err
	}
	s.ev.Deletes = dels
	return nil
}

func newEvent(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var name starlark.String
	var ts starlark.Int
	var tags starlark.Value
	var values starlark.Value
	var deletes starlark.Value

	err := starlark.UnpackArgs("Event", args, kwargs,
		"name", &name,
		"timestamp?", &ts,
		"tags?", &tags,
		"values?", &values,
		"deletes?", &deletes,
	)

	if err != nil {
		return nil, err
	}
	vs, err := toValues(values)
	if err != nil {
		return nil, err
	}
	tgs, err := toTags(tags)
	if err != nil {
		return nil, err
	}
	dels, err := toDeletes(deletes)
	if err != nil {
		return nil, err
	}
	timestamp, ok := ts.Int64()
	if !ok {
		return nil, fmt.Errorf("failed to represent %v as int64", ts)
	}
	ev := &formatters.EventMsg{
		Name:      string(name),
		Timestamp: timestamp,
		Tags:      tgs,
		Values:    vs,
		Deletes:   dels,
	}
	return &event{
		ev: ev,
	}, nil
}

func toValues(value starlark.Value) (map[string]any, error) {
	if value == nil {
		return make(map[string]any), nil
	}
	if value, ok := value.(starlark.IterableMapping); ok {
		result := make(map[string]any)
		var err error
		for _, item := range value.Items() {
			k, ok := item[0].(starlark.String)
			if !ok {
				return nil, fmt.Errorf("failed to represent value name %v as string", item[0])
			}
			result[k.GoString()], err = toGoVal(item[1])
			if err != nil {
				return nil, err
			}
		}
		return result, nil
	}
	return nil, errors.New("unexpected iterable type in values field")
}

func toTags(value starlark.Value) (map[string]string, error) {
	if value == nil {
		return make(map[string]string), nil
	}
	if value, ok := value.(starlark.IterableMapping); ok {
		result := make(map[string]string)
		for _, item := range value.Items() {
			k, ok := item[0].(starlark.String)
			if !ok {
				return nil, fmt.Errorf("failed to represent value name %v as string", item[0])
			}
			v, ok := item[1].(starlark.String)
			if !ok {
				return nil, fmt.Errorf("failed to represent value name %v as string", item[1])
			}
			result[k.GoString()] = v.GoString()
		}
		return result, nil
	}
	return nil, errors.New("unexpected iterable type in tags field")
}

func toDeletes(value starlark.Value) ([]string, error) {
	if value == nil {
		return []string{}, nil
	}
	if value, ok := value.(starlark.Sequence); ok {
		iter := value.Iterate()
		defer iter.Done()
		result := make([]string, 0, value.Len())
		for {
			var item starlark.Value
			if iter.Next(&item) {
				if s, ok := item.(starlark.String); ok {
					result = append(result, s.GoString())
					continue
				}
				return nil, errors.New("sequence item is not a 'string")
			}
			break
		}
		return result, nil
	}
	return nil, errors.New("unexpected iterable type in deletes field")
}

// toStarlarkValue converts a value to a starlark.Value.
func toStarlarkValue(value any) (starlark.Value, error) {
	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Slice:
		length := v.Len()
		array := make([]starlark.Value, 0, length)
		for i := 0; i < length; i++ {
			sVal, err := toStarlarkValue(v.Index(i).Interface())
			if err != nil {
				return starlark.None, err
			}
			array = append(array, sVal)
		}
		return starlark.NewList(array), nil
	case reflect.Map:
		dict := starlark.NewDict(v.Len())
		iter := v.MapRange()
		for iter.Next() {
			sKey, err := toStarlarkValue(iter.Key().Interface())
			if err != nil {
				return starlark.None, err
			}
			sValue, err := toStarlarkValue(iter.Value().Interface())
			if err != nil {
				return starlark.None, err
			}
			dict.SetKey(sKey, sValue)
		}
		return dict, nil
	case reflect.Float32, reflect.Float64:
		return starlark.Float(v.Float()), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return starlark.MakeInt64(v.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return starlark.MakeUint64(v.Uint()), nil
	case reflect.String:
		return starlark.String(v.String()), nil
	case reflect.Bool:
		return starlark.Bool(v.Bool()), nil
	}

	return starlark.None, errors.New("invalid type")
}

func toGoVal(value starlark.Value) (any, error) {
	switch v := value.(type) {
	case starlark.Float:
		return float64(v), nil
	case starlark.Int:
		n, ok := v.Int64()
		if !ok {
			return nil, errors.New("cannot represent integer as int64")
		}
		return n, nil
	case starlark.String:
		return string(v), nil
	case starlark.Bool:
		return bool(v), nil
	}

	return nil, errors.New("invalid starlark type")
}

func copyEvent(_ *starlark.Thread, _ *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var sm *event
	if err := starlark.UnpackPositionalArgs("copy_event", args, kwargs, 1, &sm); err != nil {
		return nil, err
	}
	tags := make(map[string]string)
	values := make(map[string]any)
	for k, v := range sm.ev.Tags {
		tags[k] = v
	}
	for k, v := range sm.ev.Values {
		values[k] = v
	}
	dup := &event{
		ev: &formatters.EventMsg{
			Name:      sm.ev.Name,
			Timestamp: sm.ev.Timestamp,
			Tags:      tags,
			Values:    values,
		},
	}

	return dup, nil
}
