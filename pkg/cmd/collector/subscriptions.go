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
	"path/filepath"
	"sort"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/olekukonko/tablewriter"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/app"
	apiserver "github.com/openconfig/gnmic/pkg/collector/api/server"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func newCollectorSubscriptionsCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "subscriptions",
		Aliases:      []string{"subscription", "sub"},
		Short:        "manage subscriptions",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	cmd.AddCommand(
		newCollectorSubscriptionsListCmd(gApp),
		newCollectorSubscriptionsGetCmd(gApp),
		newCollectorSubscriptionsSetCmd(gApp),
		newCollectorSubscriptionsDeleteCmd(gApp),
	)
	return cmd
}

func newCollectorSubscriptionsListCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "list",
		Aliases:      []string{"ls"},
		Short:        "list subscriptions",
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
			resp, err := client.Get(apiURL + "/api/v1/subscriptions")
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			tb, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to list subscriptions, status code: %d: %s", resp.StatusCode, string(tb))
			}
			// Parse the response
			subs := make([]*apiserver.SubscriptionResponse, 0)
			err = json.Unmarshal(tb, &subs)
			if err != nil {
				return err
			}

			// if len(subs) == 0 {
			// 	fmt.Println("No subscriptions found")
			// 	return nil
			// }

			// Sort by name
			sort.Slice(subs, func(i, j int) bool {
				return subs[i].Name < subs[j].Name
			})
			// Display as horizontal table
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Name", "Prefix", "Paths", "Encoding", "Mode", "Sample Interval", "Targets", "Outputs"})
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

			data := tableFormatSubscriptionsList(subs)
			table.AppendBulk(data)
			table.Render()

			return nil
		},
	}
	return cmd
}

func newCollectorSubscriptionsGetCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "get",
		Aliases:      []string{"g", "show", "sh"},
		Short:        "get a subscription",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("subscription name is required")
			}
			apiURL, err := getAPIServerURL(gApp.Store)
			if err != nil {
				return err
			}
			client, err := getAPIServerClient(gApp.Store)
			if err != nil {
				return err
			}
			resp, err := client.Get(apiURL + "/api/v1/subscriptions/" + name)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			tb, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to get subscription, status code: %d: %s", resp.StatusCode, string(tb))
			}
			// Parse the response
			subs := new(apiserver.SubscriptionResponse)
			err = json.Unmarshal(tb, subs)
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

			data := tableFormatSubscriptionVertical(subs)
			table.AppendBulk(data)
			table.Render()

			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "subscription name")
	return cmd
}

func newCollectorSubscriptionsSetCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "set",
		Aliases: []string{"create", "cr"},
		Short:   "set a subscription",
		RunE: func(cmd *cobra.Command, args []string) error {
			inputConfig, err := cmd.Flags().GetString("input")
			if err != nil {
				return err
			}
			if inputConfig == "" {
				return fmt.Errorf("input file is required")
			}
			subConfig, b, err := readSubscriptionConfigFromFile(inputConfig)
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
			resp, err := client.Post(apiURL+"/api/v1/config/subscriptions", "application/json", bytes.NewBuffer(b))
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				tb, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("failed to create subscription, status code: %d: %s", resp.StatusCode, string(tb))
			}

			fmt.Fprintf(os.Stderr, "Subscription '%s' created successfully\n", subConfig.Name)
			return nil
		},
	}
	cmd.Flags().StringP("input", "i", "", "subscription config file")
	return cmd
}

func newCollectorSubscriptionsDeleteCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"d", "del", "rm"},
		Short:   "delete a subscription",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("subscription name is required")
			}
			apiURL, err := getAPIServerURL(gApp.Store)
			if err != nil {
				return err
			}
			client, err := getAPIServerClient(gApp.Store)
			if err != nil {
				return err
			}
			req, err := http.NewRequest(http.MethodDelete, apiURL+"/api/v1/config/subscriptions/"+name, nil)
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
				return fmt.Errorf("failed to delete subscription, status code: %d: %s", resp.StatusCode, string(tb))
			}

			fmt.Fprintf(os.Stderr, "Subscription '%s' deleted successfully\n", name)
			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "subscription name")
	return cmd
}

// formatSubscriptionMode formats the mode and stream mode
func formatSubscriptionMode(sub *types.SubscriptionConfig) string {
	if sub.Mode == "" {
		sub.Mode = "STREAM"
	}
	if strings.ToLower(sub.Mode) == "stream" && sub.StreamMode == "" {
		sub.StreamMode = "TARGET_DEFINED"
	}
	if strings.ToLower(sub.Mode) == "stream" && sub.StreamMode != "" && len(sub.StreamSubscriptions) == 0 {
		return fmt.Sprintf("%s/%s", strings.ToLower(sub.Mode), strings.ToLower(sub.StreamMode))
	}
	if sub.Mode != "" {
		return strings.ToLower(sub.Mode)
	}
	return "-"
}

