package cmd

import (
	"sync"

	"github.com/spf13/cobra"
)

// sendCmd represents the send command
var sendCmd = &cobra.Command{
	Use:   "send",
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

		pbHeadSourceFiles()
		wg := sync.WaitGroup{}
		wg.Add(3)

		go func() {
			defer wg.Done()
			ClientSendSmallFileList()
			DebugInfo("ClientSendSmallFileList", "ALL_DONE")
		}()

		go func() {
			defer wg.Done()
			ClientSendMediumFileList()
			DebugInfo("largeFileList", largeFileList)
			DebugInfo("ClientSendMediumFileList", "ALL_DONE")
		}()

		go func() {
			defer wg.Done()
			ClientSendLargeFileList()
			DebugInfo("ClientSendLargeFileList", "ALL_DONE")
		}()
		wg.Wait()

		ClientSendDirSymlink()
		ClientGetReport()

		//time.Sleep(1 * time.Second)
		DebugInfo("ClientSendFiles: CloseSend", "...")
		err = gClientStream.CloseSend()
		PrintError("ClientSendFiles: CloseSend", err)
		// close connection
		//gClientConn.Close()
	},
}

func init() {
	rootCmd.AddCommand(sendCmd)
	//
	sendCmd.Flags().StringVar(&SourceDir, "source-dir", "", "source folder")
	sendCmd.Flags().BoolVar(&IsZstdSend, "zstd", false, "if enable zstd compression, better for txt/pdf ...")
	sendCmd.Flags().BoolVar(&IsFollowSymlink, "follow-symlink", false, "if copy the linked file rather than the symlink ...")
	//
	sendCmd.Flags().BoolVar(&IsIgnoreDotFile, "ignore-dot-file", false, "ignore the file if its file name starts with dot(.), i.e.: .DS_Store")
	sendCmd.Flags().BoolVar(&IsIgnoreEmptyFolder, "ignore-empty-dir", true, "ignore the folder if it contains nothing")
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

	rootCmd.MarkFlagRequired("host")
	rootCmd.MarkFlagRequired("port")
	sendCmd.MarkFlagRequired("source-dir")
}
