// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"fmt"
	"strings"

	"github.com/openconfig/gnmi/proto/gnmi"
)

func (a *App) printCapResponse(printPrefix string, msg *gnmi.CapabilityResponse) {
	sb := strings.Builder{}
	sb.WriteString("gNMI version: ")
	sb.WriteString(msg.GNMIVersion)
	sb.WriteString("\n")
	if a.Config.LocalFlags.CapabilitiesVersion {
		return
	}
	sb.WriteString("supported models:\n")
	for _, sm := range msg.SupportedModels {
		sb.WriteString("  - ")
		sb.WriteString(sm.GetName())
		sb.WriteString(", ")
		sb.WriteString(sm.GetOrganization())
		sb.WriteString(", ")
		sb.WriteString(sm.GetVersion())
		sb.WriteString("\n")
	}
	sb.WriteString("supported encodings:\n")
	for _, se := range msg.SupportedEncodings {
		sb.WriteString("  - ")
		sb.WriteString(se.String())
		sb.WriteString("\n")
	}
	fmt.Fprintf(a.out, "%s\n", indent(printPrefix, sb.String()))
}

func indent(prefix, s string) string {
	if prefix == "" {
		return s
	}
	prefix = "\n" + strings.TrimRight(prefix, "\n")
	lines := strings.Split(s, "\n")
	return strings.TrimLeft(fmt.Sprintf("%s%s", prefix, strings.Join(lines, prefix)), "\n")
}
