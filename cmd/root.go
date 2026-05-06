package cmd

import (
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	IsDebug             bool
	IsIgnoreDotFile     bool
	IsIgnoreEmptyFolder bool
	IsOverwrite         bool
	MaxSize             int64
	MinSize             int64
	MaxSizeMB           int64
	MinSizeMB           int64
	MinAge              string
	MaxAge              string
	SourceDir           string
	TargetDir           string
	LogDir              string
	FileExt             string
	//
)

var (
	minAge64 int64
	maxAge64 int64
)

var (
	timeStart int64
	timeStop  int64
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rpcopy",
	Short: "./rpcopy --source-dir=/path/to/folder-you-want-to-copy --target-dir=/path/to/target-folder ",
	Long:  ``,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		SourceDir = strings.TrimRight(ToUnixSlash(SourceDir), "/")
		TargetDir = strings.TrimRight(ToUnixSlash(TargetDir), "/")
		timeStart = GetNowUnix()
	},
	Run: func(cmd *cobra.Command, args []string) {

	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		timeStop = GetNowUnix()
		if timeStart > 0 && timeStop > 0 {
			timeDuration = timeStop - timeStart
			if timeDuration > 2 {
				// ClientSendFiles: clientStream.CloseSend()
				// Wait for 2 seconds before send
				timeDuration = timeDuration - 2
			}
			if timeDuration > 0 {
				totalSpeed = int64(math.Ceil(float64(totalWriteSize)/float64(timeDuration))) >> 20
				fmt.Println(SEP)
				fmt.Printf("::: Copied: %v, Size: %v MB, Speed: %v MB/s\n", totalNum, totalWriteSize>>20, totalSpeed)
			}

			fmt.Printf("\n***** Elapse: %v (sec) *****\n", timeDuration)
		}

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
	rootCmd.PersistentFlags().StringVar(&LogDir, "log-dir", ".", "log dir")
}
