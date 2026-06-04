/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"

	"github.com/spf13/cobra"
)

var (
	SourceDir string
	//
	IsIgnoreDotFile bool
	IsFollowSymlink bool
	MaxSize         int64
	MinSize         int64
	MaxSizeMB       int64
	MinSizeMB       int64
	MinAge          string
	MaxAge          string
	FileExt         string
	//
	fextMatch *regexp.Regexp
)

// pushCmd represents the push command
var pushCmd = &cobra.Command{
	Use:   "push",
	Short: "",
	Long:  ``,
	PreRun: func(cmd *cobra.Command, args []string) {
		PrintlnInfo("white", "source-dir", SourceDir)
		PrintlnInfo("white", "log-dir", LogDir)
		if !Exists(SourceDir) {
			FatalError("push", NewError("dir does not exist: ", SourceDir))
		}
		fextMatch = regexp.MustCompile("(?i)" + FileExt)
		if MinSizeMB > -1 {
			MinSize = MinSizeMB << 20
		}
		if MaxSizeMB > -1 {
			MaxSize = MaxSizeMB << 20

		}
		serverHealthCheck()
		fmt.Println(sepLine)
	},
	Run: func(cmd *cobra.Command, args []string) {
		wg := sync.WaitGroup{}
		wg.Add(4)
		go func() {
			defer wg.Done()
			ClientWalkSourceDir()
		}()

		go func() {
			defer wg.Done()
			ClientSendLargeFiles()
			atomic.AddInt32(&progressFlag, 1)
		}()

		go func() {
			defer wg.Done()
			ClientSendSmallFiles()
			atomic.AddInt32(&progressFlag, 1)
		}()

		go func() {
			defer wg.Done()
			ShowProgress()
		}()

		wg.Wait()

		ClientSendDirSymlink()
	},
}

func init() {
	rootCmd.AddCommand(pushCmd)
	//
	rootCmd.MarkFlagRequired("host")
	rootCmd.MarkFlagRequired("port")
	rootCmd.MarkFlagRequired("log-dir")

	pushCmd.Flags().StringVar(&SourceDir, "source-dir", "", "source folder")
	//
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
	pushCmd.Flags().StringVar(&MinAge, "min-age", "", "format: 2023-12-03,15:09:08, means 2023-12-03 15:09:08")
	pushCmd.Flags().StringVar(&MaxAge, "max-age", "", "format: 2023-12-25,23:59:59, means 2023-12-25 23:59:59")

	pushCmd.MarkFlagRequired("source-dir")
}
