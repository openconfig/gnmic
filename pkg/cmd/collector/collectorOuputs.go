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

func newCollectorOutputsCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "outputs",
		Aliases: []string{"output", "out"},
		Short:   "manage outputs",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	cmd.AddCommand(
		newCollectorOutputsListCmd(gApp),
		newCollectorOutputsGetCmd(gApp),
		newCollectorOutputsCreateCmd(gApp),
		newCollectorOutputsDeleteCmd(gApp),
	)
	return cmd
}

func newCollectorOutputsListCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "list outputs",
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL, err := getAPIServerURL(gApp.Store)
			if err != nil {
				return err
			}
			client, err := getAPIServerClient(gApp.Store)
			if err != nil {
				return err
			}
			resp, err := client.Get(apiURL + "/api/v1/config/outputs")
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			tb, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to list outputs, status code: %d: %s", resp.StatusCode, string(tb))
			}

			// Parse the response as array of maps
			outputsResponse := make(map[string]interface{}, 0)
			err = json.Unmarshal(tb, &outputsResponse)
			if err != nil {
				return err
			}

			if len(outputsResponse) == 0 {
				fmt.Println("No outputs found")
				return nil
			}
			outputs := make([]map[string]interface{}, 0)
			for name, output := range outputsResponse {
				switch output := output.(type) {
				case map[string]any:
					output["name"] = name
					outputs = append(outputs, output)
				default:
					return fmt.Errorf("unknown output type: %T", output)
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

			data := tableFormatOutputsList(outputs)
			table.AppendBulk(data)
			table.Render()

			return nil
		},
	}
	return cmd
}

func newCollectorOutputsGetCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Aliases: []string{"g", "show", "sh"},
		Short:   "get an output",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("output name is required")
			}
			apiURL, err := getAPIServerURL(gApp.Store)
			if err != nil {
				return err
			}
			client, err := getAPIServerClient(gApp.Store)
			if err != nil {
				return err
			}
			resp, err := client.Get(apiURL + "/api/v1/config/outputs/" + name)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			tb, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to get output, status code: %d: %s", resp.StatusCode, string(tb))
			}

			// Parse the response as a map
			output := make(map[string]interface{})
			err = json.Unmarshal(tb, &output)
			if err != nil {
				return err
			}

			// Display as vertical table (key-value pairs)
			table := tablewriter.NewWriter(os.Stdout)
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

			data := tableFormatOutputVertical(output)
			table.AppendBulk(data)
			table.Render()

			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "output name")
	return cmd
}

func newCollectorOutputsCreateCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "create an output",
		RunE: func(cmd *cobra.Command, args []string) error {
			inputConfig, err := cmd.Flags().GetString("input")
			if err != nil {
				return err
			}
			if inputConfig == "" {
				return fmt.Errorf("input file is required")
			}
			b, err := os.ReadFile(inputConfig)
			if err != nil {
				return err
			}
			var outputConfig map[string]interface{}
			err = json.Unmarshal(b, &outputConfig)
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
			resp, err := client.Post(apiURL+"/api/v1/config/outputs", "application/json", bytes.NewBuffer(b))
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				tb, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("failed to create output, status code: %d: %s", resp.StatusCode, string(tb))
			}

			outputName := formatValue(outputConfig["name"])
			fmt.Printf("Output '%s' created successfully\n", outputName)
			return nil
		},
	}
	cmd.Flags().StringP("input", "i", "", "output config file")
	return cmd
}

func newCollectorOutputsDeleteCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"d", "del", "rm"},
		Short:   "delete an output",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("output name is required")
			}
			apiURL, err := getAPIServerURL(gApp.Store)
			if err != nil {
				return err
			}
			client, err := getAPIServerClient(gApp.Store)
			if err != nil {
				return err
			}
			req, err := http.NewRequest(http.MethodDelete, apiURL+"/api/v1/config/outputs/"+name, nil)
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
				return fmt.Errorf("failed to delete output, status code: %d: %s", resp.StatusCode, string(tb))
			}

			fmt.Println("Output deleted successfully")
			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "output name")
	return cmd
}

// tableFormatOutputVertical formats a single output as vertical table (key-value pairs)
func tableFormatOutputVertical(output map[string]interface{}) [][]string {
	data := make([][]string, 0)

	// Sort keys for consistent output
	keys := make([]string, 0, len(output))
	for k := range output {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Add each key-value pair
	for _, key := range keys {
		value := output[key]
		formattedValue := formatValue(value)
		data = append(data, []string{key, formattedValue})
	}

	return data
}

// tableFormatOutputsList formats multiple outputs as horizontal table (summary view)
func tableFormatOutputsList(outputs []map[string]interface{}) [][]string {
	data := make([][]string, 0, len(outputs))

	for _, output := range outputs {
		name := formatValue(output["name"])
		outputType := formatValue(output["type"])
		format := formatValue(output["format"])

		// Handle event-processors
		eventProcessors := "-"
		if ep, ok := output["event-processors"]; ok {
			eventProcessors = formatValueShort(ep)
		}

		data = append(data, []string{
			name,
			outputType,
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
