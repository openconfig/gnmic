// © 2023 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package asciigraph_output

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/guptarohit/asciigraph"
	"github.com/nsf/termbox-go"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/protobuf/proto"

	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/api/utils"
	"github.com/openconfig/gnmic/pkg/config/store"
	"github.com/openconfig/gnmic/pkg/formatters"
	"github.com/openconfig/gnmic/pkg/gtemplate"
	"github.com/openconfig/gnmic/pkg/outputs"
	gutils "github.com/openconfig/gnmic/pkg/utils"
)

const (
	loggingPrefix       = "[asciigraph_output:%s] "
	defaultRefreshTimer = time.Second
	defaultPrecision    = 2
	defaultTimeout      = 10 * time.Second
)

var (
	defaultLabelColor   = asciigraph.Blue
	defaultCaptionColor = asciigraph.Default
	defaultAxisColor    = asciigraph.Default
)

func init() {
	outputs.Register("asciigraph", func() outputs.Output {
		return &asciigraphOutput{
			cfg:     &cfg{},
			logger:  log.New(io.Discard, loggingPrefix, utils.DefaultLoggingFlags),
			eventCh: make(chan *formatters.EventMsg, 100),
			m:       new(sync.RWMutex),
			data:    make(map[string]*series),
			colors:  make(map[asciigraph.AnsiColor]struct{}),
		}
	})
}

// asciigraphOutput //
type asciigraphOutput struct {
	outputs.BaseOutput
	cfg     *cfg
	logger  *log.Logger
	eventCh chan *formatters.EventMsg

	m       *sync.RWMutex
	data    map[string]*series
	colors  map[asciigraph.AnsiColor]struct{}
	caption string

	captionColor asciigraph.AnsiColor
	axisColor    asciigraph.AnsiColor
	labelColor   asciigraph.AnsiColor
	evps         []formatters.EventProcessor

	targetTpl *template.Template
	store     store.Store[any]
}

type series struct {
	name  string
	data  []float64
	color asciigraph.AnsiColor
}

// cfg //
type cfg struct {
	// The caption to be displayed under the graph
	Caption string `mapstructure:"caption,omitempty" json:"caption,omitempty"`
	// The graph height
	Height int `mapstructure:"height,omitempty" json:"height,omitempty"`
	// The graph width
	Width int `mapstructure:"width,omitempty" json:"width,omitempty"`
	// The graph minimum value for the vertical axis
	LowerBound *float64 `mapstructure:"lower-bound,omitempty" json:"lower-bound,omitempty"`
	// the graph maximum value for the vertical axis
	UpperBound *float64 `mapstructure:"upper-bound,omitempty" json:"upper-bound,omitempty"`
	// The graph offset
	Offset int `mapstructure:"offset,omitempty" json:"offset,omitempty"`
	// The decimal point precision of the label values
	Precision uint `mapstructure:"precision,omitempty" json:"precision,omitempty"`
	// The caption color
	CaptionColor string `mapstructure:"caption-color,omitempty" json:"caption-color,omitempty"`
	// The axis color
	AxisColor string `mapstructure:"axis-color,omitempty" json:"axis-color,omitempty"`
	// The label color
	LabelColor string `mapstructure:"label-color,omitempty" json:"label-color,omitempty"`
	// The graph refresh timer
	RefreshTimer time.Duration `mapstructure:"refresh-timer,omitempty" json:"refresh-timer,omitempty"`
	// Add target the received subscribe responses
	AddTarget string `mapstructure:"add-target,omitempty" json:"add-target,omitempty"`
	//
	TargetTemplate string `mapstructure:"target-template,omitempty" json:"target-template,omitempty"`
	// list of event processors
	EventProcessors []string `mapstructure:"event-processors,omitempty" json:"event-processors,omitempty"`
	// enable extra logging
	Debug bool `mapstructure:"debug,omitempty" json:"debug,omitempty"`
}

