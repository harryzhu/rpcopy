package cmd

import (
	"sync"

	"github.com/spf13/cobra"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "",
	Long:  ``,
	PreRun: func(cmd *cobra.Command, args []string) {
		//DebugInfo("serverCmd", "PreRun")
		MakeDirs(TargetDir)
		MakeDirs(LogDir)
	},
	Run: func(cmd *cobra.Command, args []string) {
		//DebugInfo("serverCmd", "Run")
		wg := sync.WaitGroup{}
		wg.Add(5)
		go func() {
			defer wg.Done()
			taskChanFile()
		}()
		go func() {
			defer wg.Done()
			taskChanFile1()
		}()
		go func() {
			defer wg.Done()
			taskChanFile2()
		}()
		go func() {
			defer wg.Done()
			taskChanFile3()
		}()

		go func() {
			defer wg.Done()
			if IsWithTLS {
				PrintlnInfo("blue", "try to enable TLS mode")
				StartTLSFileTransferServer()
			} else {
				StartFileTransferServer()
			}
		}()
		wg.Wait()

	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	rootCmd.MarkFlagRequired("host")
	rootCmd.MarkFlagRequired("port")
	rootCmd.MarkFlagRequired("log-dir")

	serverCmd.Flags().StringVar(&TargetDir, "target-dir", "", "root dir for saving")
	serverCmd.Flags().BoolVar(&IsOverwrite, "overwrite", true, "allow to overwrite the existing files")

	serverCmd.MarkFlagRequired("target-dir")
}