func formatSubscriptionConfigVertical(prefix string, cfg *types.SubscriptionConfig) [][]string {
	if cfg == nil {
		return [][]string{}
	}

	data := [][]string{
		{prefix + "Prefix", formatValue(cfg.Prefix)},
		{prefix + "Target", formatValue(cfg.Target)},
		{prefix + "Set Target", fmt.Sprintf("%t", cfg.SetTarget)},
		{prefix + "Paths", formatValue(cfg.Paths)},
		{prefix + "Encoding", formatValue(cfg.Encoding)},
		{prefix + "Mode", formatSubscriptionMode(cfg)},
		{prefix + "Sample Interval", formatValue(cfg.SampleInterval)},
		{prefix + "Heartbeat Interval", formatValue(cfg.HeartbeatInterval)},
		{prefix + "Outputs", formatValue(cfg.Outputs)},
		{prefix + "Models", formatValue(cfg.Models)},
		{prefix + "QoS", formatValue(cfg.Qos)},
		{prefix + "Depth", formatValue(cfg.Depth)},
		{prefix + "Suppress Redundant", fmt.Sprintf("%t", cfg.SuppressRedundant)},
		{prefix + "Updates Only", fmt.Sprintf("%t", cfg.UpdatesOnly)},
	}

	// History section (if present)
	if cfg.History != nil {
		if !cfg.History.Snapshot.IsZero() {
			data = append(data, []string{prefix + "History Snapshot", cfg.History.Snapshot.String()})
		}
		if !cfg.History.Start.IsZero() {
			data = append(data, []string{prefix + "History Start", cfg.History.Start.String()})
		}
		if !cfg.History.End.IsZero() {
			data = append(data, []string{prefix + "History End", cfg.History.End.String()})
		}
	}

	return data
}

func formatStreamSubscriptionConfigVertical(prefix string, cfg *types.SubscriptionConfig) [][]string {
	if cfg == nil {
		return [][]string{}
	}

	data := [][]string{
		{prefix + "Paths", formatValue(cfg.Paths)},
		{prefix + "Mode", formatSubscriptionMode(cfg)},
		{prefix + "Sample Interval", formatValue(cfg.SampleInterval)},
		{prefix + "Heartbeat Interval", formatValue(cfg.HeartbeatInterval)},
	}

	return data
}

func tableFormatSubscriptionVertical(sub *apiserver.SubscriptionResponse) [][]string {
	if sub.Config == nil {
		return [][]string{{"Name", sub.Name}}
	}

	data := [][]string{
		{"Name", sub.Name},
	}

	// Main subscription config
	data = append(data, formatSubscriptionConfigVertical("", sub.Config)...)

	// Targets (top-level only)
	if len(sub.Targets) > 0 {
		targetNames := make([]string, 0, len(sub.Targets))
		for name := range sub.Targets {
			targetNames = append(targetNames, name)
		}
		sort.Strings(targetNames)

		var targetInfo []string
		for _, name := range targetNames {
			targetInfo = append(targetInfo, fmt.Sprintf("%s (%s)", name, sub.Targets[name].State))
		}

		data = append(data, []string{"Targets", strings.Join(targetInfo, "\n")})
	}

	// Stream Subscriptions
	for _, sc := range sub.Config.StreamSubscriptions {
		header := []string{fmt.Sprintf("\tStream Subscription: %s", sc.Name), ""}

		data = append(data, []string{header[0], header[1]})

		// Indent keys to show hierarchy cleanly
		data = append(data, formatStreamSubscriptionConfigVertical("  ", sc)...)
	}

	return data
}

// tableFormatSubscriptionsList formats multiple subscriptions as horizontal table (summary view)
func tableFormatSubscriptionsList(subs []*apiserver.SubscriptionResponse) [][]string {
	data := make([][]string, 0, len(subs))

	for _, sub := range subs {
		if sub.Config == nil {
			continue
		}

		// Add main subscription row
		data = append(data, formatSubscriptionRow(
			sub.Name,
			sub.Config,
			sub.Targets,
		))

		// Add stream-subscriptions (children)
		for i, sc := range sub.Config.StreamSubscriptions {
			// Indent child name for visual grouping
			childName := "↳" + fmt.Sprintf("[%d]", i) + sub.Name

			data = append(data, formatSubscriptionRow(
				childName,
				sc,
				nil, // children have no targets
			))
		}
	}

	return data
}

func formatSubscriptionRow(
	name string,
	cfg *types.SubscriptionConfig,
	targets map[string]*apiserver.TargetStateInfo,
) []string {

	// Paths
	paths := "-"
	if len(cfg.Paths) > 0 {
		paths = strings.Join(cfg.Paths, "\n")
	}

	// Targets summary
	targetsStr := "-"
	if len(targets) > 0 {
		names := make([]string, 0, len(targets))
		for n := range targets {
			names = append(names, n)
		}
		sort.Strings(names)

		running, disabled := 0, 0
		for _, n := range names {
			if targets[n].State == "running" {
				running++
			} else {
				disabled++
			}
		}

		targetsStr = fmt.Sprintf("%d/%d", running, len(targets))
		if disabled > 0 {
			targetsStr += fmt.Sprintf(" (%d disabled)", disabled)
		}
	}

	return []string{
		name,
		formatValue(cfg.Prefix),
		paths,
		formatSubscriptionEncoding(cfg.Encoding),
		formatSubscriptionMode(cfg),
		formatValue(cfg.SampleInterval),
		targetsStr,
		formatValueShort(cfg.Outputs),
	}
}

func readSubscriptionConfigFromFile(filename string) (*types.SubscriptionConfig, []byte, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, nil, err
	}
	cfg := make(map[string]any)
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".json":
		err = json.Unmarshal(b, &cfg)
	case ".yaml", ".yml":
		err = yaml.Unmarshal(b, &cfg)
	default:
		return nil, nil, fmt.Errorf("unsupported file type: %s", filepath.Ext(filename))
	}
	if err != nil {
		return nil, nil, err
	}
	subConfig := new(types.SubscriptionConfig)
	decoder, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{
			DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
			Result:     subConfig,
		})
	if err != nil {
		return nil, nil, err
	}
	err = decoder.Decode(cfg)
	if err != nil {
		return nil, nil, err
	}
	return subConfig, b, nil
}

func formatSubscriptionEncoding(encoding *string) string {
	if encoding == nil {
		return "json"
	}
	if *encoding == "" {
		return "json"
	}
	return formatValue(*encoding)
}