func (a *asciigraphOutput) String() string {
	b, err := json.Marshal(a.cfg)
	if err != nil {
		return ""
	}
	return string(b)
}

func (a *asciigraphOutput) setEventProcessors(logger *log.Logger) error {
	tcs, ps, acts, err := gutils.GetConfigMaps(a.store)
	if err != nil {
		return err
	}
	a.evps, err = formatters.MakeEventProcessors(
		logger,
		a.cfg.EventProcessors,
		ps,
		tcs,
		acts,
	)
	if err != nil {
		return err
	}
	return nil
}

func (a *asciigraphOutput) setLogger(logger *log.Logger) {
	if logger != nil && a.logger != nil {
		a.logger.SetOutput(logger.Writer())
		a.logger.SetFlags(logger.Flags())
	}
}

// Init //
func (a *asciigraphOutput) Init(ctx context.Context, name string, cfg map[string]interface{}, opts ...outputs.Option) error {
	err := outputs.DecodeConfig(cfg, a.cfg)
	if err != nil {
		return err
	}

	a.logger.SetPrefix(fmt.Sprintf(loggingPrefix, name))

	options := &outputs.OutputOptions{}
	for _, opt := range opts {
		if err := opt(options); err != nil {
			return err
		}
	}
	a.store = options.Store

	a.setLogger(options.Logger)

	err = a.setEventProcessors(options.Logger)
	if err != nil {
		return err
	}
	if a.cfg.TargetTemplate == "" {
		a.targetTpl = outputs.DefaultTargetTemplate
	} else if a.cfg.AddTarget != "" {
		a.targetTpl, err = gtemplate.CreateTemplate("target-template", a.cfg.TargetTemplate)
		if err != nil {
			return err
		}
		a.targetTpl = a.targetTpl.Funcs(outputs.TemplateFuncs)
	}
	// set defaults
	err = a.setDefaults()
	if err != nil {
		return err
	}
	//
	go a.graph(ctx)
	a.logger.Printf("initialized asciigraph output: %s", a.String())
	return nil
}

func (a *asciigraphOutput) setDefaults() error {
	a.labelColor = defaultLabelColor
	if a.cfg.LabelColor != "" {
		if lc, ok := asciigraph.ColorNames[a.cfg.LabelColor]; ok {
			a.labelColor = lc
		} else {
			return fmt.Errorf("unknown label color %s", a.cfg.LabelColor)
		}
	}

	a.captionColor = defaultCaptionColor
	if a.cfg.CaptionColor != "" {
		if lc, ok := asciigraph.ColorNames[a.cfg.CaptionColor]; ok {
			a.captionColor = lc
		} else {
			return fmt.Errorf("unknown caption color %s", a.cfg.CaptionColor)
		}
	}

	a.axisColor = defaultAxisColor
	if a.cfg.AxisColor != "" {
		if lc, ok := asciigraph.ColorNames[a.cfg.AxisColor]; ok {
			a.axisColor = lc
		} else {
			return fmt.Errorf("unknown axis color %s", a.cfg.AxisColor)
		}

	}

	if a.cfg.RefreshTimer <= 0 {
		a.cfg.RefreshTimer = defaultRefreshTimer
	}
	if a.cfg.Precision <= 0 {
		a.cfg.Precision = defaultPrecision
	}

	return a.getTermSize()
}

// Write //
func (a *asciigraphOutput) Write(ctx context.Context, rsp proto.Message, meta outputs.Meta) {
	if rsp == nil {
		return
	}

	subRsp, err := outputs.AddSubscriptionTarget(rsp, meta, a.cfg.AddTarget, a.targetTpl)
	if err != nil {
		a.logger.Printf("failed to add target to the response: %v", err)
		return
	}
	evs, err := formatters.ResponseToEventMsgs(meta["subscription-name"], subRsp, meta, a.evps...)
	if err != nil {
		a.logger.Printf("failed to convert messages to events: %v", err)
		return
	}
	for _, ev := range evs {
		a.WriteEvent(ctx, ev)
	}
}

