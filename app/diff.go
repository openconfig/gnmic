package app

import (
	"context"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/formatters"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"google.golang.org/protobuf/proto"
)

type targetDiffResponse struct {
	t  string
	r  *gnmi.GetResponse
	rs []proto.Message
}

// InitDiffFlags used to init or reset diffCmd flags for gnmic-prompt mode
func (a *App) InitDiffFlags(cmd *cobra.Command) {
	cmd.ResetFlags()

	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.DiffPath, "path", "", []string{}, "diff request paths")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.DiffRef, "ref", "", "", "reference node to compare the other nodes to")
	cmd.MarkFlagRequired("ref")
	cmd.Flags().StringArrayVarP(&a.Config.LocalFlags.DiffCompare, "compare", "", []string{}, "nodes to compare to the reference")
	cmd.MarkFlagRequired("compare")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.DiffPrefix, "prefix", "", "", "diff request prefix")
	cmd.Flags().StringSliceVarP(&a.Config.LocalFlags.DiffModel, "model", "", []string{}, "diff request models")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.DiffType, "type", "t", "ALL", "data type requested from the target. one of: ALL, CONFIG, STATE, OPERATIONAL")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.DiffTarget, "target", "", "", "get request target")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.DiffSub, "sub", "", false, "use subscribe ONCE mode instead of a get request")

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}

func (a *App) DiffRun(cmd *cobra.Command, args []string) error {
	defer a.InitDiffFlags(cmd)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// setupCloseHandler(cancel)
	refTarget, targetsConfig, err := a.Config.GetDiffTargets()
	if err != nil {
		return fmt.Errorf("failed getting diff targets config: %v", err)
	}
	if refTarget == nil {
		return fmt.Errorf("failed getting diff reference target config")
	}
	if len(targetsConfig) == 0 {
		return fmt.Errorf("failed getting diff compare targets config")
	}
	if a.collector == nil {
		cfg := &collector.Config{
			Debug:               a.Config.Debug,
			Format:              a.Config.Format,
			TargetReceiveBuffer: a.Config.TargetBufferSize,
			RetryTimer:          a.Config.Retry,
		}
		allTargets := make(map[string]*collector.TargetConfig)
		for n, tc := range targetsConfig {
			allTargets[n] = tc
		}
		allTargets[refTarget.Name] = refTarget

		a.collector = collector.NewCollector(cfg, allTargets,
			collector.WithDialOptions(a.createCollectorDialOpts()),
			collector.WithLogger(a.Logger),
		)
	} else {
		// prompt mode
		a.collector.AddTarget(refTarget)
		for _, tc := range targetsConfig {
			a.collector.AddTarget(tc)
		}
	}

	numTargets := len(targetsConfig) + 1
	a.errCh = make(chan error, numTargets*2)
	a.wg.Add(numTargets)

	compares := make([]string, 0, len(targetsConfig))
	for t := range targetsConfig {
		compares = append(compares, t)
	}
	sort.Strings(compares)

	err = a.diff(ctx, refTarget.Name, compares)
	if err != nil {
		a.logError(err)
	}
	return a.checkErrors()
}

func (a *App) diff(ctx context.Context, ref string, compare []string) error {
	if a.Config.DiffSub {
		return a.subscribeBasedDiff(ctx, ref, compare)
	}
	return a.getBasedDiff(ctx, ref, compare)
}

func (a *App) subscribeBasedDiff(ctx context.Context, ref string, compare []string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	subReq, err := a.Config.CreateDiffSubscribeRequest()
	if err != nil {
		return err
	}
	numCompares := len(compare)
	refResponse := make([]proto.Message, 0)
	rspChan := make(chan *targetDiffResponse, numCompares)
	err = a.collector.CreateTarget(ref)
	if err != nil {
		return err
	}
	if refTarget, ok := a.collector.Targets[ref]; ok {
		go func() {
			defer a.wg.Done()
			err = refTarget.CreateGNMIClient(ctx, a.createCollectorDialOpts()...)
			if err != nil {
				a.logError(err)
				return
			}
			rspChan, errChan := refTarget.SubscribeOnce(ctx, subReq, "diff-sub")
			for {
				select {
				case r := <-rspChan:
					switch r.Response.(type) {
					case *gnmi.SubscribeResponse_Update:
						refResponse = append(refResponse, r)
					case *gnmi.SubscribeResponse_SyncResponse:
						return
					}
				case err := <-errChan:
					if err != io.EOF {
						a.logError(err)
					}
					return
				}
			}
		}()

	} else {
		return fmt.Errorf("unknown reference target %q", ref)
	}
	for _, tName := range compare {
		err = a.collector.CreateTarget(tName)
		if err != nil {
			return err
		}
		if t, ok := a.collector.Targets[tName]; ok {
			go func(tName string) {
				defer a.wg.Done()
				err = t.CreateGNMIClient(ctx, a.createCollectorDialOpts()...)
				if err != nil {
					a.logError(err)
					return
				}
				responses := make([]proto.Message, 0)
				subRspChan, errChan := t.SubscribeOnce(ctx, subReq, "diff-sub")
				for {
					select {
					case r := <-subRspChan:
						switch r.Response.(type) {
						case *gnmi.SubscribeResponse_Update:
							responses = append(responses, r)
						case *gnmi.SubscribeResponse_SyncResponse:
							rspChan <- &targetDiffResponse{
								t:  tName,
								rs: responses,
							}
							return
						}
					case err := <-errChan:
						if err == io.EOF {
							rspChan <- &targetDiffResponse{
								t:  tName,
								rs: responses,
							}
							return
						}
						a.logError(err)
						return
					}
				}
			}(tName)
			continue
		}
		a.logError(fmt.Errorf("unknown target %q", tName))
	}
	a.wg.Wait()
	close(rspChan)

	rsps := make([]*targetDiffResponse, 0, numCompares)
	for r := range rspChan {
		rsps = append(rsps, r)
	}
	if len(rsps) == 0 {
		a.Logger.Printf("missing response(s)")
		return fmt.Errorf("missing response(s)")
	}

	for _, cr := range rsps {
		fmt.Fprintf(os.Stderr, "%q vs %q\n", ref, cr.t)
		err = a.responsesDiff(refResponse, cr.rs)
		if err != nil {
			a.logError(err)
		}
	}
	return nil
}

