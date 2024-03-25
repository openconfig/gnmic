package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/AlekSi/pointer"
	"github.com/openconfig/gnmic/pkg/config"
	"github.com/openconfig/gnmic/pkg/file"
	"github.com/openconfig/gnmic/pkg/formatters"
	promcom "github.com/openconfig/gnmic/pkg/outputs/prometheus_output"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/spf13/cobra"
)

func (a *App) ProcessorPreRunE(cmd *cobra.Command, args []string) error {
	a.Config.SetLocalFlagsFromFile(cmd)

	err := a.initPluginManager()
	if err != nil {
		return err
	}
	return nil
}

func (a *App) ProcessorRunE(cmd *cobra.Command, args []string) error {
	actionsConfig, err := a.Config.GetActions()
	if err != nil {
		return fmt.Errorf("failed reading actions config: %v", err)
	}
	pConfig, err := a.Config.GetEventProcessors()
	if err != nil {
		return fmt.Errorf("failed reading event processors config: %v", err)
	}
	tcs, err := a.Config.GetTargets()
	if err != nil {
		if !errors.Is(err, config.ErrNoTargetsFound) {
			return err
		}
	}
	// initialize processors
	evps, err := formatters.MakeEventProcessors(
		a.Logger,
		a.Config.LocalFlags.ProcessorName,
		pConfig,
		tcs,
		actionsConfig,
	)
	if err != nil {
		return err
	}
	// read input file
	inputBytes, err := file.ReadFile(cmd.Context(), a.Config.LocalFlags.ProcessorInput)
	if err != nil {
		return err
	}
	evInput := make([][]*formatters.EventMsg, 0)
	msgs := bytes.Split(inputBytes, []byte(a.Config.LocalFlags.ProcessorInputDelimiter))
	for i, bg := range msgs {
		if len(bg) == 0 {
			continue
		}
		mevs := make([]map[string]any, 0)
		err = json.Unmarshal(bg, &mevs)
		if err != nil {
			return fmt.Errorf("failed json Unmarshal at msg index %d: %s: %v", i, bg, err)
		}

		evs := make([]*formatters.EventMsg, 0, len(mevs))
		for _, mev := range mevs {
			ev, err := formatters.EventFromMap(mev)
			if err != nil {
				return err
			}
			evs = append(evs, ev)
		}
		evInput = append(evInput, evs)
	}
	rrevs := make([][]*formatters.EventMsg, 0, len(evInput))
	for _, evs := range evInput {
		revs := evs
		for _, p := range evps {
			revs = p.Apply(revs...)
		}
		rrevs = append(rrevs, revs)
	}

	if len(a.Config.LocalFlags.ProcessorOutput) != 0 {
		b, err := a.promFormat(rrevs, a.Config.LocalFlags.ProcessorOutput)
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}

	numEvOut := len(rrevs)
	for i, rev := range rrevs {
		b, err := json.MarshalIndent(rev, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		if i == numEvOut-1 {
			break
		}
	}
	return nil
}

func (a *App) InitProcessorFlags(cmd *cobra.Command) {
	cmd.ResetFlags()

	cmd.Flags().StringVarP(&a.Config.LocalFlags.ProcessorInput, "input", "", "", "processors input")
	cmd.MarkFlagRequired("input")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.ProcessorInputDelimiter, "delimiter", "", "\n", "processors input delimiter")
	cmd.Flags().StringSliceVarP(&a.Config.LocalFlags.ProcessorName, "name", "", nil, "list of processors to apply to the input")
	cmd.MarkFlagRequired("name")
	cmd.Flags().StringSliceVarP(&a.Config.LocalFlags.ProcessorPrometheusOpts, "prom-opts", "", nil, "list of prometheus output options")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.ProcessorOutput, "output", "", "", "output name")
}

func (a *App) promFormat(rrevs [][]*formatters.EventMsg, outName string) ([]byte, error) {
	// read output config
	outputPath := "outputs/" + outName
	outputConfig := a.Config.FileConfig.GetStringMap(outputPath)
	if outputConfig == nil {
		return nil, fmt.Errorf("unknown output name: %s", outName)
	}

	mb := &promcom.MetricBuilder{
		Prefix:                 a.Config.FileConfig.GetString(outputPath + "/metric-prefix"),
		AppendSubscriptionName: a.Config.FileConfig.GetBool(outputPath + "/append-subscription-name"),
		StringsAsLabels:        a.Config.FileConfig.GetBool(outputPath + "/strings-as-labels"),
		OverrideTimestamps:     a.Config.FileConfig.GetBool(outputPath + "/override-timestamps"),
		ExportTimestamps:       a.Config.FileConfig.GetBool(outputPath + "/export-timestamps"),
	}

	b := new(bytes.Buffer)
	now := time.Now()
	for _, revs := range rrevs {
		for _, ev := range revs {
			pms := mb.MetricsFromEvent(ev, now)
			for _, pm := range pms {
				m := &dto.Metric{}
				err := pm.Write(m)
				if err != nil {
					return nil, err
				}
				_, err = expfmt.MetricFamilyToText(b, &dto.MetricFamily{
					Name:   pointer.ToString(pm.Name),
					Help:   pointer.ToString("gNMIc generated metric"),
					Type:   dto.MetricType_UNTYPED.Enum(),
					Metric: []*dto.Metric{m},
				})
				if err != nil {
					return nil, err
				}
			}
		}
	}
	return b.Bytes(), nil
}
