/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "",
	Long:  ``,
	PreRun: func(cmd *cobra.Command, args []string) {
		StartLogging("server.log")

		if TargetDir == "" || !Exists(TargetDir) {
			FatalError("arg error", NewError("--target-dir=  does not exist."))
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		if IsWithTLS {
			PrintlnInfo("green", "SERVER", "in TLS mode")
			StartTLSFileTransferServer()
		} else {
			StartFileTransferServer()
		}

	},
	PostRun: func(cmd *cobra.Command, args []string) {

	},
}

func init() {
	rootCmd.AddCommand(serverCmd)

	rootCmd.MarkFlagRequired("host")
	rootCmd.MarkFlagRequired("port")
	rootCmd.MarkFlagRequired("log-dir")

	serverCmd.PersistentFlags().StringVar(&TargetDir, "target-dir", "", "target dir for file saving")

}
