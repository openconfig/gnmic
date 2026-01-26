// © 2025 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"

	"github.com/olekukonko/tablewriter"
	"github.com/openconfig/gnmic/pkg/app"
	"github.com/spf13/cobra"
)

func newCollectorInputsCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "inputs",
		Aliases:      []string{"input", "in"},
		Short:        "manage inputs",
		SilenceUsage: true,
	}
	cmd.AddCommand(
		newCollectorInputsListCmd(gApp),
		newCollectorInputsGetCmd(gApp),
		newCollectorInputsSetCmd(gApp),
		newCollectorInputsDeleteCmd(gApp),
	)
	return cmd
}

func newCollectorInputsListCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Aliases:      []string{"ls"},
		Short:        "list inputs",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL, err := getAPIServerURL(gApp.Store)
			if err != nil {
				return err
			}
			client, err := getAPIServerClient(gApp.Store)
			if err != nil {
				return err
			}
			resp, err := client.Get(apiURL + "/api/v1/config/inputs")
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			tb, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to list inputs, status code: %d: %s", resp.StatusCode, string(tb))
			}

			// Parse the response as array of maps
			inputsResponse := make(map[string]interface{}, 0)
			err = json.Unmarshal(tb, &inputsResponse)
			if err != nil {
				return err
			}

			inputs := make([]map[string]interface{}, 0)
			for name, input := range inputsResponse {
				switch input := input.(type) {
				case map[string]any:
					input["name"] = name
					inputs = append(inputs, input)
				default:
					return fmt.Errorf("unknown input type: %T", input)
				}
			}
			// Display as horizontal table
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Name", "Type", "Format", "Event Processors"})
			table.SetAutoWrapText(false)
			table.SetAutoFormatHeaders(true)
			table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
			table.SetAlignment(tablewriter.ALIGN_LEFT)
			table.SetCenterSeparator("")
			table.SetColumnSeparator("")
			table.SetRowSeparator("")
			table.SetHeaderLine(false)
			table.SetBorder(false)
			table.SetTablePadding("\t")
			table.SetNoWhiteSpace(true)

			data := tableFormatInputsList(inputs)
			table.AppendBulk(data)
			table.Render()

			return nil
		},
	}
	return cmd
}

func newCollectorInputsGetCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get",
		Aliases:      []string{"g", "show", "sh"},
		Short:        "get an input",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("input name is required")
			}
			apiURL, err := getAPIServerURL(gApp.Store)
			if err != nil {
				return err
			}
			client, err := getAPIServerClient(gApp.Store)
			if err != nil {
				return err
			}
			resp, err := client.Get(apiURL + "/api/v1/config/inputs/" + name)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			tb, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to get input, status code: %d: %s", resp.StatusCode, string(tb))
			}

			// Parse the response as a map
			input := make(map[string]any)
			err = json.Unmarshal(tb, &input)
			if err != nil {
				return err
			}

			// Display as vertical table (key-value pairs)
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"PARAM", "VALUE"})
			table.SetAutoWrapText(false)
			table.SetAutoFormatHeaders(false)
			table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
			table.SetAlignment(tablewriter.ALIGN_LEFT)
			table.SetCenterSeparator("")
			table.SetColumnSeparator(":")
			table.SetRowSeparator("")
			table.SetHeaderLine(false)
			table.SetBorder(false)
			table.SetTablePadding("\t")
			table.SetNoWhiteSpace(true)
			table.SetColumnAlignment([]int{tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_LEFT})

			data := tableFormatInputVertical(input)
			table.AppendBulk(data)
			table.Render()

			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "input name")
	return cmd
}

func newCollectorInputsSetCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "set",
		Aliases:      []string{"create", "cr"},
		Short:        "set an input",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			inputConfigFile, err := cmd.Flags().GetString("input")
			if err != nil {
				return err
			}
			if inputConfigFile == "" {
				return fmt.Errorf("input file is required")
			}
			b, err := os.ReadFile(inputConfigFile)
			if err != nil {
				return err
			}
			var inputConfig map[string]interface{}
			err = json.Unmarshal(b, &inputConfig)
			if err != nil {
				return err
			}
			client, err := getAPIServerClient(gApp.Store)
			if err != nil {
				return err
			}
			apiURL, err := getAPIServerURL(gApp.Store)
			if err != nil {
				return err
			}
			resp, err := client.Post(apiURL+"/api/v1/config/inputs", "application/json", bytes.NewBuffer(b))
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				tb, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("failed to create input, status code: %d: %s", resp.StatusCode, string(tb))
			}

			inputName := formatValue(inputConfig["name"])
			fmt.Fprintf(os.Stderr, "Input '%s' created successfully\n", inputName)
			return nil
		},
	}
	cmd.Flags().StringP("input", "i", "", "input config file")
	return cmd
}

func newCollectorInputsDeleteCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "delete",
		Aliases:      []string{"d", "del", "rm"},
		Short:        "delete an input",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("input name is required")
			}
			apiURL, err := getAPIServerURL(gApp.Store)
			if err != nil {
				return err
			}
			client, err := getAPIServerClient(gApp.Store)
			if err != nil {
				return err
			}
			req, err := http.NewRequest(http.MethodDelete, apiURL+"/api/v1/config/inputs/"+name, nil)
			if err != nil {
				return err
			}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				tb, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("failed to delete input, status code: %d: %s", resp.StatusCode, string(tb))
			}

			fmt.Fprintln(os.Stderr, "Input deleted successfully")
			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "input name")
	return cmd
}

// tableFormatOutputVertical formats a single output as vertical table (key-value pairs)
func tableFormatInputVertical(input map[string]any) [][]string {
	data := make([][]string, 0)
	// Sort keys for consistent output
	keys := make([]string, 0, len(input))
	for k := range input {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Add each key-value pair
	for _, key := range keys {
		value := input[key]
		formattedValue := formatValue(value)
		data = append(data, []string{key, formattedValue})
	}

	return data
}

// tableFormatInputsList formats multiple outputs as horizontal table (summary view)
func tableFormatInputsList(inputs []map[string]any) [][]string {
	data := make([][]string, 0, len(inputs))

	for _, input := range inputs {
		name := formatValue(input["name"])
		inputType := formatValue(input["type"])
		format := formatValue(input["format"])

		// Handle event-processors
		eventProcessors := "-"
		if ep, ok := input["event-processors"]; ok {
			eventProcessors = formatValueShort(ep)
		}

		data = append(data, []string{
			name,
			inputType,
			format,
			eventProcessors,
		})
	}

	// Sort by name
	sort.Slice(data, func(i, j int) bool {
		return data[i][0] < data[j][0]
	})

	return data
}
