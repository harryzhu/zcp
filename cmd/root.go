/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	Host       string
	Port       string
	IsSerial   bool = false
	IsDebug    bool = false
	IsWithTLS  bool = false
	IsWithDiff bool = true
	LogDir     string
	//
	SourceDir string
	TargetDir string
	//
	FileExt    string
	MinSize    int64
	MaxSize    int64
	MinSizeMB  int64
	MaxSizeMB  int64
	MinAge     string
	MaxAge     string
	MinAgeUnix int64
	MaxAgeUnix int64
	//
	IsFollowSymlink bool = false
	IsIgnoreDotFile bool = false
	//
	ErrorLogFile string = "logs/errors.log"
	//
	tStart time.Time
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "zcp",
	Short: "",
	Long:  ``,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		SourceDir = strings.TrimSuffix(ToUnixSlash(SourceDir), "/")
		TargetDir = strings.TrimSuffix(ToUnixSlash(TargetDir), "/")
		LogDir = strings.TrimSuffix(ToUnixSlash(LogDir), "/")

		tStart = GetNowTime()
	},

	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		PrintlnInfo("green", "Total Time", time.Since(tStart))
		StopLogging()
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
	rootCmd.PersistentFlags().BoolVar(&IsSerial, "serial", false, "copy files one-by-one")
	//
	rootCmd.PersistentFlags().StringVar(&Host, "host", "0.0.0.0", "host ip")
	rootCmd.PersistentFlags().StringVar(&Port, "port", "9527", "port")
	rootCmd.PersistentFlags().StringVar(&LogDir, "log-dir", "./logs", "log dir")
	rootCmd.PersistentFlags().BoolVar(&IsWithTLS, "with-tls", false, "if use TLS")
}
