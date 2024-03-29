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

// photosCmd represents the photos command
var photosCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate any missing photos",
	Long:  `This commands goes through all photos and generates new cropped versions of them`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("generate photos crops")
		config.InitConfig()
		db, err := dao.NewPGDB()
		if err != nil {
			fmt.Println(err)
			return
		}
		photos, err := db.Photo.List()
		if err != nil {
			fmt.Println(err)
			return
		}
		if err = dao.CreateImageDirs(); err != nil {
			fmt.Println(err)
			return
		}

		for _, photo := range photos {
			if err = dao.GenerateImages(photo.FileName); err != nil {
				fmt.Println(err)
				return
			}
		}
	},
}

func init() {
	photoCmd.AddCommand(photosCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// photosCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// photosCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
