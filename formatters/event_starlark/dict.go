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
	"sort"

	"go.starlark.net/starlark"
)

type isDict interface {
	starlark.HasSetKey
	starlark.IterableMapping
	Clear() error
	Delete(starlark.Value) (starlark.Value, bool, error)
}

type dict[K comparable, V any] struct {
	name      string
	m         map[K]V
	iterCount int
	frozen    bool
}

func newDict[K comparable, V any](name string, m map[K]V) *dict[K, V] {
	if m == nil {
		m = make(map[K]V)
	}
	return &dict[K, V]{name: name, m: m}
}

type builtinMethod func(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error)

// https://github.com/google/starlark-go/blob/243c74974e97462c5df21338e182470391748b04/starlark/library.go#L147
func builtinAttr(recv starlark.Value, name string, methods map[string]builtinMethod) (starlark.Value, error) {
	method := methods[name]
	if method == nil {
		return starlark.None, fmt.Errorf("no such method %q", name)
	}

	// Allocate a closure over 'method'.
	impl := func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		return method(b, args, kwargs)
	}
	return starlark.NewBuiltin(name, impl).BindReceiver(recv), nil
}

func builtinAttrNames(methods map[string]builtinMethod) []string {
	names := make([]string, 0, len(methods))
	for name := range methods {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// dict implements starlark.Value
func (d *dict[K, V]) String() string {
	b, _ := json.Marshal(d.m)
	return string(b)
}

// dict implements starlark.Value
func (d *dict[K, V]) Type() string {
	return d.name
}

// dict implements starlark.Value
func (d *dict[K, V]) Freeze() {
	d.frozen = true
}

// dict implements starlark.Value
func (d *dict[K, V]) Truth() starlark.Bool {
	return len(d.m) != 0
}

// dict implements starlark.Value
func (d *dict[K, V]) Hash() (uint32, error) {
	return 0, errors.New("dict is not hashable")
}

// AttrNames implements the starlark.HasAttrs interface.
func (d *dict[K, V]) AttrNames() []string {
	return builtinAttrNames(dictMethods)
}

// Attr implements the starlark.HasAttrs interface.
func (d *dict[K, V]) Attr(name string) (starlark.Value, error) {
	return builtinAttr(d, name, dictMethods)
}

var dictMethods = map[string]builtinMethod{
	"clear":      dictClear,
	"get":        dictGet,
	"items":      dictItems,
	"keys":       dictKeys,
	"pop":        dictPop,
	"setdefault": dictSetDefault,
	"update":     dictUpdate,
	"values":     dictValues,
}

// Get implements the starlark.Mapping interface.
func (d *dict[K, V]) Get(key starlark.Value) (v starlark.Value, found bool, err error) {
	k, err := toGoVal(key)
	if err != nil {
		return nil, false, err
	}
	if kk, ok := k.(K); ok {
		gv, found := d.m[kk]
		if !found {
			return starlark.None, false, nil
		}
		vv, err := toStarlarkValue(gv)
		return vv, true, err
	}
	return starlark.None, false, errors.New("key must be of type 'string'")
}

// SetKey implements the starlark.HasSetKey interface to support map update
// using x[k]=v syntax, like a dictionary.
func (d *dict[K, V]) SetKey(k, v starlark.Value) error {
	if d.iterCount > 0 {
		return fmt.Errorf("cannot insert during iteration")
	}
	kk, err := toGoVal(k)
	if err != nil {
		return err
	}
	key, ok := kk.(K)
	if !ok {
		return fmt.Errorf("unexpected key type: %T", kk)
	}

	vv, err := toGoVal(v)
	if err != nil {
		return err
	}
	if val, ok := vv.(V); ok {
		d.m[key] = val
		return nil
	}

	return fmt.Errorf("unexpected value type: %T", vv)
}

// Items implements the starlark.IterableMapping interface.
func (d *dict[K, V]) Items() []starlark.Tuple {
	items := make([]starlark.Tuple, 0, len(d.m))
	for k, v := range d.m {
		value, err := toStarlarkValue(v)
		if err != nil {
			continue
		}
		kk, err := toStarlarkValue(k)
		if err != nil {
			continue
		}
		pair := starlark.Tuple{kk, value}
		items = append(items, pair)
	}
	return items
}

func (d *dict[K, V]) Clear() error {
	if d.iterCount > 0 {
		return fmt.Errorf("cannot clear dict during iteration")
	}
	for k := range d.m {
		delete(d.m, k)
	}
	return nil
}

func (d *dict[K, V]) Delete(k starlark.Value) (v starlark.Value, found bool, err error) {
	if d.iterCount > 0 {
		return nil, false, fmt.Errorf("cannot delete a key during iteration")
	}
	gk, err := toGoVal(k)
	if err != nil {
		return nil, false, err
	}
	gkk, ok := gk.(K)
	if !ok {
		return nil, false, fmt.Errorf("unexpected key type: %T", gk)
	}
	value, ok := d.m[gkk]
	if ok {
		delete(d.m, gkk)
		v, err := toStarlarkValue(value)
		return v, ok, err
	}
	return starlark.None, false, nil
}

// Iterate implements the starlark.Iterator interface.
func (d *dict[K, V]) Iterate() starlark.Iterator {
	d.iterCount++
	tags := make([]*tag[K, V], 0, len(d.m))
	for k, v := range d.m {
		tags = append(tags, &tag[K, V]{key: k, value: v})
	}
	return &dictIterator[K, V]{
		dict: &dict[K, V]{m: d.m},
		tags: tags,
	}
}

type tag[K, V any] struct {
	key   K
	value V
}

type dictIterator[K comparable, V any] struct {
	*dict[K, V]
	tags []*tag[K, V]
}

// Next implements the starlark.Iterator interface.
func (i *dictIterator[K, V]) Next(p *starlark.Value) bool {
	if len(i.tags) == 0 {
		return false
	}

	tag := i.tags[0]
	i.tags = i.tags[1:]
	sk, err := toStarlarkValue(tag.key)
	if err != nil {
		return false
	}
	*p = sk

	return true
}

// Done implements the starlark.Iterator interface.
func (i *dictIterator[K, V]) Done() {
	i.iterCount--
}

// --- dictionary methods ---

// https://github.com/google/starlark-go/blob/master/doc/spec.md#dict·clear
func dictClear(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0); err != nil {
		return starlark.None, fmt.Errorf("%s: %v", b.Name(), err)
	}

	return starlark.None, b.Receiver().(isDict).Clear()
}

// https://github.com/google/starlark-go/blob/master/doc/spec.md#dict·pop
func dictPop(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var k, d starlark.Value
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &k, &d); err != nil {
		return starlark.None, fmt.Errorf("%s: %v", b.Name(), err)
	}

	v, found, err := b.Receiver().(isDict).Delete(k)
	if err != nil {
		return starlark.None, fmt.Errorf("%s: %v", b.Name(), err)
	}
	if found {
		return v, nil
	}
	if d != nil {
		return d, nil
	}
	return starlark.None, fmt.Errorf("%s: missing key", b.Name())
}

