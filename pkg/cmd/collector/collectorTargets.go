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

func newCollectorTargetsCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "targets",
		Aliases: []string{"target", "tg"},
		Short:   "manage targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}
	cmd.AddCommand(newCollectorTargetsListCmd(gApp))
	cmd.AddCommand(newCollectorTargetsGetCmd(gApp))
	cmd.AddCommand(newCollectorTargetsSetCmd(gApp))
	cmd.AddCommand(newCollectorTargetsDeleteCmd(gApp))
	return cmd
}

func newCollectorTargetsListCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "list targets",
		RunE: func(cmd *cobra.Command, args []string) error {
			apiURL, err := getAPIServerURL(gApp.Store)
			if err != nil {
				return err
			}
			// TODO: TLS
			client, err := getAPIServerClient(gApp.Store)
			if err != nil {
				return err
			}
			resp, err := client.Get(apiURL + "/api/v1/targets")
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			tb, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to list targets, status code: %d: %s", resp.StatusCode, string(tb))
			}

			// Parse the response
			tc := make([]*apiserver.TargetResponse, 0)
			err = json.Unmarshal(tb, &tc)
			if err != nil {
				return err
			}

			if len(tc) == 0 {
				fmt.Println("No targets found")
				return nil
			}

			// Display as horizontal table
			table := tablewriter.NewWriter(os.Stdout)
			table.SetHeader([]string{"Name", "Address", "Username", "State", "Subscriptions", "Outputs", "Insecure", "Skip Verify"})
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

			data := tableFormatTargetsList(tc)
			table.AppendBulk(data)
			table.Render()

			return nil
		},
	}
	return cmd
}

func newCollectorTargetsGetCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get",
		Aliases: []string{"g", "show", "sh"},
		Short:   "get a target",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("target name is required")
			}
			apiURL, err := getAPIServerURL(gApp.Store)
			if err != nil {
				return err
			}
			client, err := getAPIServerClient(gApp.Store)
			if err != nil {
				return err
			}
			resp, err := client.Get(apiURL + "/api/v1/targets/" + name)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			tb, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to get target, status code: %d: %s", resp.StatusCode, string(tb))
			}

			tc := make([]*apiserver.TargetResponse, 0)
			err = json.Unmarshal(tb, &tc)
			if err != nil {
				return err
			}
			if len(tc) == 0 {
				return fmt.Errorf("no targets found")
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

			data := tableFormatTargetVertical(tc[0])
			table.AppendBulk(data)
			table.Render()

			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "target name")
	return cmd
}

func newCollectorTargetsSetCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set",
		Short: "set a target",
		RunE: func(cmd *cobra.Command, args []string) error {
			inputConfig, err := cmd.Flags().GetString("input-config")
			if err != nil {
				return err
			}
			b, err := os.ReadFile(inputConfig)
			if err != nil {
				return err
			}
			var targetConfig *types.TargetConfig
			err = json.Unmarshal(b, &targetConfig)
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
			resp, err := client.Post(apiURL+"/api/v1/targets", "application/json", bytes.NewBuffer(b))
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			tb, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			rs := bytes.NewBuffer(nil)
			json.Indent(rs, tb, "", "  ")
			fmt.Println(rs.String())
			return nil
		},
	}
	cmd.Flags().StringP("input", "i", "", "target file input")
	return cmd
}

func newCollectorTargetsDeleteCmd(gApp *app.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete",
		Aliases: []string{"d", "del", "rm"},
		Short:   "delete a target",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("target name is required")
			}
			apiURL, err := getAPIServerURL(gApp.Store)
			if err != nil {
				return err
			}
			client, err := getAPIServerClient(gApp.Store)
			if err != nil {
				return err
			}
			req, err := http.NewRequest(http.MethodDelete, apiURL+"/api/v1/targets/"+name, nil)
			if err != nil {
				return err
			}
			resp, err := client.Do(req)
			if err != nil {
				return err
			}
			resp.Body.Close()
			fmt.Println("target deleted")
			fmt.Println(resp.Status)
			tb, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			rs := bytes.NewBuffer(nil)
			json.Indent(rs, tb, "", "  ")
			fmt.Println(rs.String())
			return nil
		},
	}
	cmd.Flags().StringP("name", "n", "", "target name")
	return cmd
}

