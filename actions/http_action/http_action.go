package http_action

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/karimra/gnmic/actions"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
)

const (
	defaultMethod       = "GET"
	defaultTimeout      = 5 * time.Second
	loggingPrefix       = "[http_action] "
	actionType          = "http"
	defaultBodyTemplate = "{{ json . }}"
)

func init() {
	actions.Register(actionType, func() actions.Action {
		return &httpAction{
			logger: log.New(io.Discard, "", 0),
		}
	})
}

type httpAction struct {
	Name    string            `mapstructure:"name,omitempty"`
	Method  string            `mapstructure:"method,omitempty"`
	URL     string            `mapstructure:"url,omitempty"`
	Headers map[string]string `mapstructure:"headers,omitempty"`
	Timeout time.Duration     `mapstructure:"timeout,omitempty"`
	Body    string            `mapstructure:"body,omitempty"`
	Debug   bool              `mapstructure:"debug,omitempty"`

	url    *template.Template
	body   *template.Template
	logger *log.Logger
}

func (h *httpAction) Init(cfg map[string]interface{}, opts ...actions.Option) error {
	err := actions.DecodeConfig(cfg, h)
	if err != nil {
		return err
	}

	for _, opt := range opts {
		opt(h)
	}
	if h.Name == "" {
		return fmt.Errorf("action type %q missing name field", actionType)
	}
	err = h.setDefaults()
	if err != nil {
		return err
	}

	h.body, err = template.New("body").Funcs(funcMap).Parse(h.Body)
	if err != nil {
		return err
	}
	h.url, err = template.New("url").Funcs(funcMap).Parse(h.URL)
	return err
}

func (h *httpAction) Run(ctx context.Context, aCtx *actions.Context) (interface{}, error) {
	if h.url == nil {
		return nil, errors.New("missing url template")
	}
	if h.body == nil {
		return nil, errors.New("missing body template")
	}
	in := &actions.Context{
		Input:   aCtx.Input,
		Env:     aCtx.Env,
		Vars:    aCtx.Vars,
		Targets: aCtx.Targets,
	}
	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(in)
	if err != nil {
		return nil, err
	}

	b.Reset()
	err = h.body.Execute(b, in)
	if err != nil {
		return nil, err
	}
	url := new(bytes.Buffer)
	err = h.url.Execute(url, in)
	if err != nil {
		return nil, err
	}
	h.logger.Printf("url: %s", url.String())
	h.logger.Printf("body: %s", b.String())

	req, err := http.NewRequest(h.Method, url.String(), b)
	if err != nil {
		return nil, err
	}
	for k, v := range h.Headers {
		req.Header.Add(k, v)
	}
	client := &http.Client{
		Timeout: h.Timeout,
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		bodyBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return bodyBytes, nil
	}
	return nil, fmt.Errorf("status code=%d", resp.StatusCode)
}

func (h *httpAction) NName() string { return h.Name }

func (h *httpAction) setDefaults() error {
	// if !strings.HasPrefix(h.URL, "http") {
	// 	h.URL = "http://" + h.URL
	// }
	// if _, err := url.Parse(h.URL); err != nil {
	// 	return err
	// }
	if h.Method == "" {
		h.Method = defaultMethod
	}
	h.Method = strings.ToUpper(h.Method)
	switch h.Method {
	case http.MethodConnect:
		break
	case http.MethodDelete:
		break
	case http.MethodGet:
		break
	case http.MethodHead:
		break
	case http.MethodOptions:
		break
	case http.MethodPatch:
		break
	case http.MethodPost:
		break
	case http.MethodPut:
		break
	default:
		return fmt.Errorf("method %q not allowed", h.Method)
	}
	if h.Timeout <= 0 {
		h.Timeout = defaultTimeout
	}
	if h.Body == "" {
		h.Body = defaultBodyTemplate
	}
	return nil
}

func (h *httpAction) WithTargets(map[string]*types.TargetConfig) {}

func (h *httpAction) WithLogger(logger *log.Logger) {
	if h.Debug && logger != nil {
		h.logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
	} else if h.Debug {
		h.logger = log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)
	}
}

