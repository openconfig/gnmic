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

// InitDiffSetRequestFlags used to init or reset diffSetRequestCmd
// flags for gnmic-prompt mode
func (a *App) InitDiffSetRequestFlags(cmd *cobra.Command) {
	cmd.ResetFlags()

	cmd.Flags().StringVarP(&a.Config.LocalFlags.DiffSetRequestRef, "ref", "", "", "reference gNMI SetRequest textproto file for comparing against the new SetRequest")
	cmd.Flags().StringVarP(&a.Config.LocalFlags.DiffSetRequestNew, "new", "", "", "new gNMI SetRequest textproto file for comparing against the reference SetRequest")
	cmd.MarkFlagRequired("ref")
	cmd.MarkFlagRequired("new")
	cmd.Flags().BoolVarP(&a.Config.LocalFlags.DiffSetRequestFull, "full", "f", false, "show common values between the two SetRequests")

	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		a.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", "diff-setrequest", flag.Name), flag)
	})
}

func (a *App) DiffSetRequestRunE(cmd *cobra.Command, args []string) error {
	defer a.InitDiffSetRequestFlags(cmd)

	format := gnmidiff.Format{
		Full: a.Config.LocalFlags.DiffSetRequestFull,
	}

	srA, err := gnmiparse.SetRequestFromFile(a.Config.LocalFlags.DiffSetRequestRef)
	if err != nil {
		return err
	}

	srB, err := gnmiparse.SetRequestFromFile(a.Config.LocalFlags.DiffSetRequestNew)
	if err != nil {
		return err
	}

	diff, err := gnmidiff.DiffSetRequest(srA, srB, nil)
	if err != nil {
		return err
	}
	fmt.Fprint(os.Stdout, diff.Format(format))
	return nil
}
