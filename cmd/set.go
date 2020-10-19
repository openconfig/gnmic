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
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

var vTypes = []string{"json", "json_ietf", "string", "int", "uint", "bool", "decimal", "float", "bytes", "ascii"}

type setCmdInput struct {
	deletes  []string
	updates  []string
	replaces []string

	updatePaths  []string
	replacePaths []string

	updateFiles  []string
	replaceFiles []string

	updateValues  []string
	replaceValues []string
}

var setInput setCmdInput

// setCmd represents the set command
var setCmd = &cobra.Command{
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

	RunE: func(cmd *cobra.Command, args []string) error {
		if viper.GetString("format") == "event" {
			return fmt.Errorf("format event not supported for Set RPC")
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		setupCloseHandler(cancel)
		targets, err := createTargets()
		if err != nil {
			return err
		}
		if len(targets) > 1 {
			fmt.Println("[warning] running set command on multiple targets")
		}
		req, err := createSetRequest()
		if err != nil {
			return err
		}
		wg := new(sync.WaitGroup)
		wg.Add(len(targets))
		lock := new(sync.Mutex)
		for _, tc := range targets {
			go setRequest(ctx, req, collector.NewTarget(tc), wg, lock)
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

func setRequest(ctx context.Context, req *gnmi.SetRequest, target *collector.Target, wg *sync.WaitGroup, lock *sync.Mutex) {
	defer wg.Done()
	opts := createCollectorDialOpts()
	if err := target.CreateGNMIClient(ctx, opts...); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			logger.Printf("failed to create a gRPC client for target '%s', timeout (%s) reached", target.Config.Name, target.Config.Timeout)
			return
		}
		logger.Printf("failed to create a client for target '%s' : %v", target.Config.Name, err)
		return
	}
	logger.Printf("sending gNMI SetRequest: prefix='%v', delete='%v', replace='%v', update='%v', extension='%v' to %s",
		req.Prefix, req.Delete, req.Replace, req.Update, req.Extension, target.Config.Address)
	if viper.GetBool("print-request") {
		lock.Lock()
		fmt.Fprint(os.Stderr, "Set Request:\n")
		err := printMsg(target.Config.Name, req)
		if err != nil {
			logger.Printf("error marshaling set request msg: %v", err)
			if !viper.GetBool("log") {
				fmt.Printf("error marshaling set request msg: %v\n", err)
			}
		}
		lock.Unlock()
	}
	response, err := target.Set(ctx, req)
	if err != nil {
		logger.Printf("error sending set request: %v", err)
		return
	}
	lock.Lock()
	defer lock.Unlock()
	fmt.Fprint(os.Stderr, "Set Response:\n")
	err = printMsg(target.Config.Name, response)
	if err != nil {
		logger.Printf("error marshaling set response from %s: %v\n", target.Config.Name, err)
		if !viper.GetBool("log") {
			fmt.Printf("error marshaling set response from %s: %v\n", target.Config.Name, err)
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

func init() {
	rootCmd.AddCommand(setCmd)
	initSetFlags(setCmd)
}

// used to init or reset setCmd flags for gnmic-prompt mode
func initSetFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("prefix", "", "", "set request prefix")

	cmd.Flags().StringArrayVarP(&setInput.deletes, "delete", "", []string{}, "set request path to be deleted")

	cmd.Flags().StringArrayVarP(&setInput.replaces, "replace", "", []string{}, fmt.Sprintf("set request path:::type:::value to be replaced, type must be one of %v", vTypes))
	cmd.Flags().StringArrayVarP(&setInput.updates, "update", "", []string{}, fmt.Sprintf("set request path:::type:::value to be updated, type must be one of %v", vTypes))

	cmd.Flags().StringArrayVarP(&setInput.replacePaths, "replace-path", "", []string{}, "set request path to be replaced")
	cmd.Flags().StringArrayVarP(&setInput.updatePaths, "update-path", "", []string{}, "set request path to be updated")
	cmd.Flags().StringArrayVarP(&setInput.updateFiles, "update-file", "", []string{}, "set update request value in json/yaml file")
	cmd.Flags().StringArrayVarP(&setInput.replaceFiles, "replace-file", "", []string{}, "set replace request value in json/yaml file")
	cmd.Flags().StringArrayVarP(&setInput.updateValues, "update-value", "", []string{}, "set update request value")
	cmd.Flags().StringArrayVarP(&setInput.replaceValues, "replace-value", "", []string{}, "set replace request value")
	cmd.Flags().StringP("delimiter", "", ":::", "set update/replace delimiter between path, type, value")
	cmd.Flags().StringP("target", "", "", "set request target")

	viper.BindPFlag("set-prefix", cmd.LocalFlags().Lookup("prefix"))
	viper.BindPFlag("set-delete", cmd.LocalFlags().Lookup("delete"))
	viper.BindPFlag("set-replace", cmd.LocalFlags().Lookup("replace"))
	viper.BindPFlag("set-update", cmd.LocalFlags().Lookup("update"))
	viper.BindPFlag("set-update-path", cmd.LocalFlags().Lookup("update-path"))
	viper.BindPFlag("set-replace-path", cmd.LocalFlags().Lookup("replace-path"))
	viper.BindPFlag("set-update-file", cmd.LocalFlags().Lookup("update-file"))
	viper.BindPFlag("set-replace-file", cmd.LocalFlags().Lookup("replace-file"))
	viper.BindPFlag("set-update-value", cmd.LocalFlags().Lookup("update-value"))
	viper.BindPFlag("set-replace-value", cmd.LocalFlags().Lookup("replace-value"))
	viper.BindPFlag("set-delimiter", cmd.LocalFlags().Lookup("delimiter"))
	viper.BindPFlag("set-target", cmd.LocalFlags().Lookup("target"))
}

func createSetRequest() (*gnmi.SetRequest, error) {
	gnmiPrefix, err := collector.CreatePrefix(viper.GetString("set-prefix"), viper.GetString("set-target"))
	if err != nil {
		return nil, fmt.Errorf("prefix parse error: %v", err)
	}
	if viper.GetBool("debug") {
		logger.Printf("setInput struct: %+v", setInput)
	}
	err = validateSetInput()
	if err != nil {
		return nil, err
	}
	delimiter := viper.GetString("set-delimiter")

	//
	useUpdateFiles := len(setInput.updateFiles) > 0 && len(setInput.updateValues) == 0
	useReplaceFiles := len(setInput.replaceFiles) > 0 && len(setInput.replaceValues) == 0
	req := &gnmi.SetRequest{
		Prefix:  gnmiPrefix,
		Delete:  make([]*gnmi.Path, 0, len(setInput.deletes)),
		Replace: make([]*gnmi.Update, 0),
		Update:  make([]*gnmi.Update, 0),
	}
	for _, p := range setInput.deletes {
		gnmiPath, err := collector.ParsePath(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		req.Delete = append(req.Delete, gnmiPath)
	}
	for _, u := range setInput.updates {
		singleUpdate := strings.Split(u, delimiter)
		if len(singleUpdate) < 3 {
			return nil, fmt.Errorf("invalid inline update format: %s", setInput.updates)
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
	for _, r := range setInput.replaces {
		singleReplace := strings.Split(r, delimiter)
		if len(singleReplace) < 3 {
			return nil, fmt.Errorf("invalid inline replace format: %s", setInput.replaces)
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
	for i, p := range setInput.updatePaths {
		gnmiPath, err := collector.ParsePath(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		value := new(gnmi.TypedValue)
		if useUpdateFiles {
			var updateData []byte
			updateData, err = readFile(setInput.updateFiles[i])
			if err != nil {
				logger.Printf("error reading data from file '%s': %v", setInput.updateFiles[i], err)
				return nil, err
			}
			switch strings.ToUpper(viper.GetString("encoding")) {
			case "JSON":
				value.Value = &gnmi.TypedValue_JsonVal{
					JsonVal: bytes.Trim(updateData, " \r\n\t"),
				}
			case "JSON_IETF":
				value.Value = &gnmi.TypedValue_JsonIetfVal{
					JsonIetfVal: bytes.Trim(updateData, " \r\n\t"),
				}
			default:
				return nil, fmt.Errorf("encoding: %s not supported together with file values", viper.GetString("encoding"))
			}
		} else {
			err = setValue(value, "json", setInput.updateValues[i])
			if err != nil {
				return nil, err
			}
		}
		req.Update = append(req.Update, &gnmi.Update{
			Path: gnmiPath,
			Val:  value,
		})
	}
	for i, p := range setInput.replacePaths {
		gnmiPath, err := collector.ParsePath(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		value := new(gnmi.TypedValue)
		if useReplaceFiles {
			var replaceData []byte
			replaceData, err = readFile(setInput.replaceFiles[i])
			if err != nil {
				logger.Printf("error reading data from file '%s': %v", setInput.replaceFiles[i], err)
				return nil, err
			}
			switch strings.ToUpper(viper.GetString("encoding")) {
			case "JSON":
				value.Value = &gnmi.TypedValue_JsonVal{
					JsonVal: bytes.Trim(replaceData, " \r\n\t"),
				}
			case "JSON_IETF":
				value.Value = &gnmi.TypedValue_JsonIetfVal{
					JsonIetfVal: bytes.Trim(replaceData, " \r\n\t"),
				}
			default:
				return nil, fmt.Errorf("encoding: %s not supported together with file values", viper.GetString("encoding"))
			}
		} else {
			err = setValue(value, "json", setInput.replaceValues[i])
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
	if (len(setInput.deletes)+len(setInput.updates)+len(setInput.replaces)) == 0 && (len(setInput.updatePaths)+len(setInput.replacePaths)) == 0 {
		return errors.New("no paths provided")
	}
	if len(setInput.updateFiles) > 0 && len(setInput.updateValues) > 0 {
		fmt.Println(len(setInput.updateFiles))
		fmt.Println(len(setInput.updateValues))
		return errors.New("set update from file and value are not supported in the same command")
	}
	if len(setInput.replaceFiles) > 0 && len(setInput.replaceValues) > 0 {
		return errors.New("set replace from file and value are not supported in the same command")
	}
	if len(setInput.updatePaths) != len(setInput.updateValues) && len(setInput.updatePaths) != len(setInput.updateFiles) {
		return errors.New("missing update value/file or path")
	}
	if len(setInput.replacePaths) != len(setInput.replaceValues) && len(setInput.replacePaths) != len(setInput.replaceFiles) {
		return errors.New("missing replace value/file or path")
	}
	return nil
}
