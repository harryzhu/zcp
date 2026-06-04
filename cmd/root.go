/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var (
	IsDebug   bool
	Host      string
	Port      string
	IsWithTLS bool
	LogDir    string
	//
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "zcp",
	Short: "",
	Long:  ``,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		//DebugInfo("rootCmd", "PersistentPreRun")
		if LogDir != "" {
			LogDir = filepath.Join(LogDir, GetNowTimeStr("Ymd"))
		}
		SourceDir = strings.TrimRight(ToUnixSlash(SourceDir), "/")
		TargetDir = strings.TrimRight(ToUnixSlash(TargetDir), "/")
		LogDir = strings.TrimRight(ToUnixSlash(LogDir), "/")

		if LogDir != "" {
			MakeDirs(LogDir)
		}
		timeStart = GetNowUnix()
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		//DebugInfo("rootCmd", "PersistentPostRun")
		timeStop = GetNowUnix()
		timeDuration = timeStop - timeStart
		fmt.Println(sepLine)

		res := ""
		if timeDuration > 0 {
			speed := int64(float64(totalWriteSize) / float64(timeDuration))
			res = fmt.Sprintf("Total: %v, Size: %v MB, Speed: %v MB/s\n",
				totalNum, totalWriteSize>>20, speed>>20)
		} else {
			res = fmt.Sprintf("Total: %v, Size: %v MB\n",
				totalNum, totalWriteSize>>20)
		}

		fmt.Println(Cyan(res))
		PrintlnInfo("white", "zcp: Elapse(sec)", Int64Str(timeDuration))
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVar(&IsDebug, "debug", false, "if print debug info")
	//
	rootCmd.PersistentFlags().StringVar(&Host, "host", "0.0.0.0", "host ip")
	rootCmd.PersistentFlags().StringVar(&Port, "port", "9527", "port")
	rootCmd.PersistentFlags().StringVar(&LogDir, "log-dir", "./logs", "log dir")
	rootCmd.PersistentFlags().BoolVar(&IsWithTLS, "with-tls", false, "if use TLS")
}
