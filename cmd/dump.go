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
	"encoding/json"
	"fmt"
	"github.com/msvens/mphotos/internal/config"
	"github.com/msvens/mphotos/internal/dao"
	"github.com/spf13/cobra"
)

var dumpJson bool
var table uint

var dumpPhotosCmd = &cobra.Command{
	Use:   "dump",
	Short: "Dump database table",
	Long:  `Dumps a table in json format`,
	Run: func(cmd *cobra.Command, args []string) {
		config.InitConfig()
		db, err := dao.NewPGDB()
		if err != nil {
			fmt.Println(err)
			return
		}
		var out interface{}
		switch table {
		case 0:
			out, err = db.Photo.List()
		case 1:
			exifs := []*dao.Exif{}
			var pl []*dao.Photo
			pl, err = db.Photo.List()
			if err != nil {
				break
			}
			for _, p := range pl {
				if exif, e1 := db.Photo.Exif(p.Id); e1 != nil {
					err = e1
					break
				} else {
					exifs = append(exifs, exif)
				}
			}
			out = exifs
		default:
			err = fmt.Errorf("Unregcongised table: %v", table)
		}
		if err != nil {
			fmt.Println(err)
			return
		}
		if out, err := json.MarshalIndent(out, "", "  "); err != nil {
			fmt.Println(err)
			return
		} else {
			fmt.Println(string(out))
		}
	},
}

func init() {
	dbCmd.AddCommand(dumpPhotosCmd)

	dumpPhotosCmd.Flags().BoolVarP(&dumpJson, "json", "j", true, "Dump as Json")
	dumpPhotosCmd.Flags().UintVarP(&table, "table", "t", 0, "0 - photos, 1 - exif")

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// photosCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// photosCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
