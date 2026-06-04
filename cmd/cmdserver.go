/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	TargetDir   string
	IsOverwrite bool
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "",
	Long:  ``,
	PreRun: func(cmd *cobra.Command, args []string) {
		if !Exists(TargetDir) {
			MakeDirs(TargetDir)
		}
		PrintlnInfo("white", "target-dir", TargetDir)
		PrintlnInfo("white", "log-dir", LogDir)
		PrintlnInfo("white", "allow-overwrite", IsOverwrite)
		fmt.Println(sepLine)
	},
	Run: func(cmd *cobra.Command, args []string) {
		if IsWithTLS {
			PrintlnInfo("white", "try to enable TLS mode")
			StartTLSFileTransferServer()
		} else {
			StartFileTransferServer()
		}
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	//
	rootCmd.MarkFlagRequired("host")
	rootCmd.MarkFlagRequired("port")
	rootCmd.MarkFlagRequired("log-dir")

	serverCmd.Flags().StringVar(&TargetDir, "target-dir", "", "root dir for saving")
	serverCmd.Flags().BoolVar(&IsOverwrite, "overwrite", true, "allow to overwrite the existing files")

	serverCmd.MarkFlagRequired("target-dir")
}
