package globalcs

import (
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"math/big"
	"reflect"
	"runtime"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/consensys/linea-monorepo/prover/maths/common/mempool"
	sv "github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/maths/fft/fastpoly"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/protocol/wizardutils"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
	ppool "github.com/consensys/linea-monorepo/prover/utils/parallel/pool"
	"github.com/consensys/linea-monorepo/prover/utils/profiling"
)

const (
	/*
		Explanation for Manual Garbage Collection Thresholds
	*/
	// These two threshold work well for the real-world traces at the moment of writing and a 340GiB memory limit,
	// but this approach can be generalized and further improved.

	// When ctx.domainSize>=524288, proverEvaluationQueries() experiences a heavy workload,
	// consistently hitting the GOMEMLIMIT of 340GiB.
	// This results in numerous auto GCs during CPU-intensive small tasks, significantly degrading performance.
	// In the benchmark input files, GC_DOMAIN_SIZE >= 524288 means only the first call of proverEvaluationQueries().
	// With ctx.domainSize<=262144, manual GC is not necessary as auto GCs triggered by GOMEMLIMIT suffice.
	GC_DOMAIN_SIZE int = 524288

	// Auto GCs are triggered during ReEvaluate and Batch evaluation
	// when len(handles) exceeds approximately 4000, causing performance degradation.
	// This threshold is set to perform manual GCs before ReEvaluate and Batch evaluation
	// only when len(handles) reaches a size substantial enough to trigger auto GC during ReEvaluate and Batch evaluation.
	// Note that the value of GC_HANDLES_SIZE 4000 is derived from experience and analytics on the benchmark input files.
	GC_HANDLES_SIZE int = 4000
)

// quotientCtx collects all the internal fields needed to compute the quotient
type quotientCtx struct {

	// DomainSize is the domain over which the global constraints are computed
	DomainSize int

	// Ratio lists the ratio found in the global constraints
	//
	// See [mergingCtx.Ratios]
	Ratios []int

	// ColumnsForRatio[k] stores all the columns involved in the aggregate
	// expressions for ratio Ratios[k]
	ColumnsForRatio [][]ifaces.Column

	// RootsPerRatio[k] stores all the root columns involved in the aggregate
	// expressions for ration Ratios[k]. By root column we mean the underlying
	// column that are actually committed to. For instance, if Shift(A, 1) is
	// in ColumnsForRatio[k], we will have A in RootPerRatio[k]
	RootsForRatio [][]ifaces.Column

	// AllInvolvedColumns stores the union of the ColumnForRatio[k] for all k
	AllInvolvedColumns []ifaces.Column

	// AllInvolvedRoots stores the union of the RootsForRatio[k] for all k
	AllInvolvedRoots []ifaces.Column

	// AggregateExpressions[k] stores the aggregate expression for Ratios[k]
	AggregateExpressions []*symbolic.Expression

	// AggregateExpressionsBoard[k] stores the topological sorting of
	// AggregateExpressions[k]
	AggregateExpressionsBoard []symbolic.ExpressionBoard

	// QuotientShares[k] stores for each k, the list of the Ratios[k] shares
	// of the quotient for the AggregateExpression[k]
	QuotientShares [][]ifaces.Column

	// MaxNbExprNode stores the largest number of node AggregateExpressionBoard[*]
	// has. This is used to dimension the memory pool during the prover time.
	MaxNbExprNode int
}

