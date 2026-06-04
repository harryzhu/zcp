/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/spf13/cobra"
)

// diffCmd represents the diff command
var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "",
	Long:  ``,
	PreRun: func(cmd *cobra.Command, args []string) {
		PrintlnInfo("white", "source-dir", SourceDir)
		PrintlnInfo("white", "log-dir", LogDir)
		fmt.Println(sepLine)
		if !Exists(SourceDir) {
			FatalError("diff", NewError("dir does not exist: ", SourceDir))
		}

		serverHealthCheck()
	},
	Run: func(cmd *cobra.Command, args []string) {
		wg := sync.WaitGroup{}
		wg.Add(3)
		go func() {
			defer wg.Done()
			ClientWalkSourceDir()
		}()

		go func() {
			defer wg.Done()
			taskDiffFiles()
			atomic.StoreInt32(&progressFlag, 2)
		}()

		go func() {
			defer wg.Done()
			ShowProgress()
		}()

		wg.Wait()
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
	//
	rootCmd.MarkFlagRequired("host")
	rootCmd.MarkFlagRequired("port")
	rootCmd.MarkFlagRequired("log-dir")

	diffCmd.Flags().StringVar(&SourceDir, "source-dir", "", "source folder")

	diffCmd.MarkFlagRequired("source-dir")
}