func (a *asciigraphOutput) WriteEvent(ctx context.Context, ev *formatters.EventMsg) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	select {
	case <-ctx.Done():
		a.logger.Printf("write timeout: %v", ctx.Err())
	case a.eventCh <- ev:
	}
}

// Close //
func (a *asciigraphOutput) Close() error {
	return nil
}

// Metrics //
func (a *asciigraphOutput) RegisterMetrics(reg *prometheus.Registry) {
}

func (a *asciigraphOutput) SetName(name string) {}

func (a *asciigraphOutput) SetClusterName(name string) {}

func (a *asciigraphOutput) SetTargetsConfig(map[string]*types.TargetConfig) {}

func (a *asciigraphOutput) graph(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-a.eventCh:
			if !ok {
				return
			}
			a.plot(ev)
		case <-time.After(a.cfg.RefreshTimer):
			a.plot(nil)
		}
	}
}

func (a *asciigraphOutput) plot(e *formatters.EventMsg) {
	a.m.Lock()
	defer a.m.Unlock()
	a.getTermSize()
	if e != nil && len(e.Values) > 0 {
		a.updateData(e)
	}

	data, colors := a.buildData()
	if len(data) == 0 {
		return
	}
	opts := []asciigraph.Option{
		asciigraph.Height(a.cfg.Height),
		asciigraph.Width(a.cfg.Width),
		asciigraph.Offset(a.cfg.Offset),
		asciigraph.Precision(a.cfg.Precision),
		asciigraph.Caption(a.caption),
		asciigraph.CaptionColor(a.captionColor),
		asciigraph.SeriesColors(colors...),
		asciigraph.AxisColor(a.axisColor),
		asciigraph.LabelColor(a.labelColor),
	}
	if a.cfg.LowerBound != nil {
		opts = append(opts, asciigraph.LowerBound(*a.cfg.LowerBound))
	}
	if a.cfg.UpperBound != nil {
		opts = append(opts, asciigraph.UpperBound(*a.cfg.UpperBound))
	}
	plot := asciigraph.PlotMany(data, opts...)
	asciigraph.Clear()
	fmt.Fprintln(os.Stdout, plot)
}

func (a *asciigraphOutput) updateData(e *formatters.EventMsg) {
	if e == nil {
		return
	}
	evs := splitEvent(e)
	for _, ev := range evs {
		sn := a.buildSeriesName(e)
		serie := a.getOrCreateSerie(sn)
		for _, v := range ev.Values {
			i, err := toFloat(v)
			if err != nil {
				continue
			}
			serie.data = append(serie.data, i)
			break
		}
	}
}

func (a *asciigraphOutput) getOrCreateSerie(name string) *series {
	serie, ok := a.data[name]
	if ok {
		return serie
	}
	color := a.pickColor()
	serie = &series{
		name:  name,
		data:  make([]float64, 0, a.cfg.Width-a.cfg.Offset),
		color: color,
	}
	a.data[name] = serie
	a.colors[serie.color] = struct{}{}

	a.setCaption()
	return serie
}

func (a *asciigraphOutput) setCaption() {
	seriesNames := make([]string, 0, len(a.data))
	for seriesName := range a.data {
		seriesNames = append(seriesNames, seriesName)
	}
	sort.Strings(seriesNames)
	a.caption = ""
	if a.cfg.Debug {
		a.caption = fmt.Sprintf("(h=%d,w=%d)\n", a.cfg.Height, a.cfg.Width)
	}
	a.caption = fmt.Sprintf("%s\n", a.cfg.Caption)

	for _, sn := range seriesNames {
		color := a.data[sn].color
		a.caption += color.String() + "-+- " + sn + asciigraph.Default.String() + "\n"
	}
}

