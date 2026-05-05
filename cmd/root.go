package cmd

import (
	"fmt"
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
			fmt.Printf("\n***** Elapse: %v (sec) *****\n", (timeStop - timeStart))
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
}