func (a *App) getBasedDiff(ctx context.Context, ref string, compare []string) error {
	getReq, err := a.Config.CreateDiffGetRequest()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var refResponse proto.Message
	numCompares := len(compare)

	go func() {
		defer a.wg.Done()
		a.Logger.Printf("sending gNMI GetRequest: prefix='%v', path='%v', type='%v', encoding='%v', models='%+v', extension='%+v' to %s",
			getReq.Prefix, getReq.Path, getReq.Type, getReq.Encoding, getReq.UseModels, getReq.Extension, ref)
		refResponse, err = a.collector.Get(ctx, ref, getReq)
		if err != nil {
			a.logError(fmt.Errorf("target %q get request failed: %v", ref, err))
			return
		}
	}()
	rspChan := make(chan *targetDiffResponse, numCompares)
	for _, tName := range compare {
		go func(tName string) {
			defer a.wg.Done()
			a.Logger.Printf("sending gNMI GetRequest: prefix='%v', path='%v', type='%v', encoding='%v', models='%+v', extension='%+v' to %s",
				getReq.Prefix, getReq.Path, getReq.Type, getReq.Encoding, getReq.UseModels, getReq.Extension, tName)
			response, err := a.collector.Get(ctx, tName, getReq)
			if err != nil {
				a.logError(fmt.Errorf("target %q get request failed: %v", tName, err))
				return
			}
			rspChan <- &targetDiffResponse{
				t: tName,
				r: response,
			}
		}(tName)
	}
	a.wg.Wait()
	close(rspChan)
	rsps := make([]*targetDiffResponse, 0, numCompares)
	for r := range rspChan {
		rsps = append(rsps, r)
	}
	if len(rsps) == 0 {
		return fmt.Errorf("no responses received")
	}

	sort.Slice(rsps, func(i, j int) bool {
		return rsps[i].t < rsps[j].t
	})
	for _, cr := range rsps {
		fmt.Fprintf(os.Stderr, "%q vs %q\n", ref, cr.t)
		err = a.responsesDiff([]proto.Message{refResponse}, []proto.Message{cr.r})
		if err != nil {
			a.logError(err)
		}
	}
	return nil
}

func (a *App) responsesDiff(r1, r2 []proto.Message) error {
	rs1, err := formatters.ResponsesFlat(r1...)
	if err != nil {
		return err
	}
	rs2, err := formatters.ResponsesFlat(r2...)
	if err != nil {
		return err
	}
	var df diffs
	for p, v := range rs1 {
		if v2, ok := rs2[p]; ok {
			if !reflect.DeepEqual(v, v2) {
				df = append(df, diff{add: false, path: p, value: fmt.Sprintf("%v", v)})
				df = append(df, diff{add: true, path: p, value: fmt.Sprintf("%v", v2)})
			}
			delete(rs2, p)
			continue
		}
		df = append(df, diff{add: false, path: p, value: fmt.Sprintf("%v", v)})
		continue
	}
	for p, v := range rs2 {
		df = append(df, diff{add: true, path: p, value: fmt.Sprintf("%v", v)})
	}
	sort.Slice(df, func(i, j int) bool {
		return df[i].path < df[j].path
	})
	fmt.Println(df)
	return nil
}

type diff struct {
	add   bool
	path  string
	value string
}

type diffs []diff

func (ds diffs) String() string {
	ml := 0
	for _, d := range ds {
		lp := len(d.path)
		if lp > ml {
			ml = lp
		}
	}
	tpl := fmt.Sprintf("%%-%ds", ml)
	sb := new(strings.Builder)
	numDiffs := len(ds)
	for i, d := range ds {
		if d.add {
			sb.WriteString("+\t")
		} else {
			sb.WriteString("-\t")
		}
		sb.WriteString(fmt.Sprintf(tpl, d.path))
		sb.WriteString(": ")
		sb.WriteString(d.value)
		if i < numDiffs-1 {
			sb.WriteString("\n")
		}
	}
	return sb.String()
}
