// Copyright © 2020 Karim Radhouani <medkarimrdi@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/karimra/gnmic/collector"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"gopkg.in/yaml.v2"
)

var vTypes = []string{"json", "json_ietf", "string", "int", "uint", "bool", "decimal", "float", "bytes", "ascii"}

// type setCmdInput struct {
// 	deletes  []string
// 	updates  []string
// 	replaces []string

// 	updatePaths  []string
// 	replacePaths []string

// 	updateFiles  []string
// 	replaceFiles []string

// 	updateValues  []string
// 	replaceValues []string
// }

// var setInput setCmdInput

// setCmd represents the set command
func newSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "run gnmi set on targets",
		Annotations: map[string]string{
			"--delete":       "XPATH",
			"--prefix":       "PREFIX",
			"--replace":      "XPATH",
			"--replace-file": "FILE",
			"--replace-path": "XPATH",
			"--update":       "XPATH",
			"--update-file":  "FILE",
			"--update-path":  "XPATH",
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			cfg.SetLocalFlagsFromFile(cmd)
			return validateSetInput()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cfg.Globals.Format == "event" {
				return fmt.Errorf("format event not supported for Set RPC")
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			setupCloseHandler(cancel)
			targetsConfig, err := cfg.GetTargets()
			if err != nil {
				return fmt.Errorf("failed getting targets config: %v", err)
			}
			if cfg.Globals.Debug {
				logger.Printf("targets: %s", targetsConfig)
			}
			if len(targetsConfig) > 1 {
				fmt.Println("[warning] running set command on multiple targets")
			}
			if coll == nil {
				cfg := &collector.Config{
					Debug:               cfg.Globals.Debug,
					Format:              cfg.Globals.Format,
					TargetReceiveBuffer: cfg.Globals.TargetBufferSize,
					RetryTimer:          cfg.Globals.Retry,
				}

				coll = collector.NewCollector(cfg, targetsConfig,
					collector.WithDialOptions(createCollectorDialOpts()),
					collector.WithLogger(logger),
				)
			} else {
				// prompt mode
				for _, tc := range targetsConfig {
					coll.AddTarget(tc)
				}
			}
			req, err := createSetRequest()
			if err != nil {
				return err
			}
			wg := new(sync.WaitGroup)
			wg.Add(len(coll.Targets))
			lock := new(sync.Mutex)
			for tName := range coll.Targets {
				go setRequest(ctx, tName, req, wg, lock)
			}
			wg.Wait()
			return nil
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			cmd.ResetFlags()
			initSetFlags(cmd)
		},
		SilenceUsage: true,
	}
	initSetFlags(cmd)
	return cmd
}

func setRequest(ctx context.Context, tName string, req *gnmi.SetRequest, wg *sync.WaitGroup, lock *sync.Mutex) {
	defer wg.Done()
	logger.Printf("sending gNMI SetRequest: prefix='%v', delete='%v', replace='%v', update='%v', extension='%v' to %s",
		req.Prefix, req.Delete, req.Replace, req.Update, req.Extension, tName)
	if cfg.Globals.PrintRequest {
		lock.Lock()
		fmt.Fprint(os.Stderr, "Set Request:\n")
		err := printMsg(tName, req)
		if err != nil {
			logger.Printf("error marshaling set request msg: %v", err)
			if !cfg.Globals.Log {
				fmt.Printf("error marshaling set request msg: %v\n", err)
			}
		}
		lock.Unlock()
	}
	response, err := coll.Set(ctx, tName, req)
	if err != nil {
		logger.Printf("error sending set request: %v", err)
		return
	}
	lock.Lock()
	defer lock.Unlock()
	fmt.Fprint(os.Stderr, "Set Response:\n")
	err = printMsg(tName, response)
	if err != nil {
		logger.Printf("error marshaling set response from %s: %v\n", tName, err)
		if !cfg.Globals.Log {
			fmt.Printf("error marshaling set response from %s: %v\n", tName, err)
		}
	}
}

