/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

// serverCmd represents the server command
var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "--target-dir=  --host=your-server-ip  --port=your-server-port",
	Long:  ``,
	PreRun: func(cmd *cobra.Command, args []string) {
		if TargetDir == "" {
			FatalError("send", NewError("--target-dir= cannot be empty"))
		}
		bootstrap()
		MakeDirs(TargetDir)

		finfo, err := os.Stat(TargetDir)
		if err != nil {
			FatalError("send", NewError("--target-dir= does not exist"))
		}

		if !finfo.IsDir() {
			FatalError("send", NewError("--target-dir=", TargetDir, " should be a directory"))
		}

		tStart = GetNowTime()
	},
	Run: func(cmd *cobra.Command, args []string) {
		wg := sync.WaitGroup{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			StartFileTransferServer()
		}()

		wg.Wait()
	},
	PostRun: func(cmd *cobra.Command, args []string) {
		fmt.Printf("\n***** Elapse: %v *****\n", time.Since(tStart))

	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	rootCmd.MarkFlagRequired("host")
	rootCmd.MarkFlagRequired("port")

	serverCmd.Flags().StringVar(&TargetDir, "target-dir", "", "root dir for saving")
	serverCmd.Flags().BoolVar(&IsOverwrite, "overwrite", false, "allow to overwrite the existing files")

	serverCmd.MarkFlagRequired("target-dir")
}
