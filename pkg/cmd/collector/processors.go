package collector

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/hairyhenderson/yaml"
	"github.com/olekukonko/tablewriter"
	"github.com/openconfig/gnmic/pkg/app"
	apiserver "github.com/openconfig/gnmic/pkg/collector/api/server"
	"github.com/spf13/cobra"
)

func newCollectorProcessorsCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "processors",
		Aliases:      []string{"processor", "proc"},
		Short:        "manage processors",
		SilenceUsage: true,
	}
	cmd.AddCommand(
		newCollectorProcessorsListCmd(gApp),
		newCollectorProcessorsGetCmd(gApp),
		newCollectorProcessorsSetCmd(gApp),
		newCollectorProcessorsDeleteCmd(gApp),
	)
	return cmd
}

func newCollectorProcessorsListCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "list processors",
		RunE: func(cmd *cobra.Command, args []string) error {
			detailsFlag, err := cmd.Flags().GetBool("details")
			if err != nil {
				return err
			}
			apiURL, err := getAPIServerURL(gApp.Store)
			if err != nil {
				return err
			}
			client, err := getAPIServerClient(gApp.Store)
			if err != nil {
				return err
			}
			resp, err := client.Get(apiURL + "/api/v1/config/processors")
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			tb, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to list processors, status code: %d: %s", resp.StatusCode, string(tb))
			}
			processors := make([]apiserver.ProcessorConfigResponse, 0)
			err = json.Unmarshal(tb, &processors)
			if err != nil {
				return err
			}

			table := tablewriter.NewWriter(os.Stdout)
			if detailsFlag {
				table.SetHeader([]string{"Name", "Type", "Config"})
			} else {
				table.SetHeader([]string{"Name", "Type"})
			}
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

			data := tableFormatProcessorsList(processors, detailsFlag)
			table.AppendBulk(data)
			table.Render()
			return nil
		},
	}
	cmd.Flags().BoolP("details", "", false, "show processors details")
	return cmd
}

func newCollectorProcessorsGetCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get",
		Aliases:      []string{"get"},
		Short:        "get a processor",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("processor name is required")
			}
			apiURL, err := getAPIServerURL(gApp.Store)
			if err != nil {
				return err
			}
			client, err := getAPIServerClient(gApp.Store)
			if err != nil {
				return err
			}
			resp, err := client.Get(apiURL + "/api/v1/config/processors/" + name)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			processorBytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to get processor, status code: %d: %s", resp.StatusCode, string(processorBytes))
			}
			processor := new(apiserver.ProcessorConfigResponse)
			err = json.Unmarshal(processorBytes, processor)
			if err != nil {
				return err
			}

			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Name", "Type", "Config"})
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
			table.SetColumnAlignment([]int{tablewriter.ALIGN_RIGHT, tablewriter.ALIGN_LEFT})
			data := tableFormatProcessorVertical(*processor)
			table.AppendBulk(data)
			table.Render()
			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "processor name")
	cmd.MarkFlagRequired("name")

	return cmd
}

func newCollectorProcessorsSetCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "set",
		Aliases:      []string{"set", "create", "cr"},
		Short:        "set a processor",
		SilenceUsage: true,
	}
	return cmd
}

func newCollectorProcessorsDeleteCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "delete",
		Aliases:      []string{"delete"},
		Short:        "delete a processor",
		SilenceUsage: true,
	}
	return cmd
}

func tableFormatProcessorsList(processors []apiserver.ProcessorConfigResponse, detailsFlag bool) [][]string {
	data := make([][]string, 0, len(processors))
	for _, processor := range processors {
		if detailsFlag {
			data = append(data, []string{processor.Name, processor.Type, formatProcessorConfig(processor.Config)})
		} else {
			data = append(data, []string{processor.Name, processor.Type})
		}
	}
	return data
}

func tableFormatProcessorVertical(processor apiserver.ProcessorConfigResponse) [][]string {
	data := make([][]string, 0, 1)
	data = append(data, []string{processor.Name, processor.Type, formatProcessorConfig(processor.Config)})
	return data
}

func formatProcessorConfig(config any) string {
	b, err := yaml.Marshal(config)
	if err != nil {
		return ""
	}
	return string(b)
}