// readFile reads a json or yaml file. the the file is .yaml, converts it to json and returns []byte and an error
func readFile(name string) ([]byte, error) {
	data, err := ioutil.ReadFile(name)
	if err != nil {
		return nil, err
	}
	switch filepath.Ext(name) {
	case ".json":
		return data, err
	case ".yaml", ".yml":
		var out interface{}
		err = yaml.Unmarshal(data, &out)
		if err != nil {
			return nil, err
		}
		newStruct := convert(out)
		newData, err := json.Marshal(newStruct)
		if err != nil {
			return nil, err
		}
		return newData, nil
	default:
		return nil, fmt.Errorf("unsupported file format %s", filepath.Ext(name))
	}
}
func convert(i interface{}) interface{} {
	switch x := i.(type) {
	case map[interface{}]interface{}:
		nm := map[string]interface{}{}
		for k, v := range x {
			nm[k.(string)] = convert(v)
		}
		return nm
	case []interface{}:
		for i, v := range x {
			x[i] = convert(v)
		}
	}
	return i
}

// used to init or reset setCmd flags for gnmic-prompt mode
func initSetFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("prefix", "", "", "set request prefix")

	cmd.Flags().StringArrayVarP(&cfg.LocalFlags.SetDelete, "delete", "", []string{}, "set request path to be deleted")

	cmd.Flags().StringArrayVarP(&cfg.LocalFlags.SetReplace, "replace", "", []string{}, fmt.Sprintf("set request path:::type:::value to be replaced, type must be one of %v", vTypes))
	cmd.Flags().StringArrayVarP(&cfg.LocalFlags.SetUpdate, "update", "", []string{}, fmt.Sprintf("set request path:::type:::value to be updated, type must be one of %v", vTypes))

	cmd.Flags().StringArrayVarP(&cfg.LocalFlags.SetReplacePath, "replace-path", "", []string{}, "set request path to be replaced")
	cmd.Flags().StringArrayVarP(&cfg.LocalFlags.SetUpdatePath, "update-path", "", []string{}, "set request path to be updated")
	cmd.Flags().StringArrayVarP(&cfg.LocalFlags.SetUpdateFile, "update-file", "", []string{}, "set update request value in json/yaml file")
	cmd.Flags().StringArrayVarP(&cfg.LocalFlags.SetReplaceFile, "replace-file", "", []string{}, "set replace request value in json/yaml file")
	cmd.Flags().StringArrayVarP(&cfg.LocalFlags.SetUpdateValue, "update-value", "", []string{}, "set update request value")
	cmd.Flags().StringArrayVarP(&cfg.LocalFlags.SetReplaceValue, "replace-value", "", []string{}, "set replace request value")
	cmd.Flags().StringVarP(&cfg.LocalFlags.SetDelimiter, "delimiter", "", ":::", "set update/replace delimiter between path, type, value")
	cmd.Flags().StringVarP(&cfg.LocalFlags.SetTarget, "target", "", "", "set request target")

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		cfg.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}

