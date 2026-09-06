/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/spf13/cobra"
)

var (
	fextMatch *regexp.Regexp
)

// pushCmd represents the push command
var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "",
	Long:  ``,
	PreRun: func(cmd *cobra.Command, args []string) {
		StartLogging(LogDir, "client.log")

		if SourceDir == "" || !Exists(SourceDir) {
			FatalError("--source-dir= error", NewError(SourceDir+" does not exist."))
		}

		if MinSizeMB != -1 {
			MinSize = MinSizeMB << 20
		}
		if MaxSizeMB != -1 {
			MaxSize = MinSizeMB << 20
		}
		if MinAge != "" {
			MinAgeUnix = TimeStr2Unix(MinAge)
		}
		if MaxAge != "" {
			MaxAgeUnix = TimeStr2Unix(MaxAge)
		}
		if FileExt != "" {
			fextMatch = regexp.MustCompile("(?i)" + FileExt)
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		if IsWithTLS {
			PrintlnInfo("green", "PUSH", "in TLS mode")
		}
		serverInfo := gClientHandshake()
		DebugInfo("Server Info", serverInfo)
		if strings.Contains(serverInfo, "windows") {
			if runPlatform == "windows" {
				IsFollowSymlink = true
			}
			if runPlatform != "windows" {
				PrintlnInfo("cyan", "remote server is running on Windows, [./zcp push] with --follow-symlink=true is recommended. Currently", IsFollowSymlink)
			}
		}

		wg := sync.WaitGroup{}
		wg.Add(3)

		go func() {
			taskSendLargeFiles()
			wg.Done()
		}()

		go func() {
			taskSendSmallFiles()
			wg.Done()
		}()

		go func() {
			selectFiles()
			wg.Done()
		}()

		wg.Wait()
		gClientSyncFolderSymlink()
		logSendFailure()
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		if IsDebug {
			tSizeMB := atomic.LoadInt64(&totalSize) >> 20
			speed := gClientGetSpeed() >> 20
			tNum := atomic.LoadInt32(&totalNum)
			PrintlnInfo("purple", "Stats", "Speed: ", speed, " MB/s, Files: ",
				tNum, ", Size: ", tSizeMB, " MB")
		}
	},
}

func init() {
	rootCmd.AddCommand(pushCmd)

	rootCmd.MarkFlagRequired("host")
	rootCmd.MarkFlagRequired("port")
	rootCmd.MarkFlagRequired("log-dir")

	pushCmd.PersistentFlags().StringVar(&SourceDir, "source-dir", "", "source dir for file push")
	//
	pushCmd.Flags().BoolVar(&IsWithDiff, "with-diff", true, "if diff local files from remote before push")
	pushCmd.Flags().BoolVar(&IsFollowSymlink, "follow-symlink", false, "if copy the linked file rather than the symlink ...")
	pushCmd.Flags().BoolVar(&IsIgnoreDotFile, "ignore-dot-file", false, "ignore the file if its file name starts with dot(.), i.e.: .DS_Store")
	//
	pushCmd.Flags().StringVar(&FileExt, "ext", "", "file type filter, i.e.: .mp4 or .png or .(mp4|txt|png) ")
	//
	pushCmd.Flags().Int64Var(&MinSize, "min-size", -1, "from the minimum file size")
	pushCmd.Flags().Int64Var(&MaxSize, "max-size", -1, "to the maximum file size")
	pushCmd.Flags().Int64Var(&MinSizeMB, "min-size-mb", -1, "i.e.: 16 means 16MB, will replace --min-size=16*1024*1024 automatically")
	pushCmd.Flags().Int64Var(&MaxSizeMB, "max-size-mb", -1, "i.e.: 32 means 32MB, will replace --max-size=32*1024*1024 automatically")
	//
	pushCmd.Flags().StringVar(&MinAge, "min-age", "", "format: \"2023-12-03 15:09:08\", must be with quotation mark")
	pushCmd.Flags().StringVar(&MaxAge, "max-age", "", "format: \"2023-12-25 23:59:59\", must be with quotation mark")
	//
	pushCmd.MarkFlagRequired("source-dir")
}
