package cmd

import (
	"context"
	"strings"
)

type LogStatsArgs struct {
	Input      string
	StatsFile  string
	ConfigFile string
}

func LogStats(_ context.Context, args LogStatsArgs) error {

	panic("uncomment when the limitless prover has been integrated")

	// const cmdName = "log-stats"

	// // Read config
	// cfg, err := config.NewConfigFromFileUnchecked(args.ConfigFile)
	// if err != nil {
	// 	return fmt.Errorf("%s failed to read config file: %w", cmdName, err)
	// }

	// // Read the input file
	// req := &execution.Request{}
	// if err := readRequest(args.Input, req); err != nil {
	// 	return fmt.Errorf("could not read the input file (%v): %w", args.Input, err)
	// }

	// // Setup execution witness and output response
	// var (
	// 	out          = execution.CraftProverOutput(cfg, req)
	// 	witness      = execution.NewWitness(cfg, req, &out)
	// 	lz           = zkevm.NewLimitlessRawZkEVM(cfg)
	// 	stats        = lz.RunStatRecords(witness.ZkEVM)
	// 	shortReqName = path.Base(args.Input)
	// )

	// // Open the file in append mode, create it if it doesn't exist
	// file, err := os.OpenFile(args.StatsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	// if err != nil {
	// 	return fmt.Errorf("an error occurred while opening the stats file: %w", err)
	// }
	// defer file.Close() // Ensure the file is closed when the function exits

	// fileLock := flock.New(args.StatsFile)
	// fileLock.Lock()
	// defer fileLock.Unlock()

	// // Write the stats to the document. We always append the header to the CSV
	// // because it's easier to remove it from the stats-files if needed than the
	// // doing the contrary.
	// w := struct2csv.NewWriter(file)
	// w.SetComma('|')

	// if err := w.WriteColNames(stats[0]); err != nil {
	// 	return fmt.Errorf("an error occurred while writing the stats file header: %w", err)
	// }

	// for _, stat := range stats {

	// 	stat.Request = shortReqName

	// 	// Remove the commas is not fundamentally important (as we use | as a
	// 	// separator). But this helps avoiding issues with CSV analysis.
	// 	removeCommas(&stat.ModuleName)
	// 	removeCommas(&stat.FirstColumnAlphabetical)
	// 	removeCommas(&stat.LastColumnAlphabetical)
	// 	removeCommas(&stat.LastLeftPadded)
	// 	removeCommas(&stat.LastRightPadded)

	// 	if err := w.WriteStruct(stat); err != nil {
	// 		return fmt.Errorf("an error occurred while writing the stats file row: %w", err)
	// 	}
	// }

	// w.Flush()
	// return nil
}

func removeCommas[T ~string](s *T) {
	*s = T(strings.ReplaceAll(string(*s), ",", "_"))
}
