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
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/openconfig/gnmic/pkg/api/types"
	"github.com/openconfig/gnmic/pkg/app"
	apiserver "github.com/openconfig/gnmic/pkg/collector/api/server"
	"github.com/spf13/cobra"
)

func newCollectorSubscriptionsCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "subscriptions",
		Aliases: []string{"subscription", "sub"},
		Short:   "manage subscriptions",
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
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "list subscriptions",
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
			subs := make([]*apiserver.RuntimeSubscriptionResponse, 0)
			err = json.Unmarshal(tb, &subs)
			if err != nil {
				return err
			}

			if len(subs) == 0 {
				fmt.Println("No subscriptions found")
				return nil
			}

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

			data := tableFormatRuntimeSubscriptionsList(subs)
			table.AppendBulk(data)
			table.Render()

			return nil
		},
	}
	return cmd
}

func newCollectorSubscriptionsGetCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Aliases: []string{"g", "show", "sh"},
		Short:   "get a subscription",
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
			subs := make([]*apiserver.RuntimeSubscriptionResponse, 0)
			err = json.Unmarshal(tb, &subs)
			if err != nil {
				return err
			}
			if len(subs) == 0 {
				return fmt.Errorf("subscription %s not found", name)
			}
			sub := subs[0]
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

			data := tableFormatRuntimeSubscriptionVertical(sub)
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
		Use:   "set",
		Short: "set a subscription",
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
			var subConfig *types.SubscriptionConfig
			err = json.Unmarshal(b, &subConfig)
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

			fmt.Printf("Subscription '%s' created successfully\n", subConfig.Name)
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

			fmt.Println("Subscription deleted successfully")
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
	if sub.Mode == "STREAM" && sub.StreamMode == "" {
		sub.StreamMode = "TARGET_DEFINED"
	}
	if strings.ToLower(sub.Mode) == "stream" && sub.StreamMode != "" {
		return fmt.Sprintf("%s/%s", strings.ToLower(sub.Mode), strings.ToLower(sub.StreamMode))
	}
	if sub.Mode != "" {
		return strings.ToLower(sub.Mode)
	}
	return "-"
}

// tableFormatSubscriptionVertical formats a single subscription as vertical table (key-value pairs)
func tableFormatSubscriptionVertical(sub *types.SubscriptionConfig) [][]string {
	data := [][]string{
		{"Name", sub.Name},
		{"Prefix", formatValue(sub.Prefix)},
		{"Target", formatValue(sub.Target)},
		{"Set Target", fmt.Sprintf("%t", sub.SetTarget)},
		{"Paths", formatValue(sub.Paths)},
		{"Encoding", formatValue(sub.Encoding)},
		{"Mode", formatSubscriptionMode(sub)},
		{"Sample Interval", formatValue(sub.SampleInterval)},
		{"Heartbeat Interval", formatValue(sub.HeartbeatInterval)},
		{"Outputs", formatValue(sub.Outputs)},
		{"Models", formatValue(sub.Models)},
		{"QoS", formatValue(sub.Qos)},
		{"Depth", formatValue(sub.Depth)},
		{"Suppress Redundant", fmt.Sprintf("%t", sub.SuppressRedundant)},
		{"Updates Only", fmt.Sprintf("%t", sub.UpdatesOnly)},
	}

	// Add history if present
	if sub.History != nil {
		if !sub.History.Snapshot.IsZero() {
			data = append(data, []string{"History Snapshot", sub.History.Snapshot.String()})
		}
		if !sub.History.Start.IsZero() {
			data = append(data, []string{"History Start", sub.History.Start.String()})
		}
		if !sub.History.End.IsZero() {
			data = append(data, []string{"History End", sub.History.End.String()})
		}
	}

	return data
}

