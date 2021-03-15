package http_action

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/antonmedv/expr"
	"github.com/antonmedv/expr/vm"
	"github.com/karimra/gnmic/actions"
	"github.com/karimra/gnmic/formatters"
)

const (
	defaultMethod        = "GET"
	defaultTimeout       = 5 * time.Second
	loggingPrefix        = "[http_action] "
	actionType           = "http"
	defaultExpressionAll = "event"
)

func init() {
	actions.Register(actionType, func() actions.Action {
		return &httpAction{
			logger: log.New(ioutil.Discard, "", 0),
		}
	})
}

type httpAction struct {
	Method     string            `mapstructure:"method,omitempty"`
	URL        string            `mapstructure:"url,omitempty"`
	Headers    map[string]string `mapstructure:"headers,omitempty"`
	Timeout    time.Duration     `mapstructure:"timeout,omitempty"`
	Template   string            `mapstructure:"template,omitempty"`
	Expression string            `mapstructure:"expression,omitempty"`
	Debug      bool              `mapstructure:"debug,omitempty"`

	tpl    *template.Template
	prg    *vm.Program
	logger *log.Logger
}

func (h *httpAction) Init(cfg map[string]interface{}, logger *log.Logger) error {
	err := actions.DecodeConfig(cfg, h)
	if err != nil {
		return err
	}
	if h.Debug && logger != nil {
		h.logger = log.New(logger.Writer(), loggingPrefix, logger.Flags())
	} else if h.Debug {
		h.logger = log.New(os.Stderr, loggingPrefix, log.LstdFlags|log.Lmicroseconds)
	}
	if h.Template != "" {
		h.tpl, err = template.ParseFiles(h.Template)
		if err != nil {
			return err
		}
	}
	err = h.setDefaults()
	if err != nil {
		return err
	}
	h.prg, err = expr.Compile(h.Expression)
	return err
}

func (h *httpAction) Run(e *formatters.EventMsg) (interface{}, error) {
	b := new(bytes.Buffer)
	if h.tpl != nil {
		err := h.tpl.Execute(b, map[string]*formatters.EventMsg{"event": e})
		if err != nil {
			return nil, err
		}
	} else {
		result, err := expr.Run(h.prg, map[string]*formatters.EventMsg{"event": e})
		if err != nil {
			return nil, err
		}
		fmt.Printf("result: %+v\n", result)
		b.Reset()
		err = json.NewEncoder(b).Encode(result)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(h.Method, h.URL, b)
	if err != nil {
		return nil, err
	}
	for k, v := range h.Headers {
		req.Header.Add(k, v)
	}
	client := &http.Client{
		Timeout: h.Timeout,
	}
	resp, err := client.Do(req)
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

func (h *httpAction) setDefaults() error {
	if !strings.HasPrefix(h.URL, "http") {
		h.URL = "http://" + h.URL
	}
	if _, err := url.Parse(h.URL); err != nil {
		return err
	}
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
	if h.Expression == "" {
		h.Expression = defaultExpressionAll
	}
	return nil
}
