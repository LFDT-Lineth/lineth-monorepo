package cmd

import (
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "state-manager-inspector",
	Short: "fetches and audit a sequence of merkle proofs for a range of blocks",
	RunE:  fetchAndInspect,
}

// global variables holding the programs arguments
var (
	startArg   int
	stopArg    int
	url        string
	maxRps     time.Duration
	numThreads int
	blockRange int
)

// initializes the programs flags
func init() {
	rootCmd.Flags().IntVar(&startArg, "start", 0, "starting block of the range (must be the beginning of a conflated batch)")
	rootCmd.Flags().IntVar(&stopArg, "stop", 0, "end of the range to fetch")
	rootCmd.Flags().StringVar(&url, "url", "https://127.0.0.1:443", "host:port of the shomei state-manager to query")
	rootCmd.Flags().DurationVar(&maxRps, "max-rps", 20*time.Millisecond, "minimal time to wait between each request")
	rootCmd.Flags().IntVar(&numThreads, "num-threads", runtime.NumCPU(), "number of threads to use for verification")
	rootCmd.Flags().IntVar(&blockRange, "block-range", 10, "size of the range to fetch from shomei for each request")
}

// Execute is the entry point of the current package and runs the state-manager
// inspector command.
func Execute() error {
	return rootCmd.Execute()
}
