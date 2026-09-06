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
		StartLogging(LogDir, "server.log")
		if runPlatform == "windows" {
			tips := `[zcp.exe server] is running on Windows. If you want to KEEP symbol link from Linux/MacOS, running [zcp.exe server] with Administrator Privileges is recommended, or symbol links will be IGNORED with permission error`
			PrintlnInfo("cyan", tips)
		}

		if TargetDir == "" || !Exists(TargetDir) {
			FatalError("--target-dir= error", NewError(TargetDir+" does not exist."))
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
