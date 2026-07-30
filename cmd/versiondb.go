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
	"fmt"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/dao"
	"github.com/spf13/cobra"
)

// versionCmd reports the database schema version. It is read-only: it must never
// stamp the version, or it desyncs the version marker from the actual schema and
// makes a subsequent `db upgrade` a silent no-op. Use `db upgrade` to migrate.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the current database schema version",
	Long:  `Reports the database's current schema version and the version this binary targets. Read-only; use 'db upgrade' to migrate.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := config.InitConfig(); err != nil {
			fmt.Println(err)
			return
		}
		db, err := dao.NewPGDB()
		if err != nil {
			fmt.Println(err)
			return
		}
		v, err := db.Version.Get()
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Printf("Database schema version: %d (this binary targets %d)\n", v.VersionId, dao.DbVersion)
		if v.VersionId < dao.DbVersion {
			fmt.Println("Run 'db upgrade' to migrate to the latest version.")
		} else if v.VersionId > dao.DbVersion {
			fmt.Println("Warning: database is newer than this binary.")
		}
	},
}

func init() {
	dbCmd.AddCommand(versionCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// upgradedbCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// upgradedbCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