func (a *asciigraphOutput) buildData() ([][]float64, []asciigraph.AnsiColor) {
	numgraphs := len(a.data)
	series := make([]*series, 0, numgraphs)
	// sort series by name
	for _, serie := range a.data {
		size := len(serie.data)
		if size == 0 {
			continue
		}
		if size > a.cfg.Width {
			serie.data = serie.data[size-a.cfg.Width:]
		}
		series = append(series, serie)
	}
	sort.Slice(series,
		func(i, j int) bool {
			return series[i].name < series[j].name
		})

	data := make([][]float64, 0, numgraphs)
	colors := make([]asciigraph.AnsiColor, 0, numgraphs)
	// get float slices and colors
	for _, serie := range series {
		data = append(data, serie.data)
		colors = append(colors, serie.color)
	}
	return data, colors
}

func splitEvent(e *formatters.EventMsg) []*formatters.EventMsg {
	numVals := len(e.Values)
	switch numVals {
	case 0:
		return nil
	case 1:
		return []*formatters.EventMsg{e}
	}

	evs := make([]*formatters.EventMsg, 0, numVals)
	for k, v := range e.Values {
		ev := &formatters.EventMsg{
			Name:      e.Name,
			Timestamp: e.Timestamp,
			Tags:      e.Tags,
			Values:    map[string]interface{}{k: v},
		}
		evs = append(evs, ev)
	}
	return evs
}

func (a *asciigraphOutput) buildSeriesName(e *formatters.EventMsg) string {
	sb := &strings.Builder{}
	sb.WriteString(e.Name)
	sb.WriteString(":")
	for k := range e.Values {
		sb.WriteString(k)
	}
	numTags := len(e.Tags)
	if numTags == 0 {
		return sb.String()
	}
	sb.WriteString("{")
	tagNames := make([]string, 0, numTags)
	for k := range e.Tags {
		tagNames = append(tagNames, k)
	}
	sort.Strings(tagNames)
	for i, tn := range tagNames {
		fmt.Fprintf(sb, "%s=%s", tn, e.Tags[tn])
		if numTags != i+1 {
			sb.WriteString(", ")
		}
	}
	sb.WriteString("}")
	return sb.String()
}

func toFloat(v interface{}) (float64, error) {
	switch i := v.(type) {
	case float64:
		return float64(i), nil
	case float32:
		return float64(i), nil
	case int64:
		return float64(i), nil
	case int32:
		return float64(i), nil
	case int16:
		return float64(i), nil
	case int8:
		return float64(i), nil
	case uint64:
		return float64(i), nil
	case uint32:
		return float64(i), nil
	case uint16:
		return float64(i), nil
	case uint8:
		return float64(i), nil
	case int:
		return float64(i), nil
	case uint:
		return float64(i), nil
	case string:
		f, err := strconv.ParseFloat(i, 64)
		if err != nil {
			return math.NaN(), err
		}
		return f, err
		//lint:ignore SA1019 still need DecimalVal for backward compatibility
	case *gnmi.Decimal64:
		return float64(i.Digits) / math.Pow10(int(i.Precision)), nil
	default:
		return math.NaN(), errors.New("getFloat: unknown value is of incompatible type")
	}
}

func (a *asciigraphOutput) pickColor() asciigraph.AnsiColor {
	for _, c := range asciigraph.ColorNames {
		if _, ok := a.colors[c]; !ok {
			return c
		}
	}
	return 0
}

func (a *asciigraphOutput) getTermSize() error {
	err := termbox.Init()
	if err != nil {
		return fmt.Errorf("could not initialize a terminal box: %v", err)
	}
	w, h := termbox.Size()
	termbox.Close()
	if a.cfg.Width <= 0 || a.cfg.Width > w-10 {
		a.cfg.Width = w - 10
	}
	numSeries := len(a.data)
	if a.cfg.Height <= 0 || a.cfg.Height > h-(numSeries+1)-5 {
		a.cfg.Height = h - (numSeries + 1) - 5
	}
	return nil
}
