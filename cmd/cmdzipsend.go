package cmd

import (
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
)

// zipsendCmd represents the send command
var zipsendCmd = &cobra.Command{
	Use:   "zipsend",
	Short: "",
	Long:  ``,
	PreRun: func(cmd *cobra.Command, args []string) {
		//DebugInfo("sendCmd", "PreRun")
		MakeDirs(LogDir)
		if MinSizeMB != -1 {
			MinSize = MinSizeMB << 20
		}
		if MaxSizeMB != -1 {
			MaxSize = MaxSizeMB << 20
		}
		fextMatch = regexp.MustCompile("(?i)" + FileExt)
	},
	Run: func(cmd *cobra.Command, args []string) {
		//DebugInfo("sendCmd", "Run")
		var err error
		if IsWithTLS {
			err = SetTLSClientStreamConn()
		} else {
			err = SetClientStreamConn()
		}
		FatalError("send: cannot connect to server", err)

		serverHealthCheck()

		timeStart = GetNowUnix()
		pbHeadSourceFiles()
		wg := sync.WaitGroup{}
		wg.Add(3)

		go func() error {
			defer wg.Done()
			t1 := GetNowTime()
			PrintlnInfo("green", "SendSmallFileList", "Start ...")
			ClientSendSmallFileList()
			PrintlnInfo("green", "SendSmallFileList", " Done ... Elapse: ", time.Since(t1))
			atomic.AddInt32(&progressFlag, 1)
			return nil
		}()

		go func() error {
			defer wg.Done()
			t1 := GetNowTime()
			PrintlnInfo("blue", "SendLargeFileList", "Start ...")
			ClientSendLargeFileList()
			PrintlnInfo("blue", "SendLargeFileList", " Done ... Elapse: ", time.Since(t1))
			atomic.AddInt32(&progressFlag, 1)
			return nil
		}()

		go func() error {
			defer wg.Done()
			PrintProgress()
			PrintlnInfo("purple", "100%", "bye ...")
			return nil
		}()

		wg.Wait()

		ClientSendDirSymlink()
		ClientGetReport()

		err = gClientStream.CloseSend()
		PrintError("ClientSendFiles: CloseSend", err)
		// close connection
		gClientConn.Close()

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
	rootCmd.AddCommand(zipsendCmd)
	//
	zipsendCmd.Flags().StringVar(&SourceDir, "source-dir", "", "source folder")
	zipsendCmd.Flags().BoolVar(&IsFollowSymlink, "follow-symlink", false, "if copy the linked file rather than the symlink ...")
	//
	zipsendCmd.Flags().BoolVar(&IsIgnoreDotFile, "ignore-dot-file", false, "ignore the file if its file name starts with dot(.), i.e.: .DS_Store")
	//
	zipsendCmd.Flags().StringVar(&FileExt, "ext", "", "file type filter, i.e.: .mp4 or .png or .(mp4|txt|png) ")
	//
	zipsendCmd.Flags().Int64Var(&MinSize, "min-size", -1, "from the minimum file size")
	zipsendCmd.Flags().Int64Var(&MaxSize, "max-size", -1, "to the maximum file size")
	zipsendCmd.Flags().Int64Var(&MinSizeMB, "min-size-mb", -1, "i.e.: 16 means 16MB, will replace --min-size=16*1024*1024 automatically")
	zipsendCmd.Flags().Int64Var(&MaxSizeMB, "max-size-mb", -1, "i.e.: 32 means 32MB, will replace --max-size=32*1024*1024 automatically")
	//
	zipsendCmd.Flags().StringVar(&MinAge, "min-age", "", "format: 2023-12-03,15:09:08, means 2023-12-03 15:09:08")
	zipsendCmd.Flags().StringVar(&MaxAge, "max-age", "", "format: 2023-12-25,23:59:59, means 2023-12-25 23:59:59")

	rootCmd.MarkFlagRequired("host")
	rootCmd.MarkFlagRequired("port")
	rootCmd.MarkFlagRequired("log-dir")
	zipsendCmd.MarkFlagRequired("source-dir")
}
