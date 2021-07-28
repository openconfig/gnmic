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