// createQuotientCtx constructs a [quotientCtx] from a list of ratios and aggreated
// expressions. The function organizes the handles but does not declare anything
// in the current wizard.CompiledIOP.
func createQuotientCtx(comp *wizard.CompiledIOP, ratios []int, aggregateExpressions []*symbolic.Expression) quotientCtx {

	var (
		allInvolvedHandlesIndex = map[ifaces.ColID]int{}
		allInvolvedRootsSet     = collection.NewSet[ifaces.ColID]()
		_, _, domainSize        = wizardutils.AsExpr(aggregateExpressions[0])
		ctx                     = quotientCtx{
			DomainSize:                domainSize,
			Ratios:                    ratios,
			AggregateExpressions:      aggregateExpressions,
			AggregateExpressionsBoard: make([]symbolic.ExpressionBoard, len(ratios)),
			AllInvolvedColumns:        []ifaces.Column{},
			AllInvolvedRoots:          []ifaces.Column{},
			ColumnsForRatio:           make([][]ifaces.Column, len(ratios)),
			RootsForRatio:             make([][]ifaces.Column, len(ratios)),
			QuotientShares:            generateQuotientShares(comp, ratios, domainSize),
		}
	)

	for k, expr := range ctx.AggregateExpressions {

		var (
			board               = expr.Board()
			uniqueRootsForRatio = collection.NewSet[ifaces.ColID]()
		)

		ctx.AggregateExpressionsBoard[k] = board
		ctx.MaxNbExprNode = max(ctx.MaxNbExprNode, board.CountNodes())

		// This loop scans the metadata looking for columns with the goal of
		// populating the collections composing quotientCtx.
		for _, metadata := range board.ListVariableMetadata() {

			// Scan in column metadata only
			col, ok := metadata.(ifaces.Column)
			if !ok {
				continue
			}

			var (
				rootCol          = column.RootParents(col)
				isShiftOrNatural = len(rootCol) == 1 && rootCol[0].Size() == col.Size()
			)

			// Append the handle (we trust that there are no duplicate of handles
			// within a constraint). This works because the symbolic package has
			// automatic simplifications routines that ensure that an expression
			// does not refer to duplicates of the same variable.
			ctx.ColumnsForRatio[k] = append(ctx.ColumnsForRatio[k], col)

			if isShiftOrNatural && !uniqueRootsForRatio.Exists(rootCol[0].GetColID()) {
				ctx.RootsForRatio[k] = append(ctx.RootsForRatio[k], rootCol[0])
			}

			// Get the name of the
			if _, alreadyThere := allInvolvedHandlesIndex[col.GetColID()]; alreadyThere {
				continue
			}

			allInvolvedHandlesIndex[col.GetColID()] = len(ctx.AllInvolvedColumns)
			ctx.AllInvolvedColumns = append(ctx.AllInvolvedColumns, col)

			// If the handle is simply a shift or a natural columns tracks its root
			if isShiftOrNatural && !allInvolvedRootsSet.Exists(rootCol[0].GetColID()) {
				allInvolvedRootsSet.Insert(rootCol[0].GetColID())
				ctx.AllInvolvedRoots = append(ctx.AllInvolvedRoots, rootCol[0])
			}
		}
	}

	return ctx
}

// generateQuotientShares declares and returns the quotient share columns
func generateQuotientShares(comp *wizard.CompiledIOP, ratios []int, domainSize int) [][]ifaces.Column {

	var (
		quotientShares = make([][]ifaces.Column, len(ratios))
		currRound      = comp.NumRounds() - 1
	)

	for i, ratio := range ratios {
		quotientShares[i] = make([]ifaces.Column, ratio)
		for k := range quotientShares[i] {
			quotientShares[i][k] = comp.InsertCommit(
				currRound,
				ifaces.ColID(deriveName(comp, QUOTIENT_POLY_TMPL, ratio, k)),
				domainSize,
			)
		}
	}

	return quotientShares
}