// https://github.com/google/starlark-go/blob/master/doc/spec.md#dict·get
func dictGet(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key, d starlark.Value
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &key, &d); err != nil {
		return starlark.None, fmt.Errorf("%s: %v", b.Name(), err)
	}
	v, ok, err := b.Receiver().(isDict).Get(key)
	if err != nil {
		return starlark.None, fmt.Errorf("%s: %v", b.Name(), err)
	}
	if ok {
		return v, nil
	}
	if d != nil {
		return d, nil
	}
	return starlark.None, nil
}

// https://github.com/google/starlark-go/blob/master/doc/spec.md#dict·setdefault
func dictSetDefault(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var key starlark.Value
	var d = starlark.None
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &key, &d); err != nil {
		return starlark.None, fmt.Errorf("%s: %v", b.Name(), err)
	}

	recv := b.Receiver().(isDict)
	v, found, err := recv.Get(key)
	if err != nil {
		return starlark.None, fmt.Errorf("%s: %v", b.Name(), err)
	}
	if !found {
		v = d
		if err := recv.SetKey(key, d); err != nil {
			return starlark.None, fmt.Errorf("%s: %v", b.Name(), err)
		}
	}
	return v, nil
}

// https://github.com/google/starlark-go/blob/master/doc/spec.md#dict·update
func dictUpdate(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	// Unpack the arguments
	if len(args) > 1 {
		return nil, fmt.Errorf("update: got %d arguments, want at most 1", len(args))
	}

	// Get the target
	recv := b.Receiver().(isDict)

	if len(args) == 1 {
		switch updates := args[0].(type) {
		case starlark.IterableMapping:
			// Iterate over dict's key/value pairs, not just keys.
			for _, item := range updates.Items() {
				if err := recv.SetKey(item[0], item[1]); err != nil {
					return nil, err // dict is frozen
				}
			}
		case starlark.Iterable:
			// all other sequences
			iter := starlark.Iterate(updates)
			if iter == nil {
				return nil, fmt.Errorf("got %s, want iterable", updates.Type())
			}
			defer iter.Done()
			var pair starlark.Value
			for i := 0; iter.Next(&pair); i++ {
				iter2 := starlark.Iterate(pair)
				if iter2 == nil {
					return nil, fmt.Errorf("dictionary update sequence element #%d is not iterable (%s)", i, pair.Type())
				}
				defer iter2.Done()
				length := starlark.Len(pair)
				if length < 0 {
					return nil, fmt.Errorf("dictionary update sequence element #%d has unknown length (%s)", i, pair.Type())
				}
				if length != 2 {
					return nil, fmt.Errorf("dictionary update sequence element #%d has length %d, want 2", i, length)
				}
				var k, v starlark.Value
				iter2.Next(&k)
				iter2.Next(&v)
				err := recv.SetKey(k, v)
				if err != nil {
					return nil, err
				}
			}
		default:
			return nil, errors.New("cannot update dict: update values are not iterable")
		}
	}

	// Then add the kwargs.
	before := starlark.Len(recv)
	for _, pair := range kwargs {
		if err := recv.SetKey(pair[0], pair[1]); err != nil {
			return nil, err // dict is frozen
		}
	}
	// In the common case, each kwarg will add another dict entry.
	// If that's not so, check whether it is because there was a duplicate kwarg.
	if starlark.Len(recv) < before+len(kwargs) {
		keys := make(map[starlark.String]bool, len(kwargs))
		for _, kv := range kwargs {
			k := kv[0].(starlark.String)
			if keys[k] {
				return nil, fmt.Errorf("duplicate keyword arg: %v", k)
			}
			keys[k] = true
		}
	}

	return starlark.None, nil
}

// https://github.com/google/starlark-go/blob/master/doc/spec.md#dict·items
func dictItems(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0); err != nil {
		return starlark.None, fmt.Errorf("%s: %v", b.Name(), err)
	}
	items := b.Receiver().(isDict).Items()
	res := make([]starlark.Value, len(items))
	for i, item := range items {
		res[i] = item // convert [2]starlark.Value to starlark.Value
	}
	return starlark.NewList(res), nil
}

// https://github.com/google/starlark-go/blob/master/doc/spec.md#dict·keys
func dictKeys(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0); err != nil {
		return starlark.None, fmt.Errorf("%s: %v", b.Name(), err)
	}

	items := b.Receiver().(isDict).Items()
	res := make([]starlark.Value, len(items))
	for i, item := range items {
		res[i] = item[0]
	}
	return starlark.NewList(res), nil
}

// https://github.com/google/starlark-go/blob/master/doc/spec.md#dict·update
func dictValues(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 0); err != nil {
		return starlark.None, fmt.Errorf("%s: %v", b.Name(), err)
	}
	items := b.Receiver().(isDict).Items()
	res := make([]starlark.Value, len(items))
	for i, item := range items {
		res[i] = item[1]
	}
	return starlark.NewList(res), nil
}
