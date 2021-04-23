// Copyright © 2020 Karim Radhouani <medkarimrdi@gmail.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/karimra/gnmic/config"
	"github.com/openconfig/goyang/pkg/yang"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// pathCmd represents the path command
func newPathCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "path",
		Short: "generate gnmi or xpath style from yang file",
		Annotations: map[string]string{
			"--file": "YANG",
			"--dir":  "DIR",
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			gApp.Config.SetLocalFlagsFromFile(cmd)
			if gApp.Config.LocalFlags.PathPathType != "xpath" && gApp.Config.LocalFlags.PathPathType != "gnmi" {
				return fmt.Errorf("path-type must be one of 'xpath' or 'gnmi'")
			}
			gApp.Config.LocalFlags.PathDir = config.SanitizeArrayFlagValue(gApp.Config.LocalFlags.PathDir)
			gApp.Config.LocalFlags.PathFile = config.SanitizeArrayFlagValue(gApp.Config.LocalFlags.PathFile)
			gApp.Config.LocalFlags.PathExclude = config.SanitizeArrayFlagValue(gApp.Config.LocalFlags.PathExclude)

			var err error
			gApp.Config.LocalFlags.PathDir, err = resolveGlobs(gApp.Config.LocalFlags.PathDir)
			if err != nil {
				return err
			}
			gApp.Config.LocalFlags.PathFile, err = resolveGlobs(gApp.Config.LocalFlags.PathFile)
			if err != nil {
				return err
			}
			for _, dirpath := range gApp.Config.LocalFlags.PathDir {
				expanded, err := yang.PathsWithModules(dirpath)
				if err != nil {
					return err
				}
				if gApp.Config.Debug {
					for _, fdir := range expanded {
						gApp.Logger.Printf("adding %s to YANG paths", fdir)
					}
				}
				yang.AddPath(expanded...)
			}
			yfiles, err := findYangFiles(gApp.Config.LocalFlags.PathFile)
			if err != nil {
				return err
			}
			gApp.Config.LocalFlags.PathFile = make([]string, 0, len(yfiles))
			gApp.Config.LocalFlags.PathFile = append(gApp.Config.LocalFlags.PathFile, yfiles...)
			if gApp.Config.Debug {
				for _, file := range gApp.Config.LocalFlags.PathFile {
					gApp.Logger.Printf("loading %s file", file)
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return gApp.PathCmdRun(
				gApp.Config.LocalFlags.PathDir,
				gApp.Config.LocalFlags.PathFile,
				gApp.Config.LocalFlags.PathExclude,
				gApp.Config.LocalFlags.PathSearch,
				gApp.Config.LocalFlags.PathWithPrefix,
				gApp.Config.LocalFlags.PathWithTypes,
				gApp.Config.LocalFlags.PathPathType,
			)
		},
		PostRun: func(cmd *cobra.Command, args []string) {
			cmd.ResetFlags()
			initPathFlags(cmd)
		},
		SilenceUsage: true,
	}
	initPathFlags(cmd)
	return cmd
}

// used to init or reset pathCmd flags for gnmic-prompt mode
func initPathFlags(cmd *cobra.Command) {
	cmd.Flags().StringArrayVarP(&gApp.Config.LocalFlags.PathFile, "file", "", []string{}, "yang files to get the paths")
	cmd.MarkFlagRequired("file")
	cmd.Flags().StringArrayVarP(&gApp.Config.LocalFlags.PathExclude, "exclude", "", []string{}, "yang modules to be excluded from path generation")
	cmd.Flags().StringArrayVarP(&gApp.Config.LocalFlags.PathDir, "dir", "", []string{}, "directories to search yang includes and imports")
	cmd.Flags().StringVarP(&gApp.Config.LocalFlags.PathPathType, "path-type", "", "xpath", "path type xpath or gnmi")
	cmd.Flags().BoolVarP(&gApp.Config.LocalFlags.PathWithPrefix, "with-prefix", "", false, "include module/submodule prefix in path elements")
	cmd.Flags().BoolVarP(&gApp.Config.LocalFlags.PathWithTypes, "types", "", false, "print leaf type")
	cmd.Flags().BoolVarP(&gApp.Config.LocalFlags.PathSearch, "search", "", false, "search through path list")
	cmd.LocalFlags().VisitAll(func(flag *pflag.Flag) {
		gApp.Config.FileConfig.BindPFlag(fmt.Sprintf("%s-%s", cmd.Name(), flag.Name), flag)
	})
}

func walkDir(path, ext string) ([]string, error) {
	fs := make([]string, 0)
	err := filepath.Walk(path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			fi, err := os.Stat(path)
			if err != nil {
				return err
			}
			switch mode := fi.Mode(); {
			case mode.IsRegular():
				if filepath.Ext(path) == ext {
					fs = append(fs, path)
				}
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	return fs, nil
}

func findYangFiles(files []string) ([]string, error) {
	yfiles := make([]string, 0, len(files))
	for _, file := range files {
		fi, err := os.Stat(file)
		if err != nil {
			return nil, err
		}
		switch mode := fi.Mode(); {
		case mode.IsDir():
			fls, err := walkDir(file, ".yang")
			if err != nil {
				return nil, err
			}
			yfiles = append(yfiles, fls...)
		case mode.IsRegular():
			if filepath.Ext(file) == ".yang" {
				yfiles = append(yfiles, file)
			}
		}
	}
	return yfiles, nil
}