// formatValue formats any value based on its type for table display
func formatValue(v interface{}) string {
	if v == nil {
		return "-"
	}

	switch val := v.(type) {
	case *string:
		if val == nil {
			return "-"
		}
		if *val == "" {
			return "-"
		}
		return *val
	case string:
		if val == "" {
			return "-"
		}
		return val
	case *bool:
		if val == nil {
			return "-"
		}
		return fmt.Sprintf("%t", *val)
	case bool:
		return fmt.Sprintf("%t", val)
	case *int:
		if val == nil {
			return "-"
		}
		return fmt.Sprintf("%d", *val)
	case int:
		if val == 0 {
			return "-"
		}
		return fmt.Sprintf("%d", val)
	case uint:
		if val == 0 {
			return "-"
		}
		return fmt.Sprintf("%d", val)
	case []string:
		if len(val) == 0 {
			return "-"
		}
		return strings.Join(val, ", ")
	case map[string]string:
		if len(val) == 0 {
			return "-"
		}
		var parts []string
		for k, v := range val {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
		sort.Strings(parts)
		return strings.Join(parts, ", ")
	case map[string]*apiserver.SubscriptionResponse:
		if len(val) == 0 {
			return "-"
		}
		var parts []string
		for k, v := range val {
			parts = append(parts, fmt.Sprintf("%s(%s)", k, v.State))
		}
		sort.Strings(parts)
		return strings.Join(parts, ", ")
	default:
		str := fmt.Sprintf("%v", val)
		if str == "" || str == "0s" || str == "<nil>" {
			return "-"
		}
		return str
	}
}

// formatValueShort formats value for list view (shorter version)
func formatValueShort(v interface{}) string {
	if v == nil {
		return "-"
	}

	switch val := v.(type) {
	case []string:
		if len(val) == 0 {
			return "-"
		}
		return fmt.Sprintf("%d", len(val))
	case map[string]string:
		if len(val) == 0 {
			return "-"
		}
		return fmt.Sprintf("%d", len(val))
	case map[string]*apiserver.SubscriptionResponse:
		if len(val) == 0 {
			return "-"
		}
		return fmt.Sprintf("%d", len(val))
	default:
		return formatValue(val)
	}
}

// tableFormatTargetVertical formats a single target as vertical table (key-value pairs)
func tableFormatTargetVertical(target *apiserver.TargetResponse) [][]string {
	cfg := target.Config
	data := [][]string{
		{"Name", target.Name},
		{"State", target.State},
		{"Address", formatValue(cfg.Address)},
		{"Username", formatValue(cfg.Username)},
		{"Password", formatValue(cfg.Password)},
		{"Auth Scheme", formatValue(cfg.AuthScheme)},
		{"Timeout", formatValue(cfg.Timeout)},
		{"Insecure", formatValue(cfg.Insecure)},
		{"Skip Verify", formatValue(cfg.SkipVerify)},
		{"TLS CA", formatValue(cfg.TLSCA)},
		{"TLS Cert", formatValue(cfg.TLSCert)},
		{"TLS Key", formatValue(cfg.TLSKey)},
		{"TLS Server Name", formatValue(cfg.TLSServerName)},
		{"TLS Min Version", formatValue(cfg.TLSMinVersion)},
		{"TLS Max Version", formatValue(cfg.TLSMaxVersion)},
		{"TLS Version", formatValue(cfg.TLSVersion)},
		{"Log TLS Secret", formatValue(cfg.LogTLSSecret)},
		{"Subscriptions", formatValue(target.Subscriptions)},
		{"Outputs", formatValue(cfg.Outputs)},
		{"Buffer Size", formatValue(cfg.BufferSize)},
		{"Retry Timer", formatValue(cfg.RetryTimer)},
		{"Token", formatValue(cfg.Token)},
		{"Proxy", formatValue(cfg.Proxy)},
		{"Encoding", formatValue(cfg.Encoding)},
		{"Tags", formatValue(cfg.Tags)},
		{"Event Tags", formatValue(cfg.EventTags)},
		{"Metadata", formatValue(cfg.Metadata)},
		{"Gzip", formatValue(cfg.Gzip)},
		{"Proto Files", formatValue(cfg.ProtoFiles)},
		{"Proto Dirs", formatValue(cfg.ProtoDirs)},
		{"Cipher Suites", formatValue(cfg.CipherSuites)},
		{"TCP Keepalive", formatValue(cfg.TCPKeepalive)},
		{"GRPC Keepalive", formatValue(cfg.GRPCKeepalive)},
		{"Tunnel Target Type", formatValue(cfg.TunnelTargetType)},
	}
	return data
}

// tableFormatTargetsList formats multiple targets as horizontal table (summary view)
func tableFormatTargetsList(targets []*apiserver.TargetResponse) [][]string {
	data := make([][]string, 0, len(targets))
	for _, target := range targets {
		data = append(data, []string{
			target.Name,
			formatValue(target.Config.Address),
			formatValue(target.Config.Username),
			target.State,
			formatValueShort(target.Subscriptions),
			formatValueShort(target.Config.Outputs),
			formatValue(target.Config.Insecure),
			formatValue(target.Config.SkipVerify),
		})
	}
	// Sort by name
	sort.Slice(data, func(i, j int) bool {
		return data[i][0] < data[j][0]
	})
	return data
}
