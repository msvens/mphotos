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

// cleanPhotosCmd represents the photos command
var cleanPhotosCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean photos from server",
	Long:  `This command goes through all image files and removes those that are no longer in the photo database`,
	Run: func(cmd *cobra.Command, args []string) {
		config.InitConfig()
		//imgDir := config.ServicePath("img")
		//baseDir := config.ServiceRoot()
		db, err := dao.NewPGDB()
		if err != nil {
			fmt.Println(err)
			return
		}
		if err = dao.CleanImageDirs(db); err != nil {
			fmt.Println(err)
		}
		/*
			photos, err := db.Photo.List()
			if err != nil {
				fmt.Println(err)
				return
			}
			if err = server.CreateImageDirs(); err != nil {
				fmt.Println(err)
				return
			}

			for _, photo := range photos {
				if err = server.GenerateImages(filepath.Join(imgDir, photo.FileName), baseDir); err != nil {
					fmt.Println(err)
					return
				}
			}
		*/
	},
}

func init() {
	photoCmd.AddCommand(cleanPhotosCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// photosCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// photosCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
