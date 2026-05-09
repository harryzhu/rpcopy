/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/spf13/cobra"
)

var (
	tStart          time.Time
	IsZstdSend      bool
	IsFollowSymlink bool
)

// sendCmd represents the send command
var sendCmd = &cobra.Command{
	Use:   "send",
	Short: "--source-dir=  --host=your-server-ip  --port=your-server-port",
	Long:  ``,
	PreRun: func(cmd *cobra.Command, args []string) {
		if SourceDir == "" {
			FatalError("send", NewError("--source-dir= cannot be empty"))
		}
		if FileExists(SourceDir) == false {
			FatalError("send", NewError("folder does not exist: --source-dir=", SourceDir))
		}
		argsFinfoValidate()
		argsValidate()
		bootstrap()

		MakeDirs(LogDir)
		timeStart = GetNowUnix()
	},
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		if WithTLS {
			PrintlnInfo("try to enable TLS mode")
			err = SetTLSClientStreamConn()
		} else {
			err = SetClientStreamConn()
		}

		if err != nil {
			FatalError("send: client", err)
		}
		// server ping
		sp1 := GetNowTime()
		ping, err := serverPing()
		spDuration := time.Since(sp1)
		if err != nil {
			PrintError("cannot connect to server", err)
		}
		PrintlnInfo("HealthCheck response from Server", ping, ". Delay: ", spDuration)

		SetFileList()

		atomic.StoreInt32(&progressFlag, 0)

		wg := sync.WaitGroup{}
		wg.Add(4)
		go func() {
			defer wg.Done()
			ClientSendSmallFileList()
			atomic.AddInt32(&progressFlag, 1)
		}()

		go func() {
			defer wg.Done()
			ClientSendLargeFileList()
			atomic.AddInt32(&progressFlag, 1)
		}()

		go func() {
			defer wg.Done()
			ClientSetDirSymList()
			atomic.AddInt32(&progressFlag, 1)
		}()

		go func() {
			defer wg.Done()
			PrintProgress()
		}()

		wg.Wait()

		tsDir := GetNowUnix()
		DebugInfo("ClientSendDirSymlink: syncing", "dir & symlink")
		ClientSendDirSymlink()
		DebugInfo("ClientSendDirSymlink: elapse", GetNowUnix()-tsDir)

		DebugInfo("sendReportSignal: Grabbing", "report")
		sendReportSignal(gClient)
		//
		time.Sleep(2 * time.Second)
		DebugInfo("ClientSendFiles: CloseSend", "...")
		err = gClientStream.CloseSend()
		PrintError("ClientSendFiles: CloseSend", err)

		gClientConn.Close()
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		timeStop = GetNowUnix()
	},
}

func init() {
	rootCmd.AddCommand(sendCmd)
	//
	sendCmd.Flags().StringVar(&SourceDir, "source-dir", "", "source folder")
	//
	sendCmd.Flags().BoolVar(&IsZstdSend, "zstd", false, "if enable zstd compression, better for txt/pdf ...")
	//
	sendCmd.Flags().BoolVar(&IsIgnoreDotFile, "ignore-dot-file", false, "ignore the file if its file name starts with dot(.), i.e.: .DS_Store")
	sendCmd.Flags().BoolVar(&IsIgnoreEmptyFolder, "ignore-empty-dir", false, "ignore the folder if it contains nothing")

	sendCmd.Flags().BoolVar(&IsFollowSymlink, "follow-symlink", false, "if true: copy linked file), if false: copy the symlink rather than the linked file")
	//
	sendCmd.Flags().StringVar(&FileExt, "ext", "", "file type filter, i.e.: .mp4 or .png or .(mp4|txt|png) ")
	//
	sendCmd.Flags().Int64Var(&MinSize, "min-size", -1, "from the minimum file size")
	sendCmd.Flags().Int64Var(&MaxSize, "max-size", -1, "to the maximum file size")
	sendCmd.Flags().Int64Var(&MinSizeMB, "min-size-mb", -1, "i.e.: 16 means 16MB, will replace --min-size=16*1024*1024 automatically")
	sendCmd.Flags().Int64Var(&MaxSizeMB, "max-size-mb", -1, "i.e.: 32 means 32MB, will replace --max-size=32*1024*1024 automatically")
	//
	sendCmd.Flags().StringVar(&MinAge, "min-age", "", "format: 2023-12-03,15:09:08, means 2023-12-03 15:09:08")
	sendCmd.Flags().StringVar(&MaxAge, "max-age", "", "format: 2023-12-25,23:59:59, means 2023-12-25 23:59:59")
	//
	rootCmd.MarkFlagRequired("host")
	rootCmd.MarkFlagRequired("port")
	sendCmd.MarkFlagRequired("source-dir")
}
