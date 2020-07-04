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
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/gnxi/utils/xpath"
	"github.com/karimra/gnmic/collector"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"google.golang.org/protobuf/encoding/prototext"
	"gopkg.in/yaml.v2"
)

var vTypes = []string{"json", "json_ietf", "string", "int", "uint", "bool", "decimal", "float", "bytes", "ascii"}

type setRspMsg struct {
	Source    string             `json:"source,omitempty"`
	Timestamp int64              `json:"timestamp,omitempty"`
	Time      time.Time          `json:"time,omitempty"`
	Prefix    string             `json:"prefix,omitempty"`
	Results   []*updateResultMsg `json:"results,omitempty"`
}

type updateResultMsg struct {
	Operation string `json:"operation,omitempty"`
	Path      string `json:"path,omitempty"`
}

type setReqMsg struct {
	Prefix  string       `json:"prefix,omitempty"`
	Delete  []string     `json:"delete,omitempty"`
	Replace []*updateMsg `json:"replace,omitempty"`
	Update  []*updateMsg `json:"update,omitempty"`
	// extension is not implemented
}

type updateMsg struct {
	Path string `json:"path,omitempty"`
	Val  string `json:"val,omitempty"`
}

// setCmd represents the set command
var setCmd = &cobra.Command{
	Use:   "set",
	Short: "run gnmi set on targets",

	RunE: func(cmd *cobra.Command, args []string) error {
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
}

func setRequest(ctx context.Context, req *gnmi.SetRequest, target *collector.Target, wg *sync.WaitGroup, lock *sync.Mutex) {
	defer wg.Done()
	opts := createCollectorDialOpts()
	err := target.CreateGNMIClient(ctx, opts...)
	if err != nil {
		if err == context.DeadlineExceeded {
			logger.Printf("failed to create a gRPC client for target '%s' timeout (%s) reached: %v", target.Config.Name, target.Config.Timeout, err)
			return
		}
		logger.Printf("failed to create a client for target '%s' : %v", target.Config.Name, err)
		return
	}

	printPrefix := ""
	if numTargets() > 1 && !viper.GetBool("no-prefix") {
		printPrefix = fmt.Sprintf("[%s] ", target.Config.Address)
	}
	lock.Lock()
	defer lock.Unlock()
	if viper.GetBool("set-print-request") {
		printSetRequest(printPrefix, req)
	}
	logger.Printf("sending gNMI SetRequest: prefix='%v', delete='%v', replace='%v', update='%v', extension='%v' to %s",
		req.Prefix, req.Delete, req.Replace, req.Update, req.Extension, target.Config.Address)
	response, err := target.Set(ctx, req)
	if err != nil {
		logger.Printf("error sending set request: %v", err)
		return
	}
	printSetResponse(printPrefix, target.Config.Address, response)
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
func printSetRequest(printPrefix string, request *gnmi.SetRequest) {
	if viper.GetString("format") == "textproto" {
		fmt.Printf("%s\n", indent("  ", prototext.Format(request)))
		return
	}
	fmt.Printf("%sSet Request: \n", printPrefix)
	req := new(setReqMsg)
	req.Prefix = gnmiPathToXPath(request.Prefix)
	req.Delete = make([]string, 0, len(request.Delete))
	req.Replace = make([]*updateMsg, 0, len(request.Replace))
	req.Update = make([]*updateMsg, 0, len(request.Update))

	for _, del := range request.Delete {
		p := gnmiPathToXPath(del)
		req.Delete = append(req.Delete, p)
	}

	for _, upd := range request.Replace {
		updMsg := new(updateMsg)
		updMsg.Path = gnmiPathToXPath(upd.Path)
		updMsg.Val = upd.Val.String()
		req.Replace = append(req.Replace, updMsg)
	}

	for _, upd := range request.Update {
		updMsg := new(updateMsg)
		updMsg.Path = gnmiPathToXPath(upd.Path)
		updMsg.Val = upd.Val.String()
		req.Update = append(req.Update, updMsg)
	}

	b, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		fmt.Println("failed marshaling the set request", err)
		return
	}
	fmt.Println(string(b))
}
func printSetResponse(printPrefix, address string, response *gnmi.SetResponse) {
	if viper.GetString("format") == "textproto" {
		fmt.Printf("%s\n", indent(printPrefix, prototext.Format(response)))
		return
	}
	rsp := new(setRspMsg)
	rsp.Prefix = gnmiPathToXPath(response.Prefix)
	rsp.Timestamp = response.Timestamp
	rsp.Time = time.Unix(0, response.Timestamp)
	rsp.Results = make([]*updateResultMsg, 0, len(response.Response))
	rsp.Source = address
	for _, u := range response.Response {
		r := new(updateResultMsg)
		r.Operation = u.Op.String()
		r.Path = gnmiPathToXPath(u.Path)
		rsp.Results = append(rsp.Results, r)
	}
	b, err := json.MarshalIndent(rsp, "", "  ")
	if err != nil {
		fmt.Printf("failed marshaling the set response from '%s': %v", address, err)
		return
	}
	fmt.Println(string(b))
}

func init() {
	rootCmd.AddCommand(setCmd)

	setCmd.Flags().StringP("prefix", "", "", "set request prefix")

	setCmd.Flags().StringSliceP("delete", "", []string{}, "set request path to be deleted")

	setCmd.Flags().StringSliceP("replace", "", []string{}, fmt.Sprintf("set request path:::type:::value to be replaced, type must be one of %v", vTypes))
	setCmd.Flags().StringSliceP("update", "", []string{}, fmt.Sprintf("set request path:::type:::value to be updated, type must be one of %v", vTypes))

	setCmd.Flags().StringSliceP("replace-path", "", []string{""}, "set request path to be replaced")
	setCmd.Flags().StringSliceP("update-path", "", []string{""}, "set request path to be updated")
	setCmd.Flags().StringSliceP("update-file", "", []string{""}, "set update request value in json file")
	setCmd.Flags().StringSliceP("replace-file", "", []string{""}, "set replace request value in json file")
	setCmd.Flags().StringSliceP("update-value", "", []string{""}, "set update request value")
	setCmd.Flags().StringSliceP("replace-value", "", []string{""}, "set replace request value")
	setCmd.Flags().StringP("delimiter", "", ":::", "set update/replace delimiter between path,type,value")
	setCmd.Flags().BoolP("print-request", "", false, "print set request as well as the response")

	viper.BindPFlag("set-prefix", setCmd.LocalFlags().Lookup("prefix"))
	viper.BindPFlag("set-delete", setCmd.LocalFlags().Lookup("delete"))
	viper.BindPFlag("set-replace", setCmd.LocalFlags().Lookup("replace"))
	viper.BindPFlag("set-update", setCmd.LocalFlags().Lookup("update"))
	viper.BindPFlag("set-update-path", setCmd.LocalFlags().Lookup("update-path"))
	viper.BindPFlag("set-replace-path", setCmd.LocalFlags().Lookup("replace-path"))
	viper.BindPFlag("set-update-file", setCmd.LocalFlags().Lookup("update-file"))
	viper.BindPFlag("set-replace-file", setCmd.LocalFlags().Lookup("replace-file"))
	viper.BindPFlag("set-update-value", setCmd.LocalFlags().Lookup("update-value"))
	viper.BindPFlag("set-replace-value", setCmd.LocalFlags().Lookup("replace-value"))
	viper.BindPFlag("set-delimiter", setCmd.LocalFlags().Lookup("delimiter"))
	viper.BindPFlag("set-print-request", setCmd.LocalFlags().Lookup("print-request"))
}

func createSetRequest() (*gnmi.SetRequest, error) {
	prefix := viper.GetString("set-prefix")
	gnmiPrefix, err := xpath.ToGNMIPath(prefix)
	if err != nil {
		return nil, err
	}
	deletes := viper.GetStringSlice("set-delete")
	updates := viper.GetStringSlice("set-update")
	replaces := viper.GetStringSlice("set-replace")

	updatePaths := viper.GetStringSlice("set-update-path")
	replacePaths := viper.GetStringSlice("set-replace-path")

	updateFiles := viper.GetStringSlice("set-update-file")
	replaceFiles := viper.GetStringSlice("set-replace-file")

	updateValues := viper.GetStringSlice("set-update-value")
	replaceValues := viper.GetStringSlice("set-replace-value")

	delimiter := viper.GetString("set-delimiter")
	if (len(deletes)+len(updates)+len(replaces)) == 0 && (len(updatePaths)+len(replacePaths)) == 0 {
		return nil, errors.New("no paths provided")
	}
	if len(updateFiles) > 0 && len(updateValues) > 0 {
		return nil, errors.New("set update from file and value are not supported in the same command")
	}
	if len(replaceFiles) > 0 && len(replaceValues) > 0 {
		return nil, errors.New("set replace from file and value are not supported in the same command")
	}
	if len(updatePaths) != len(updateValues) && len(updatePaths) != len(updateFiles) {
		return nil, errors.New("missing update value/file or path")
	}
	if len(replacePaths) != len(replaceValues) && len(replacePaths) != len(replaceFiles) {
		return nil, errors.New("missing replace value/file or path")
	}
	//
	useUpdateFiles := len(updateFiles) > 0 && len(updateValues) == 0
	useReplaceFiles := len(replaceFiles) > 0 && len(replaceValues) == 0
	req := &gnmi.SetRequest{
		Prefix:  gnmiPrefix,
		Delete:  make([]*gnmi.Path, 0, len(deletes)),
		Replace: make([]*gnmi.Update, 0),
		Update:  make([]*gnmi.Update, 0),
	}
	for _, p := range deletes {
		gnmiPath, err := xpath.ToGNMIPath(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		req.Delete = append(req.Delete, gnmiPath)
	}
	for _, u := range updates {
		singleUpdate := strings.Split(u, delimiter)
		if len(singleUpdate) < 3 {
			return nil, fmt.Errorf("invalid inline update format: %s", updates)
		}
		gnmiPath, err := xpath.ToGNMIPath(strings.TrimSpace(singleUpdate[0]))
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
	for _, r := range replaces {
		singleReplace := strings.Split(r, delimiter)
		if len(singleReplace) < 3 {
			return nil, fmt.Errorf("invalid inline replace format: %s", updates)
		}
		gnmiPath, err := xpath.ToGNMIPath(strings.TrimSpace(singleReplace[0]))
		if err != nil {
			return nil, err
		}
		value := new(gnmi.TypedValue)
		err = setValue(value, singleReplace[1], singleReplace[2])
		if err != nil {
			return nil, err
		}
		req.Replace = append(req.Update, &gnmi.Update{
			Path: gnmiPath,
			Val:  value,
		})
	}
	for i, p := range updatePaths {
		gnmiPath, err := xpath.ToGNMIPath(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		value := new(gnmi.TypedValue)
		if useUpdateFiles {
			var updateData []byte
			updateData, err = readFile(updateFiles[i])
			if err != nil {
				logger.Printf("error reading data from file '%s': %v", updateFiles[i], err)
				continue
			}
			value.Value = &gnmi.TypedValue_JsonVal{
				JsonVal: bytes.Trim(updateData, " \r\n\t"),
			}
		} else {
			err = setValue(value, "json", updateValues[i])
			if err != nil {
				return nil, err
			}
		}
		req.Update = append(req.Update, &gnmi.Update{
			Path: gnmiPath,
			Val:  value,
		})
	}
	for i, p := range replacePaths {
		gnmiPath, err := xpath.ToGNMIPath(strings.TrimSpace(p))
		if err != nil {
			return nil, err
		}
		value := new(gnmi.TypedValue)
		if useReplaceFiles {
			var replaceData []byte
			replaceData, err = readFile(replaceFiles[i])
			if err != nil {
				logger.Printf("error reading data from file '%s': %v", replaceFiles[i], err)
				continue
			}
			value.Value = &gnmi.TypedValue_JsonVal{
				JsonVal: bytes.Trim(replaceData, " \r\n\t"),
			}
		} else {
			err = setValue(value, "json", replaceValues[i])
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