func createSetRequest() (*gnmi.SetRequest, error) {
	gnmiPrefix, err := collector.CreatePrefix(cfg.LocalFlags.SetPrefix, cfg.LocalFlags.SetTarget)
	if err != nil {
		return nil, fmt.Errorf("prefix parse error: %v", err)
	}
	if cfg.Globals.Debug {
		logger.Printf("Set input delete: %+v", &cfg.LocalFlags.SetDelete)

		logger.Printf("Set input update: %+v", &cfg.LocalFlags.SetUpdate)
		logger.Printf("Set input update path(s): %+v", &cfg.LocalFlags.SetUpdatePath)
		logger.Printf("Set input update value(s): %+v", &cfg.LocalFlags.SetUpdateValue)
		logger.Printf("Set input update file(s): %+v", &cfg.LocalFlags.SetUpdateFile)

		logger.Printf("Set input replace: %+v", &cfg.LocalFlags.SetReplace)
		logger.Printf("Set input replace path(s): %+v", &cfg.LocalFlags.SetReplacePath)
		logger.Printf("Set input replace value(s): %+v", &cfg.LocalFlags.SetReplaceValue)
		logger.Printf("Set input replace file(s): %+v", &cfg.LocalFlags.SetReplaceFile)
	}

	//
	useUpdateFiles := len(cfg.LocalFlags.SetUpdateFile) > 0 && len(cfg.LocalFlags.SetUpdateValue) == 0
	useReplaceFiles := len(cfg.LocalFlags.SetReplaceFile) > 0 && len(cfg.LocalFlags.SetReplaceValue) == 0
	req := &gnmi.SetRequest{
		Prefix:  gnmiPrefix,
		Delete:  make([]*gnmi.Path, 0, len(cfg.LocalFlags.SetDelete)),
		Replace: make([]*gnmi.Update, 0),
		Update:  make([]*gnmi.Update, 0),
	}
	for _, p := range cfg.LocalFlags.SetDelete {
		gnmiPath, err := collector.ParsePath(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		req.Delete = append(req.Delete, gnmiPath)
	}
	for _, u := range cfg.LocalFlags.SetUpdate {
		singleUpdate := strings.Split(u, cfg.LocalFlags.SetDelimiter)
		if len(singleUpdate) < 3 {
			return nil, fmt.Errorf("invalid inline update format: %s", cfg.LocalFlags.SetUpdate)
		}
		gnmiPath, err := collector.ParsePath(strings.TrimSpace(singleUpdate[0]))
		if err != nil {
			return nil, err
		}
		value := new(gnmi.TypedValue)
		err = setValue(value, singleUpdate[1], singleUpdate[2])
		if err != nil {
			return nil, err
		}
		req.Update = append(req.Update, &gnmi.Update{
			Path: gnmiPath,
			Val:  value,
		})
	}
	for _, r := range cfg.LocalFlags.SetReplace {
		singleReplace := strings.Split(r, cfg.LocalFlags.SetDelimiter)
		if len(singleReplace) < 3 {
			return nil, fmt.Errorf("invalid inline replace format: %s", cfg.LocalFlags.SetReplace)
		}
		gnmiPath, err := collector.ParsePath(strings.TrimSpace(singleReplace[0]))
		if err != nil {
			return nil, err
		}
		value := new(gnmi.TypedValue)
		err = setValue(value, singleReplace[1], singleReplace[2])
		if err != nil {
			return nil, err
		}
		req.Replace = append(req.Replace, &gnmi.Update{
			Path: gnmiPath,
			Val:  value,
		})
	}
	for i, p := range cfg.LocalFlags.SetUpdatePath {
		gnmiPath, err := collector.ParsePath(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		value := new(gnmi.TypedValue)
		if useUpdateFiles {
			var updateData []byte
			updateData, err = readFile(cfg.LocalFlags.SetUpdateFile[i])
			if err != nil {
				logger.Printf("error reading data from file '%s': %v", cfg.LocalFlags.SetUpdateFile[i], err)
				return nil, err
			}
			switch strings.ToUpper(cfg.Globals.Encoding) {
			case "JSON":
				value.Value = &gnmi.TypedValue_JsonVal{
					JsonVal: bytes.Trim(updateData, " \r\n\t"),
				}
			case "JSON_IETF":
				value.Value = &gnmi.TypedValue_JsonIetfVal{
					JsonIetfVal: bytes.Trim(updateData, " \r\n\t"),
				}
			default:
				return nil, fmt.Errorf("encoding: %s not supported together with file values", cfg.Globals.Encoding)
			}
		} else {
			err = setValue(value, strings.ToLower(cfg.Globals.Encoding), cfg.LocalFlags.SetUpdateValue[i])
			if err != nil {
				return nil, err
			}
		}
		req.Update = append(req.Update, &gnmi.Update{
			Path: gnmiPath,
			Val:  value,
		})
	}
	for i, p := range cfg.LocalFlags.SetReplacePath {
		gnmiPath, err := collector.ParsePath(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		value := new(gnmi.TypedValue)
		if useReplaceFiles {
			var replaceData []byte
			replaceData, err = readFile(cfg.LocalFlags.SetReplaceFile[i])
			if err != nil {
				logger.Printf("error reading data from file '%s': %v", cfg.LocalFlags.SetReplaceFile[i], err)
				return nil, err
			}
			switch strings.ToUpper(cfg.Globals.Encoding) {
			case "JSON":
				value.Value = &gnmi.TypedValue_JsonVal{
					JsonVal: bytes.Trim(replaceData, " \r\n\t"),
				}
			case "JSON_IETF":
				value.Value = &gnmi.TypedValue_JsonIetfVal{
					JsonIetfVal: bytes.Trim(replaceData, " \r\n\t"),
				}
			default:
				return nil, fmt.Errorf("encoding: %s not supported together with file values", cfg.Globals.Encoding)
			}
		} else {
			err = setValue(value, strings.ToLower(cfg.Globals.Encoding), cfg.LocalFlags.SetReplaceValue[i])
			if err != nil {
				return nil, err
			}
		}
		req.Replace = append(req.Replace, &gnmi.Update{
			Path: gnmiPath,
			Val:  value,
		})
	}
	return req, nil
}

func setValue(value *gnmi.TypedValue, typ, val string) error {
	var err error
	switch typ {
	case "json":
		buff := new(bytes.Buffer)
		err = json.NewEncoder(buff).Encode(strings.TrimRight(strings.TrimLeft(val, "["), "]"))
		if err != nil {
			return err
		}
		value.Value = &gnmi.TypedValue_JsonVal{
			JsonVal: bytes.Trim(buff.Bytes(), " \r\n\t"),
		}
	case "json_ietf":
		buff := new(bytes.Buffer)
		err = json.NewEncoder(buff).Encode(strings.TrimRight(strings.TrimLeft(val, "["), "]"))
		if err != nil {
			return err
		}
		value.Value = &gnmi.TypedValue_JsonIetfVal{
			JsonIetfVal: bytes.Trim(buff.Bytes(), " \r\n\t"),
		}
	case "ascii":
		value.Value = &gnmi.TypedValue_AsciiVal{
			AsciiVal: val,
		}
	case "bool":
		bval, err := strconv.ParseBool(val)
		if err != nil {
			return err
		}
		value.Value = &gnmi.TypedValue_BoolVal{
			BoolVal: bval,
		}
	case "bytes":
		value.Value = &gnmi.TypedValue_BytesVal{
			BytesVal: []byte(val),
		}
	case "decimal":
		dVal := &gnmi.Decimal64{}
		value.Value = &gnmi.TypedValue_DecimalVal{
			DecimalVal: dVal,
		}
		return fmt.Errorf("decimal type not implemented")
	case "float":
		f, err := strconv.ParseFloat(val, 32)
		if err != nil {
			return err
		}
		value.Value = &gnmi.TypedValue_FloatVal{
			FloatVal: float32(f),
		}
	case "int":
		k, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return err
		}
		value.Value = &gnmi.TypedValue_IntVal{
			IntVal: k,
		}
	case "uint":
		u, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return err
		}
		value.Value = &gnmi.TypedValue_UintVal{
			UintVal: u,
		}
	case "string":
		value.Value = &gnmi.TypedValue_StringVal{
			StringVal: val,
		}
	default:
		return fmt.Errorf("unknown type '%s', must be one of: %v", typ, vTypes)
	}
	return nil
}

func validateSetInput() error {
	if (len(cfg.LocalFlags.SetDelete)+len(cfg.LocalFlags.SetUpdate)+len(cfg.LocalFlags.SetReplace)) == 0 && (len(cfg.LocalFlags.SetUpdatePath)+len(cfg.LocalFlags.SetReplacePath)) == 0 {
		return errors.New("no paths provided")
	}
	if len(cfg.LocalFlags.SetUpdateFile) > 0 && len(cfg.LocalFlags.SetUpdateValue) > 0 {
		fmt.Println(len(cfg.LocalFlags.SetUpdateFile))
		fmt.Println(len(cfg.LocalFlags.SetUpdateValue))
		return errors.New("set update from file and value are not supported in the same command")
	}
	if len(cfg.LocalFlags.SetReplaceFile) > 0 && len(cfg.LocalFlags.SetReplaceValue) > 0 {
		return errors.New("set replace from file and value are not supported in the same command")
	}
	if len(cfg.LocalFlags.SetUpdatePath) != len(cfg.LocalFlags.SetUpdateValue) && len(cfg.LocalFlags.SetUpdatePath) != len(cfg.LocalFlags.SetUpdateFile) {
		return errors.New("missing update value/file or path")
	}
	if len(cfg.LocalFlags.SetReplacePath) != len(cfg.LocalFlags.SetReplaceValue) && len(cfg.LocalFlags.SetReplacePath) != len(cfg.LocalFlags.SetReplaceFile) {
		return errors.New("missing replace value/file or path")
	}
	return nil
}