// Run implements the [wizard.ProverAction] interface and embeds the logic to
// compute the quotient shares.
func (ctx *quotientCtx) Run(run *wizard.ProverRuntime) {

	logrus.Infof("run the prover for the global constraint (quotient computation)")

	var (
		// Tracks the time spent on garbage collection
		totalTimeGc = int64(0)

		// Initial step is to compute the FFTs for all committed vectors
		coeffs    = sync.Map{} // (ifaces.ColID <=> sv.SmartVector)
		stopTimer = profiling.LogTimer("Computing the coeffs %v pols of size %v", len(ctx.AllInvolvedColumns), ctx.DomainSize)
		pool      = mempool.CreateFromSyncPool(symbolic.MaxChunkSize).Prewarm(runtime.GOMAXPROCS(0) * ctx.MaxNbExprNode)
		largePool = mempool.CreateFromSyncPool(ctx.DomainSize).Prewarm(len(ctx.AllInvolvedColumns))
	)

	if ctx.DomainSize >= GC_DOMAIN_SIZE {
		// Force the GC to run
		tGc := time.Now()
		runtime.GC()
		totalTimeGc += time.Since(tGc).Milliseconds()
		logrus.Infof("global constraints : spent %v ms in gc, total time %v ms", time.Since(tGc), totalTimeGc)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		// Compute once the FFT of the natural columns

		ppool.ExecutePoolChunky(len(ctx.AllInvolvedRoots), func(k int) {
			pol := ctx.AllInvolvedRoots[k]
			name := pol.GetColID()

			// gets directly a shallow copy in the map of the runtime
			var witness sv.SmartVector
			witness, isNatural := run.Columns.TryGet(name)

			// can happen if the column is verifier defined. In that case, no
			// need to protect with a lock. This will not touch run.Columns.
			if !isNatural {
				witness = pol.GetColAssignment(run)
			}

			witness = sv.FFTInverse(witness, fft.DIF, false, 0, 0, nil)

			coeffs.Store(name, witness)
		})

		wg.Done()
	}()

	go func() {
		ppool.ExecutePoolChunky(len(ctx.AllInvolvedColumns), func(k int) {
			pol := ctx.AllInvolvedColumns[k]

			// short-path, the column is a shifted column of an already
			// present columns rule out interleaved and repeated columns.
			rootCols := column.RootParents(pol)
			if len(rootCols) == 1 && rootCols[0].Size() == pol.Size() {
				// It was already processed by the above loop. Go on with the next entry
				return
			}

			// normal case for interleaved or repeated columns
			witness := pol.GetColAssignment(run)

			witness = sv.FFTInverse(witness, fft.DIF, false, 0, 0, nil)

			name := pol.GetColID()

			coeffs.Store(name, witness)
		})
		wg.Done()
	}()

	wg.Wait()
	stopTimer()

	// Take the max quotient degree
	maxRatio := utils.Max(ctx.Ratios...)

	/*
		For the quotient, we precompute the values of (wQ^N - 1)^-1 for w in H, the
		larger domain.

		Those values are D-periodic, thus we only compute a single period.
		(Where D is the ratio of the sizes of the larger and the smaller domain)

		The first value is ignored because it correspond to the case where w^N = 1
		(i.e. w is in the smaller subgroup)
	*/
	annulatorInvVals := fastpoly.EvalXnMinusOneOnACoset(ctx.DomainSize, ctx.DomainSize*maxRatio)
	annulatorInvVals = field.ParBatchInvert(annulatorInvVals, runtime.GOMAXPROCS(0))

	/*
		Also returns the evaluations of
	*/

	for i := 0; i < maxRatio; i++ {

		// use sync map to store the coset evaluated polynomials
		computedReeval := sync.Map{}
		stopTimer = profiling.LogTimer("Creation of omega")

		/*
			The following computes the quotient polynomial and assigns it

			Omega is a root of unity which generates the domain of evaluation of the
			constraint. Its size coincide with the size of the domain of evaluation.
			For each value of `i`, X will evaluate to gen*omegaQ^numCoset*omega^i.

			Gen is a generator of F^*
		*/
		var (
			omega        = fft.GetOmega(ctx.DomainSize)
			omegaQNumCos = fft.GetOmega(ctx.DomainSize * maxRatio)
			omegaI       = field.NewElement(field.MultiplicativeGen)
		)

		omegaQNumCos.Exp(omegaQNumCos, big.NewInt(int64(i)))
		omegaI.Mul(&omegaI, &omegaQNumCos)

		// Precomputations of the powers of omega, can be optimized if useful
		omegas := make([]field.Element, ctx.DomainSize)
		for i := range omegas {
			omegas[i] = omegaI
			omegaI.Mul(&omegaI, &omega)
		}

		stopTimer()

		for j, ratio := range ctx.Ratios {

			// For instance, if deg = 2 and max deg 8, we enter only if
			// i = 0 or 4 because this correspond to the cosets we are
			// interested in.
			if i%(maxRatio/ratio) != 0 {
				continue
			}

			// With the above example, if we are in the ratio = 2 and maxRatio = 8
			// and i = 1 (it can only be 0 <= i < ratio).
			var (
				share     = i * ratio / maxRatio
				handles   = ctx.ColumnsForRatio[j]
				roots     = ctx.RootsForRatio[j]
				board     = ctx.AggregateExpressionsBoard[j]
				metadatas = board.ListVariableMetadata()
			)

			if ctx.DomainSize >= GC_DOMAIN_SIZE {
				// Force the GC to run
				tGc := time.Now()
				runtime.GC()
				totalTimeGc += time.Since(tGc).Milliseconds()
				logrus.Infof("global constraints : spent %v ms in gc, total time %v ms", time.Since(tGc), totalTimeGc)
			}

			stopTimer := profiling.LogTimer("ReEvaluate %v pols of size %v on coset %v/%v", len(handles), ctx.DomainSize, share, ratio)

			ppool.ExecutePoolChunky(len(roots), func(k int) {
				localPool := mempool.WrapsWithMemCache(largePool)
				defer localPool.TearDown()

				root := roots[k]
				name := root.GetColID()

				_, found := computedReeval.Load(name)

				if found {
					// it was already computed in a previous iteration of `j`
					return
				}

				// else it's the first value of j that sees it. so we compute the
				// coset reevaluation.

				v, _ := coeffs.Load(name)
				reevaledRoot := sv.FFT(v.(sv.SmartVector), fft.DIT, false, ratio, share, localPool)
				computedReeval.Store(name, reevaledRoot)
			})

			ppool.ExecutePoolChunky(len(handles), func(k int) {
				localPool := mempool.WrapsWithMemCache(largePool)
				defer localPool.TearDown()

				pol := handles[k]
				// short-path, the column is a purely Shifted(Natural) or a Natural
				// (this excludes repeats and/or interleaved columns)
				rootCols := column.RootParents(pol)
				if len(rootCols) == 1 && rootCols[0].Size() == pol.Size() {

					root := rootCols[0]
					name := root.GetColID()

					reevaledRoot, found := computedReeval.Load(name)

					if !found {
						// it is expected to computed in the above loop
						utils.Panic("did not find the reevaluation of %v", name)
					}

					// Now, we can reuse a soft-rotation of the smart-vector to save memory
					if !pol.IsComposite() {
						// in this case, the right vector was the root so we are done
						return
					}

					if shifted, isShifted := pol.(column.Shifted); isShifted {
						polName := pol.GetColID()
						res := sv.SoftRotate(reevaledRoot.(sv.SmartVector), shifted.Offset)
						computedReeval.Store(polName, res)
						return
					}

				}

				name := pol.GetColID()
				_, ok := computedReeval.Load(name)
				if ok {
					return
				}

				v, ok := coeffs.Load(name)
				if !ok {
					utils.Panic("handle %v not found in the coeffs\n", name)
				}

				res := sv.FFT(v.(sv.SmartVector), fft.DIT, false, ratio, share, localPool)
				computedReeval.Store(name, res)

			})

			stopTimer()

			if len(handles) >= GC_HANDLES_SIZE {
				// Force the GC to run
				tGc := time.Now()
				runtime.GC()
				totalTimeGc += time.Since(tGc).Milliseconds()
				logrus.Infof("global constraints : spent %v ms in gc, total time %v ms", time.Since(tGc), totalTimeGc)
			}

			stopTimer = profiling.LogTimer("Batch evaluation of %v pols of size %v (ratio is %v)", len(handles), ctx.DomainSize, ratio)

			// Evaluates the constraint expression on the coset
			evalInputs := make([]sv.SmartVector, len(metadatas))

			for k, metadataInterface := range metadatas {
				switch metadata := metadataInterface.(type) {
				case ifaces.Column:
					//name := metadata.GetColID()
					//evalInputs[k] = computedReeval[name]
					value, _ := computedReeval.Load(metadata.GetColID())
					evalInputs[k] = value.(sv.SmartVector)
				case coin.Info:
					evalInputs[k] = sv.NewConstant(run.GetRandomCoinField(metadata.Name), ctx.DomainSize)
				case variables.X:
					evalInputs[k] = metadata.EvalCoset(ctx.DomainSize, i, maxRatio, true)
				case variables.PeriodicSample:
					evalInputs[k] = metadata.EvalCoset(ctx.DomainSize, i, maxRatio, true)
				case ifaces.Accessor:
					evalInputs[k] = sv.NewConstant(metadata.GetVal(run), ctx.DomainSize)
				default:
					utils.Panic("Not a variable type %v", reflect.TypeOf(metadataInterface))
				}
			}

			if len(handles) >= GC_HANDLES_SIZE {
				// Force the GC to run
				tGc := time.Now()
				runtime.GC()
				totalTimeGc += time.Since(tGc).Milliseconds()
				logrus.Infof("global constraints : spent %v ms in gc, total time %v ms", time.Since(tGc), totalTimeGc)
			}

			// Note that this will panic if the expression contains "no commitment"
			// This should be caught already by the constructor of the constraint.
			quotientShare := ctx.AggregateExpressionsBoard[j].Evaluate(evalInputs, pool)
			quotientShare = sv.ScalarMul(quotientShare, annulatorInvVals[i])
			run.AssignColumn(ctx.QuotientShares[j][share].GetColID(), quotientShare)

			stopTimer()

		}

		// Forcefuly clean the memory for the computed reevals
		computedReeval.Range(func(k, v interface{}) bool {

			if pooled, ok := v.(*sv.Pooled); ok {
				pooled.Free(largePool)
			}

			computedReeval.Delete(k)
			return true
		})
	}
}