// tableFormatSubscriptionsList formats multiple subscriptions as horizontal table (summary view)
func tableFormatSubscriptionsList(subs []*types.SubscriptionConfig) [][]string {
	data := make([][]string, 0, len(subs))
	for _, sub := range subs {
		// Format paths with newlines for multi-line display
		paths := "-"
		if len(sub.Paths) > 0 {
			paths = strings.Join(sub.Paths, "\n")
		}

		data = append(data, []string{
			sub.Name,
			formatValue(sub.Prefix),
			paths,
			formatValue(sub.Encoding),
			formatSubscriptionMode(sub),
			formatValue(sub.SampleInterval),
			formatValueShort(sub.Outputs),
		})
	}

	// Sort by name
	sort.Slice(data, func(i, j int) bool {
		return data[i][0] < data[j][0]
	})

	return data
}

// tableFormatRuntimeSubscriptionVertical formats a single runtime subscription as vertical table (key-value pairs)
func tableFormatRuntimeSubscriptionVertical(sub *apiserver.RuntimeSubscriptionResponse) [][]string {
	if sub.Config == nil {
		return [][]string{{"Name", sub.Name}}
	}

	data := [][]string{
		{"Name", sub.Name},
		{"Prefix", formatValue(sub.Config.Prefix)},
		{"Target", formatValue(sub.Config.Target)},
		{"Set Target", fmt.Sprintf("%t", sub.Config.SetTarget)},
		{"Paths", formatValue(sub.Config.Paths)},
		{"Encoding", formatValue(sub.Config.Encoding)},
		{"Mode", formatSubscriptionMode(sub.Config)},
		{"Sample Interval", formatValue(sub.Config.SampleInterval)},
		{"Heartbeat Interval", formatValue(sub.Config.HeartbeatInterval)},
		{"Outputs", formatValue(sub.Config.Outputs)},
		{"Models", formatValue(sub.Config.Models)},
		{"QoS", formatValue(sub.Config.Qos)},
		{"Depth", formatValue(sub.Config.Depth)},
		{"Suppress Redundant", fmt.Sprintf("%t", sub.Config.SuppressRedundant)},
		{"Updates Only", fmt.Sprintf("%t", sub.Config.UpdatesOnly)},
	}

	// Add history if present
	if sub.Config.History != nil {
		if !sub.Config.History.Snapshot.IsZero() {
			data = append(data, []string{"History Snapshot", sub.Config.History.Snapshot.String()})
		}
		if !sub.Config.History.Start.IsZero() {
			data = append(data, []string{"History Start", sub.Config.History.Start.String()})
		}
		if !sub.Config.History.End.IsZero() {
			data = append(data, []string{"History End", sub.Config.History.End.String()})
		}
	}

	// Add targets information
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

	return data
}

// tableFormatRuntimeSubscriptionsList formats multiple runtime subscriptions as horizontal table (summary view)
func tableFormatRuntimeSubscriptionsList(subs []*apiserver.RuntimeSubscriptionResponse) [][]string {
	data := make([][]string, 0, len(subs))
	for _, sub := range subs {
		if sub.Config == nil {
			continue
		}

		// Format paths with newlines for multi-line display
		paths := "-"
		if len(sub.Config.Paths) > 0 {
			paths = strings.Join(sub.Config.Paths, "\n")
		}

		// Format targets with state
		targets := "-"
		if len(sub.Targets) > 0 {
			targetNames := make([]string, 0, len(sub.Targets))
			for name := range sub.Targets {
				targetNames = append(targetNames, name)
			}
			sort.Strings(targetNames)
			runningTargets := 0
			disabledTargets := 0
			for _, name := range targetNames {
				state := sub.Targets[name].State
				if state == "running" {
					runningTargets++
				} else {
					disabledTargets++
				}
			}
			targets = fmt.Sprintf("%d/%d", runningTargets, len(sub.Targets))
			if disabledTargets > 0 {
				targets += fmt.Sprintf(" (%d disabled)", disabledTargets)
			}
		}

		data = append(data, []string{
			sub.Name,
			formatValue(sub.Config.Prefix),
			paths,
			formatValue(sub.Config.Encoding),
			formatSubscriptionMode(sub.Config),
			formatValue(sub.Config.SampleInterval),
			targets,
			formatValueShort(sub.Config.Outputs),
		})
	}

	return data
}
