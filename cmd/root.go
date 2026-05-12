package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	IsDebug             bool
	IsIgnoreDotFile     bool
	IsIgnoreEmptyFolder bool
	IsOverwrite         bool
	IsZstdSend          bool
	IsFollowSymlink     bool
	MaxSize             int64
	MinSize             int64
	MaxSizeMB           int64
	MinSizeMB           int64
	MinAge              string
	MaxAge              string
	FileExt             string
	//
	SourceDir string
	TargetDir string
	LogDir    string

	//
	Host      string
	Port      string
	IsWithTLS bool
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
	Short: "",
	Long:  ``,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		//DebugInfo("rootCmd", "PersistentPreRun")
		SourceDir = strings.TrimRight(ToUnixSlash(SourceDir), "/")
		TargetDir = strings.TrimRight(ToUnixSlash(TargetDir), "/")
		timeStart = GetNowUnix()
	},
	Run: func(cmd *cobra.Command, args []string) {
		//DebugInfo("rootCmd", "Run")
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		//DebugInfo("rootCmd", "PersistentPostRun")
		timeStop = GetNowUnix()
		timeDuration = timeStop - timeStart
		PrintlnInfo("Cyan", "rpcopy: elapse(sec)", Int64Str(timeDuration))
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
	rootCmd.PersistentFlags().StringVar(&LogDir, "log-dir", "./logs", "log dir")
	rootCmd.PersistentFlags().BoolVar(&IsWithTLS, "with-tls", false, "if use TLS")
}
