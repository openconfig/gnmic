package utils

import (
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/karimra/gnmic/testutils"
	"github.com/openconfig/gnmi/proto/gnmi"
)

var prefixSet = map[string]*gnmi.Path{
	"": nil,
	"target%%%origin:/e1/e2": {
		Origin: "origin",
		Target: "target",
		Elem: []*gnmi.PathElem{
			{Name: "e1"},
			{Name: "e2"},
		},
	},
	"/e1": {
		Elem: []*gnmi.PathElem{
			{Name: "e1"},
		},
	},
	"/e1/e2[k=v]": {
		Elem: []*gnmi.PathElem{
			{Name: "e1"},
			{Name: "e2",
				Key: map[string]string{
					"k": "v",
				}},
		},
	},
}

var pathsTable = map[string]struct {
	strPath     string
	gnmiPath    *gnmi.Path
	isOK        bool
	expectedErr error
}{
	"empty_path": {
		strPath:     "",
		gnmiPath:    &gnmi.Path{},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_slash_only": {
		strPath:  "/",
		gnmiPath: &gnmi.Path{},
		isOK:     true,
	},
	"path_with_one_path_element": {
		strPath: "e",
		gnmiPath: &gnmi.Path{
			Elem: []*gnmi.PathElem{
				{Name: "e"},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_one_path_element_with_slash": {
		strPath: "/e",
		gnmiPath: &gnmi.Path{
			Elem: []*gnmi.PathElem{
				{Name: "e"},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_two_path_elements": {
		strPath: "/e1/e2",
		gnmiPath: &gnmi.Path{
			Elem: []*gnmi.PathElem{
				{Name: "e1"},
				{Name: "e2"},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_two_path_elements_with_key": {
		strPath: "/e1/e2[k=v]",
		gnmiPath: &gnmi.Path{
			Elem: []*gnmi.PathElem{
				{Name: "e1"},
				{Name: "e2",
					Key: map[string]string{
						"k": "v",
					}},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_multiple_path_elements_and_multiple_keys": {
		strPath: "/e1/e2[k1=v1][k2=v2]",
		gnmiPath: &gnmi.Path{
			Elem: []*gnmi.PathElem{
				{Name: "e1"},
				{Name: "e2",
					Key: map[string]string{
						"k1": "v1",
						"k2": "v2",
					}},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_origin": {
		strPath: "origin:/e1/e2",
		gnmiPath: &gnmi.Path{
			Origin: "origin",
			Elem: []*gnmi.PathElem{
				{Name: "e1"},
				{Name: "e2"},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_origin_only": {
		strPath: "origin:",
		gnmiPath: &gnmi.Path{
			Origin: "origin",
		},
		isOK: true,
	},
	"path_with_origin_and_slash_only": {
		strPath: "origin:/",
		gnmiPath: &gnmi.Path{
			Origin: "origin",
		},
		isOK: true,
	},
	"path_with_empty_origin": {
		strPath:  ":",
		gnmiPath: &gnmi.Path{},
		isOK:     true,
	},
	"path_with_empty_origin_and_slash_only": {
		strPath:  ":/",
		gnmiPath: &gnmi.Path{},
		isOK:     true,
	},
	"path_with_origin_and_key": {
		strPath: "origin:/e1/e2[k=v]",
		gnmiPath: &gnmi.Path{
			Origin: "origin",
			Elem: []*gnmi.PathElem{
				{Name: "e1"},
				{Name: "e2",
					Key: map[string]string{
						"k": "v",
					}},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_origin_and_multiple_keys": {
		strPath: "origin:/e1[name=object]/e2[addr=1.1.1.1/32]",
		gnmiPath: &gnmi.Path{
			Origin: "origin",
			Elem: []*gnmi.PathElem{
				{Name: "e1",
					Key: map[string]string{
						"name": "object",
					}},
				{Name: "e2",
					Key: map[string]string{
						"addr": "1.1.1.1/32",
					}},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_colon_in_path_elem": {
		strPath: "origin:/e1:e1[k=1.1.1.1/32]/e2[k1=v2]",
		gnmiPath: &gnmi.Path{
			Origin: "origin",
			Elem: []*gnmi.PathElem{
				{Name: "e1:e1",
					Key: map[string]string{
						"k": "1.1.1.1/32",
					},
				},
				{Name: "e2",
					Key: map[string]string{
						"k1": "v2",
					},
				},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_colon_in_2_path_elems": {
		strPath: "origin:/e1:e1[k=1.1.1.1/32]/e2:e3[k1=v2]",
		gnmiPath: &gnmi.Path{
			Origin: "origin",
			Elem: []*gnmi.PathElem{
				{Name: "e1:e1",
					Key: map[string]string{
						"k": "1.1.1.1/32",
					},
				},
				{Name: "e2:e3",
					Key: map[string]string{
						"k1": "v2",
					},
				},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_escaped_open_bracket": {
		strPath: `/e1\[/e2[k=v]`,
		gnmiPath: &gnmi.Path{
			Elem: []*gnmi.PathElem{
				{Name: `e1\[`},
				{Name: "e2",
					Key: map[string]string{
						"k": "v",
					}},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_escaped_close_bracket": {
		strPath: `/e1\]/e2[k=v]`,
		gnmiPath: &gnmi.Path{
			Elem: []*gnmi.PathElem{
				{Name: `e1\]`},
				{Name: "e2",
					Key: map[string]string{
						"k": "v",
					}},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_colon_in_first_path_elem": {
		strPath: `e1:e2/e3[k=v]`,
		gnmiPath: &gnmi.Path{
			Elem: []*gnmi.PathElem{
				{Name: "e1:e2"},
				{Name: "e3",
					Key: map[string]string{
						"k": "v",
					}},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_colon_in_key_value": {
		strPath: `/e1/e2[k=v:1]`,
		gnmiPath: &gnmi.Path{
			Elem: []*gnmi.PathElem{
				{Name: "e1"},
				{Name: "e2",
					Key: map[string]string{
						"k": "v:1",
					}},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_without_origin_with_colon_in_path_elem": {
		strPath: `e1/e2:e3[k=v:1]`,
		gnmiPath: &gnmi.Path{
			Elem: []*gnmi.PathElem{
				{Name: "e1"},
				{Name: "e2:e3",
					Key: map[string]string{
						"k": "v:1",
					}},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_origin_and_colon_in_key_value": {
		strPath: `origin:/e1/e2[k=v:1]`,
		gnmiPath: &gnmi.Path{
			Origin: "origin",
			Elem: []*gnmi.PathElem{
				{Name: "e1"},
				{Name: "e2",
					Key: map[string]string{
						"k": "v:1",
					}},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_origin_and_colon_space_in_key_value": {
		strPath: `origin:/e1/e2[k=v a:1]`,
		gnmiPath: &gnmi.Path{
			Origin: "origin",
			Elem: []*gnmi.PathElem{
				{Name: "e1"},
				{Name: "e2",
					Key: map[string]string{
						"k": `v a:1`,
					}},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_origin_and_colon_space_in_key_value_double_quoted_value": {
		strPath: `origin:/e1/e2[k="v a:1"]`,
		gnmiPath: &gnmi.Path{
			Origin: "origin",
			Elem: []*gnmi.PathElem{
				{Name: "e1"},
				{Name: "e2",
					Key: map[string]string{
						"k": `"v a:1"`,
					}},
			},
		},
		isOK:        true,
		expectedErr: nil,
	},
	"path_with_missing_closing_bracket": {
		strPath:     `/e1/e2[k=v`,
		gnmiPath:    nil,
		isOK:        false,
		expectedErr: errMalformedXPath,
	},
	"path_with_missing_open_bracket": {
		strPath:     `/e1/e2k=v]`,
		gnmiPath:    nil,
		isOK:        false,
		expectedErr: errMalformedXPath,
	},
	"path_with_key_missing_equal_sign": {
		strPath:     `/e1/e2[k]`,
		gnmiPath:    nil,
		isOK:        false,
		expectedErr: errMalformedXPathKey,
	},
}

type outKeysSet struct {
	out map[string]string
	err error
}
type outPathElemSet struct {
	out *gnmi.PathElem
	err error
}

var keysSet = map[string]struct {
	in  string
	exp outKeysSet
}{
	"no_key": {
		in: "",
		exp: outKeysSet{
			out: nil,
			err: nil,
		},
	},
	"one_key": {
		in: "[k=v]",
		exp: outKeysSet{
			out: map[string]string{"k": "v"},
			err: nil,
		},
	},
	"two_key": {
		in: "[k1=v1][k2=1.1.1.1/30]",
		exp: outKeysSet{
			out: map[string]string{"k1": "v1", "k2": "1.1.1.1/30"},
			err: nil,
		},
	},
	"noval_key": {
		in: "[k1=]",
		exp: outKeysSet{
			out: nil,
			err: errMalformedXPathKey,
		},
	},
	"nokey_with_val": {
		in: "[=v]",
		exp: outKeysSet{
			out: nil,
			err: errMalformedXPathKey,
		},
	},
	"inKey_brackets": {
		in: "[k=[v]",
		exp: outKeysSet{
			out: nil,
			err: errMalformedXPathKey,
		},
	},
	"inKey_escaped_open_bracket": {
		in: `[k=\[v]`,
		exp: outKeysSet{
			out: map[string]string{"k": "[v"},
			err: nil,
		},
	},
	"inKey_escaped_close_bracket": {
		in: `[k=\]v]`,
		exp: outKeysSet{
			out: map[string]string{"k": "]v"},
			err: nil,
		},
	},
	"inKey_escaped_brackets": {
		in: `[\[k=\]v]`,
		exp: outKeysSet{
			out: map[string]string{"[k": "]v"},
			err: nil,
		},
	},
}
var pathElemSet = map[string]struct {
	in  string
	out outPathElemSet
}{
	"no_key": {
		in: "elem1",
		out: outPathElemSet{
			out: &gnmi.PathElem{Name: "elem1"},
			err: nil,
		},
	},
	"with_1_key": {
		in: "elem1[k=v]",
		out: outPathElemSet{
			out: &gnmi.PathElem{Name: "elem1", Key: map[string]string{"k": "v"}},
			err: nil,
		},
	},
	"with_2_keys": {
		in: "elem1[k1=v1][k2=v2]",
		out: outPathElemSet{
			out: &gnmi.PathElem{Name: "elem1", Key: map[string]string{"k1": "v1", "k2": "v2"}},
			err: nil,
		},
	},
	"with_1_key_malformed": {
		in: "elem1[k1=v1",
		out: outPathElemSet{
			out: nil,
			err: errMalformedXPathKey,
		},
	},
	"elem_with_escaped_bracket": {
		in: `elem1\[k1=v1`,
		out: outPathElemSet{
			out: &gnmi.PathElem{Name: `elem1\[k1=v1`},
			err: nil,
		},
	},
}

func TestCreatePrefix(t *testing.T) {
	var target, prefix string
	for e, p := range prefixSet {
		val := strings.Split(e, "%%%")
		//fmt.Printf("%d: %v\n", len(val), val)
		if len(val) == 2 {
			target, prefix = val[0], val[1]
		} else if len(val) == 1 {
			target, prefix = "", val[0]
		}
		//fmt.Println(target, prefix)
		gp, err := CreatePrefix(prefix, target)
		if err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(p, gp) {
			t.Errorf("failed at elem: %s: expecting %v, got %v", e, p, gp)
		}

	}
}

func TestParsePath(t *testing.T) {
	for name, tc := range pathsTable {
		t.Run(name, func(t *testing.T) {
			p, err := ParsePath(tc.strPath)
			if err != nil && tc.isOK {
				t.Fatal(err)
			}
			if !tc.isOK {
				if err != tc.expectedErr {
					t.Errorf("failed at '%s', expected error %+v, got %+v", name, tc.expectedErr, err)
				}
				return
			}
			if !testutils.GnmiPathsEqual(p, tc.gnmiPath) {
				t.Errorf("failed at '%s', expected %v, got %+v", name, tc.gnmiPath, p)
			}
		})
	}
}

func TestParseXPathKeys(t *testing.T) {
	for name, input := range keysSet {
		t.Run(name, func(t *testing.T) {
			keys, err := parseXPathKeys(input.in)
			if !cmp.Equal(keys, input.exp.out) {
				t.Errorf("failed at '%s', expected %v, got %+v", name, input.exp.out, keys)
			}
			if err != input.exp.err {
				t.Errorf("failed at '%s', expected error %+v, got %+v", name, input.exp.err, err)
			}
		})
	}
}

func TestStringToPathElem(t *testing.T) {
	for name, input := range pathElemSet {
		t.Run(name, func(t *testing.T) {
			gnmiPathElem, err := toPathElem(input.in)
			if gnmiPathElem == nil || input.out.out == nil {
				if gnmiPathElem != input.out.out {
					t.Errorf("failed at '%s', expected %v, got %+v", name, input.out.out, gnmiPathElem)
				}
			} else if !cmp.Equal(gnmiPathElem.Key, input.out.out.Key) || gnmiPathElem.Name != input.out.out.Name {
				t.Errorf("failed at '%s', expected %v, got %+v", name, input.out.out, gnmiPathElem)
			}
			if err != input.out.err {
				t.Errorf("failed at '%s', expected error %+v, got %+v", name, input.out.err, err)
			}
		})
	}
}

func BenchmarkParsePath(b *testing.B) {
	for name, tc := range pathsTable {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				ParsePath(tc.strPath)
			}
		})
	}
}
