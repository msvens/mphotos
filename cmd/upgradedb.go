/*
Copyright © 2020 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/dao"
	"github.com/spf13/cobra"
)

// upgradedbCmd represents the upgradedb command
var upgradedbCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade mphotos database",
	Long:  `Upgrades the mphotos database to the latest version if possible`,
	Run: func(cmd *cobra.Command, args []string) {
		config.InitConfig()
		if err := dao.UpgradeDb(); err != nil {
			println(err.Error())
		}
	},
}

func init() {
	dbCmd.AddCommand(upgradedbCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// upgradedbCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// upgradedbCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