var funcMap = template.FuncMap{
	"json": func(v interface{}) string {
		a, _ := json.Marshal(v)
		return string(a)
	},
	"name": func(v interface{}) string {
		var result interface{}
		switch v := v.(type) {
		case *formatters.EventMsg:
			result = v.Name
		default:
			return ""
		}
		a, _ := json.Marshal(result)
		return string(a)
	},
	"withTags": func(v interface{}, keys ...string) string {
		switch v := v.(type) {
		case *formatters.EventMsg:
			tags := v.Tags
			v.Tags = make(map[string]string)
			for _, k := range keys {
				if vv, ok := tags[k]; ok {
					v.Tags[k] = vv
				}
			}
			a, _ := json.Marshal(v)
			return string(a)
		case string:
			msg := make(map[string]interface{})
			json.Unmarshal([]byte(v), &msg)
			tags := msg["tags"]
			if tags == nil {
				a, _ := json.Marshal(msg)
				return string(a)
			}
			tagsMap, ok := tags.(map[string]interface{})
			if !ok {
				a, _ := json.Marshal(msg)
				return string(a)
			}
			newTags := make(map[string]interface{})
			for _, k := range keys {
				if vv, ok := tagsMap[k]; ok {
					newTags[k] = vv
				}
			}
			delete(msg, "tags")
			if len(newTags) > 0 {
				msg["tags"] = newTags
			}
			a, _ := json.Marshal(msg)
			return string(a)
		}
		return ""
	},
	"withValues": func(v interface{}, keys ...string) string {
		switch v := v.(type) {
		case *formatters.EventMsg:
			values := v.Values
			v.Values = make(map[string]interface{})
			for _, k := range keys {
				if vv, ok := values[k]; ok {
					v.Values[k] = vv
				}
			}
			a, _ := json.Marshal(v)
			return string(a)
		case string:
			msg := make(map[string]interface{})
			json.Unmarshal([]byte(v), &msg)
			values := msg["values"]
			if values == nil {
				a, _ := json.Marshal(msg)
				return string(a)
			}
			valuesMap, ok := values.(map[string]interface{})
			if !ok {
				a, _ := json.Marshal(msg)
				return string(a)
			}
			newValues := make(map[string]interface{})
			for _, k := range keys {
				if vv, ok := valuesMap[k]; ok {
					newValues[k] = vv
				}
			}
			delete(msg, "values")
			if len(newValues) > 0 {
				msg["values"] = newValues
			}
			a, _ := json.Marshal(msg)
			return string(a)
		}

		return ""
	},
	"withoutTags": func(v interface{}, keys ...string) string {
		switch v := v.(type) {
		case *formatters.EventMsg:
			for _, k := range keys {
				delete(v.Tags, k)
			}
			a, _ := json.Marshal(v)
			return string(a)
		case string:
			msg := make(map[string]interface{})
			json.Unmarshal([]byte(v), &msg)
			tags := msg["tags"]
			if tags == nil {
				a, _ := json.Marshal(msg)
				return string(a)
			}
			switch tags := msg["tags"].(type) {
			case map[string]interface{}:
				for _, k := range keys {
					delete(tags, k)
				}
				msg["tags"] = tags
			}
			a, _ := json.Marshal(msg)
			return string(a)
		}
		return ""
	},
	"withoutValues": func(v interface{}, keys ...string) string {
		switch v := v.(type) {
		case *formatters.EventMsg:
			for _, k := range keys {
				delete(v.Values, k)
			}
			a, _ := json.Marshal(v)
			return string(a)
		case string:
			msg := make(map[string]interface{})
			json.Unmarshal([]byte(v), &msg)
			if msg["values"] == nil {
				a, _ := json.Marshal(msg)
				return string(a)
			}
			switch values := msg["values"].(type) {
			case map[string]interface{}:
				for _, k := range keys {
					delete(values, k)
				}
				msg["values"] = values
			}
			a, _ := json.Marshal(msg)
			return string(a)
		}
		return ""
	},
}
