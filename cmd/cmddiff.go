/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// diffCmd represents the diff command
var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "",
	Long:  ``,
	PreRun: func(cmd *cobra.Command, args []string) {
		StartLogging(LogDir, "client.log")

		if SourceDir == "" || !Exists(SourceDir) {
			FatalError("--source-dir= error", NewError(SourceDir+" does not exist."))
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		if IsWithTLS {
			PrintlnInfo("green", "DIFF", "in TLS mode")
		}
		gClientHandshake()
		fmt.Println(Cyan("Different Files:"))
		fmt.Println(Cyan("-----------------------------------------"))
		diffFiles()
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
	rootCmd.MarkFlagRequired("host")
	rootCmd.MarkFlagRequired("port")
	rootCmd.MarkFlagRequired("log-dir")

	diffCmd.PersistentFlags().StringVar(&SourceDir, "source-dir", "", "source dir for file diff")

	diffCmd.MarkFlagRequired("source-dir")
}
