package logdata

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/sirupsen/logrus"
)

// Log is a [wizard.Compiler] implementation which logs metadata and
// stats about the wizard in its current step of compilation.
func Log(msg string) func(comp *wizard.CompiledIOP) {
	return func(comp *wizard.CompiledIOP) {
		cellCount := GetWizardStats(comp)
		logrus.Infof("[wizard.analytic] msg=%v cell-count=%+v", msg, cellCount)
	}
}

// WizardStats is a struct storing numerous metrics pertaining to a
// compiled-IOP.
type WizardStats struct {
	NumColumnsCommitted        int
	NumColumnsProof            int
	NumColumnsPrecomputed      int
	NumColumnsVerificationKeys int
	NumCellsCommitted          int
	NumCellsProof              int
	NumCellsPrecomputed        int
	NumCellsVerificationKeys   int
	NumResultsInnerProduct     int
	NumResultUnivariate        int
	NumResultLocalOpening      int
	NumResultLogDerivativeSum  int
	NumResultGrandProduct      int
	NumResultHorner            int
	NumQueriesInnerProduct     int
	NumQueriesUnivariate       int
	NumQueriesLocalOpening     int
	NumQueriesLogDerivativeSum int
	NumQueriesGrandProduct     int
	NumQueriesHorner           int
	Transcript                 []TranscriptStats
}
type TranscriptStats struct {
	Round                  int
	NumFieldWritten        int
	NumFieldSampled        int
	WeightColumns          int
	WeightUnivariate       int
	WeightInnerProduct     int
	WeightHorner           int
	WeightLogDerivativeSum int
	WeightLocalOpenings    int
	WeightGrandProduct     int
}

// GetWizardStats counts the cells occuring in the provided wizard-IOP.
func GetWizardStats(comp *wizard.CompiledIOP) *WizardStats {

	var (
		listOfColumns = comp.Columns.AllKeys()
		listOfQueries = comp.QueriesParams.AllKeys()
		cellCount     = &WizardStats{}
	)

	for _, colName := range listOfColumns {

		var (
			col    = comp.Columns.GetHandle(colName)
			status = comp.Columns.Status(colName)
			size   = col.Size()
		)

		switch status {
		case column.Committed:
			cellCount.NumCellsCommitted += size
			cellCount.NumColumnsCommitted++
		case column.Proof:
			cellCount.NumCellsProof += size
			cellCount.NumColumnsProof++
		case column.Precomputed:
			cellCount.NumCellsPrecomputed += size
			cellCount.NumColumnsPrecomputed++
		case column.VerifyingKey:
			cellCount.NumCellsVerificationKeys += size
			cellCount.NumColumnsVerificationKeys++
		}
	}

	for _, qName := range listOfQueries {

		q := comp.QueriesParams.Data(qName)

		switch qParams := q.(type) {
		case query.UnivariateEval:
			cellCount.NumQueriesUnivariate++
			cellCount.NumResultUnivariate += len(qParams.Pols)
		case query.InnerProduct:
			cellCount.NumQueriesInnerProduct++
			cellCount.NumResultsInnerProduct += len(qParams.Bs)
		case query.LocalOpening:
			cellCount.NumQueriesLocalOpening++
			cellCount.NumResultLocalOpening++
		case query.LogDerivativeSum:
			cellCount.NumQueriesLogDerivativeSum++
			cellCount.NumResultLogDerivativeSum++
		case query.GrandProduct:
			cellCount.NumQueriesGrandProduct++
			cellCount.NumResultGrandProduct++
		case *query.Horner:
			cellCount.NumQueriesHorner++
			cellCount.NumResultHorner += 1 + 2*len(qParams.Parts)
		}
	}

	numRounds := comp.NumRounds()
	for round := 0; round < numRounds; round++ {

		var (
			tr = TranscriptStats{
				Round: round,
			}
		)

		msgsToFS := comp.Columns.AllKeysInProverTranscript(round)
		for _, msgName := range msgsToFS {

			if comp.Columns.IsExplicitlyExcludedFromProverFS(msgName) {
				continue
			}

			if comp.Precomputed.Exists(msgName) {
				continue
			}

			col := comp.Columns.GetHandle(msgName)
			tr.NumFieldWritten += col.Size()
			tr.WeightColumns += col.Size()
		}

		paramsToFS := comp.QueriesParams.AllKeysAt(round)
		for _, qName := range paramsToFS {
			if comp.QueriesParams.IsSkippedFromProverTranscript(qName) {
				continue
			}

			q_ := comp.QueriesParams.Data(qName)

			switch q := q_.(type) {
			case query.UnivariateEval:
				tr.NumFieldWritten += len(q.Pols)
				tr.WeightUnivariate += len(q.Pols)
			case query.InnerProduct:
				tr.NumFieldWritten += len(q.Bs)
				tr.WeightInnerProduct += len(q.Bs)
			case *query.Horner:
				w := 1 + 2*len(q.Parts)
				tr.NumFieldWritten += w
				tr.WeightHorner += w
			case query.LocalOpening:
				tr.NumFieldWritten++
				tr.WeightLocalOpenings++
			case query.LogDerivativeSum:
				tr.NumFieldWritten++
				tr.WeightLogDerivativeSum++
			case query.GrandProduct:
				tr.NumFieldWritten++
				tr.WeightGrandProduct++
			}
		}

		toCompute := comp.Coins.AllKeysAt(round)
		for _, myCoin := range toCompute {
			if comp.Coins.IsSkippedFromProverTranscript(myCoin) {
				continue
			}

			info := comp.Coins.Data(myCoin)
			if info.Type == coin.Field || info.Type == coin.FieldFromSeed {
				tr.NumFieldSampled++
			} else {

				var (
					fieldBits    = field.Bits
					smallIntBits = utils.Log2Ceil(info.UpperBound)
					toSample     = utils.DivCeil(info.Size*smallIntBits, fieldBits)
				)

				tr.NumFieldSampled += toSample
			}
		}

		cellCount.Transcript = append(cellCount.Transcript, tr)
	}

	return cellCount
}

// TotalCells returns the total number of cells recorded in the wizard.
func (c *WizardStats) TotalCells() int {
	return c.NumCellsCommitted +
		c.NumCellsPrecomputed +
		c.NumCellsProof
}

// TotalFSStats returns the fiat-shamir stats for every rounds
func (c *WizardStats) TotalFSStats() (numWritten, numSampled int) {

	for _, t := range c.Transcript {
		numWritten += t.NumFieldWritten
		numSampled += t.NumFieldSampled
	}

	return numWritten, numSampled
}
