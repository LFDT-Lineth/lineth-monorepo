package main

import (
	"os"
	"strings"

	"github.com/consensys/gnark/logger"
	"github.com/consensys/linea-monorepo/prover/cmd/prover/cmd"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// rootCmd represents the base command when called without any subcommands
	rootCmd = &cobra.Command{
		Use:   "prover",
		Short: "run pre-compute or compute proofs for Linea circuits",
	}
	fConfigFile string

	// setupCmd represents the setup command
	setupCmd = &cobra.Command{
		Use:   "setup",
		Short: "pre compute assets for Linea circuits",
		RunE:  cmdSetup,
	}
	setupArgs cmd.SetupArgs

	// proveCmd represents the prove command
	proveCmd = &cobra.Command{
		Use:   "prove",
		Short: "prove process a request, creates a proof with the adequate circuit and writes the proof to a file",
		RunE:  cmdProve,
	}
	proverArgs cmd.ProverArgs
)

func main() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&fConfigFile, "config", "", "config file")

	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: "15:04:05", NoColor: true}
	l := zerolog.New(output).With().Timestamp().Logger()

	// Set global log level for gnark
	logger.Set(l)

	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().BoolVar(&setupArgs.Force, "force", false, "overwrites existing files")
	setupCmd.Flags().StringVar(&setupArgs.Circuits, "circuits", strings.Join(allCircuitList(), ","), "comma separated list of circuits to setup")
	setupCmd.Flags().StringVar(&setupArgs.DictPath, "dict", "", "path to the dictionary file used in blob (de)compression (for v0 only)")
	setupCmd.Flags().IntVar(&setupArgs.DictSize, "dict-size", 65536, "size in bytes of the dictionary used in blob (de)compression")
	setupCmd.Flags().StringVar(&setupArgs.AssetsDir, "assets-dir", "", "path to the directory where the assets are stored (override conf)")

	viper.BindPFlag("assets_dir", setupCmd.Flags().Lookup("assets-dir"))

	rootCmd.AddCommand(proveCmd)

	proveCmd.Flags().StringVar(&proverArgs.Input, "in", "", "input file")
	proveCmd.Flags().StringVar(&proverArgs.Output, "out", "", "output file")
	proveCmd.Flags().BoolVar(&proverArgs.Large, "large", false, "run the large execution circuit")
}

func cmdSetup(_cmd *cobra.Command, _ []string) error {
	setupArgs.ConfigFile = fConfigFile
	return cmd.Setup(_cmd.Context(), setupArgs)
}

func cmdProve(*cobra.Command, []string) error {
	proverArgs.ConfigFile = fConfigFile
	return cmd.Prove(proverArgs)
}

// allCircuitList returns the list [cmd.AllCircuits] where the circuit id
// are converted into strings.
func allCircuitList() []string {
	res := make([]string, len(cmd.AllCircuits))
	for i := range res {
		res[i] = string(cmd.AllCircuits[i])
	}
	return res
}
