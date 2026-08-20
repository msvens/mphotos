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

// checkCmd verifies the database schema matches this binary's DbVersion. It is the
// machine-checkable counterpart to `db version`: it exits non-zero (with a
// message) when the DB is behind or ahead, so a deploy script can gate on it
// before starting the server — which would otherwise hard-stop and crash-loop
// while systemctl still reported success. Read-only.
var checkCmd = &cobra.Command{
	Use:           "check",
	Short:         "Verify the database schema matches this binary (exit non-zero on mismatch)",
	Long:          `Checks that the database schema version equals the version this binary targets. Read-only; exits non-zero with a message if a migration is needed or the database is newer than the binary. Intended as a pre-start gate in deploy scripts.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := config.InitConfig(); err != nil {
			return err
		}
		db, err := dao.NewPGDB()
		if err != nil {
			return err
		}
		if err := db.Version.Check(); err != nil {
			return err
		}
		fmt.Printf("Database schema is up to date (version %d).\n", dao.DbVersion)
		return nil
	},
}

func init() {
	dbCmd.AddCommand(checkCmd)
}
