/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"sync/atomic"

	"github.com/spf13/cobra"
)

// diffCmd represents the diff command
var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "--source-dir=  --target-dir=",
	Long:  ``,
	PreRun: func(cmd *cobra.Command, args []string) {
		//DebugInfo("sendCmd", "PreRun")
		MakeDirs(LogDir)
	},
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		if IsWithTLS {
			err = SetTLSClientStreamConn()
		} else {
			err = SetClientStreamConn()
		}
		FatalError("send: cannot connect to server", err)

		serverHealthCheck()

		timeStart = GetNowUnix()

		ClientDiffAllFiles()
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		timeStop = GetNowUnix()
		tnum := atomic.LoadInt32(&totalNum)
		tduration := timeStop - timeStart
		if tduration > 0 {
			tws := atomic.LoadInt64(&totalWriteSize)
			tspeed := int64((float64(tws) / float64(tduration)))
			fmt.Println(sepLine)
			fmt.Printf("\nCount: %d, Size: %d MB, Speed: %d MB/s\n", tnum, tws>>20, tspeed>>20)
			fmt.Println(sepLine)
		}
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
	rootCmd.MarkFlagRequired("host")
	rootCmd.MarkFlagRequired("port")
	rootCmd.MarkFlagRequired("log-dir")

	diffCmd.Flags().StringVar(&SourceDir, "source-dir", "", "source folder")

	diffCmd.MarkFlagRequired("source-dir")
}
