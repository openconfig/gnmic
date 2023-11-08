// Copyright 2023 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"fmt"
	"os"

	"github.com/openconfig/ygot/gnmidiff"
	"github.com/openconfig/ygot/gnmidiff/gnmiparse"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// InitDiffSetToNotifsFlags used to init or reset newDiffSetRequestCmd
// flags for gnmic-prompt mode
func (a *App) InitDiffSetToNotifsFlags(cmd *cobra.Command) {
	cmd.ResetFlags()

	cmd.Flags().StringVarP(&a.Config.LocalFlags.DiffSetToNotifsSet, "setrequest", "", "", "reference gNMI SetRequest textproto file for comparing against stored notifications from a device")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.DiffSetToNotifsResponse, "response", "", "", "gNMI Notifications textproto file (can be GetResponse or SubscribeResponse stream) for comparing against the reference SetRequest")
	cmd.MarkFlagRequired("setrequest")
	cmd.MarkFlagRequired("response")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.DiffSetToNotifsFull, "full", "f", false, "show common values")

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", "diff-set-to-notifs", flag.Name), flag)
	})
}

func (a *App) DiffSetToNotifsRunE(cmd *cobra.Command, args []string) error {
	defer a.InitDiffSetRequestFlags(cmd)

	format := gnmidiff.Format{
		Full: a.Config.LocalFlags.DiffSetToNotifsFull,
	}

	setreq, err := gnmiparse.SetRequestFromFile(a.Config.LocalFlags.DiffSetToNotifsSet)
	if err != nil {
		return err
	}

	notifs, err := gnmiparse.NotifsFromFile(a.Config.LocalFlags.DiffSetToNotifsResponse)
	if err != nil {
		return err
	}

	diff, err := gnmidiff.DiffSetRequestToNotifications(setreq, notifs, nil)
	if err != nil {
		return err
	}
	fmt.Fprint(os.Stderr, diff.Format(format))
	return nil
}
