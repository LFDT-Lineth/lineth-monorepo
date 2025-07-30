package logs

import (
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
	util "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/utilities"
)

// LogHasher is used to MiMC-hash the data in LogMessages. Using a zero initial stata,
// the data in L2L1 logs must be hashed as follows:
// msg1HashHi, msg1HashLo, msg2HashHi, msg2HashLo, and so on.
// The hashing proceeds row by row. For row i, after hashing msgHashHi[i], the result is put into hashFirst[i].
// hashFirst[i] is then used as the state to hash msgHashLo[i], and the result is put into hashSecond[i]
// Finally, hashSecond[i] is used as the initial state for hashing msgHashHi[i+1]
// Here is a diagram of how the values are computed
// state:          0            x1           x1'       x2
// block:       msg1Hi         msg1Lo      msg2Hi    msg2Lo     ....
// MimcOutput     x1            x1'          x2        x2'
// x1, x2 will correspond to the values stored in hashFirst
// x1', x2' will correspond to values stored in hashSecond
// The final value of the chained hash can be retrieved as ---> hashSecond[ctMax[any index]]
type LogHasher struct {
	// the hash value after each step, as explained in the description of LogHasher
	HashFirst, HashSecond ifaces.Column
	// L2L1 logs: Inter is a shifted version of hashSecond, necessary due to how the MiMC constraints operate
	Inter ifaces.Column
	// the relevant value of the hash (the last value when isActive ends)
	HashFinal ifaces.Column
}

// NewLogHasher returns a new LogHasher with initialized columns that are not constrained.
func NewLogHasher(comp *wizard.CompiledIOP, size int, name string) LogHasher {
	return LogHasher{
		HashFirst:  util.CreateCol(name, "HASH_FIRST", size, comp),
		HashSecond: util.CreateCol(name, "HASH_SECOND", size, comp),
		Inter:      util.CreateCol(name, "INTER", size, comp),
		HashFinal:  util.CreateCol(name, "HASH_FINAL", size, comp),
	}
}

// DefineHasher specifies the constraints of the LogHasher with respect to the ExtractedData fetched from the arithmetization
func DefineHasher(comp *wizard.CompiledIOP, hasher LogHasher, name string, fetched ExtractedData) {

	// Needed for the limitless prover to understand that the columns are
	// not just empty with just padding and suboptimal representation.
	pragmas.MarkRightPadded(hasher.Inter)
	pragmas.MarkRightPadded(hasher.HashFirst)
	pragmas.MarkRightPadded(hasher.HashSecond)

	// MiMC constraints
	comp.InsertMiMC(0, ifaces.QueryIDf("%s_%s", name, "MIMC_CONSTRAINT"), fetched.Hi, hasher.Inter, hasher.HashFirst, fetched.FilterFetched)
	comp.InsertMiMC(0, ifaces.QueryIDf("%s_%s", name, "MIMC_CONSTRAINT_SECOND"), fetched.Lo, hasher.HashFirst, hasher.HashSecond, fetched.FilterFetched)

	// intermediary state integrity
	comp.InsertGlobal(0, ifaces.QueryIDf("%s_%s", name, "CONSISTENCY_INTER_AND_HASH_LAST"), // LAST is either hashSecond or hashThird
		sym.Mul(
			sym.Sub(column.Shift(hasher.HashSecond, -1), hasher.Inter),
			fetched.FilterFetched,
		),
	)

	// inter, the old state column, is initially zero
	comp.InsertLocal(0, ifaces.QueryIDf("%s_%s", name, "INTER_LOCAL"), ifaces.ColumnAsVariable(hasher.Inter))

	// constrain HashFinal
	commonconstraints.MustBeConstant(comp, hasher.HashFinal)
	util.CheckLastELemConsistency(comp, fetched.FilterFetched, hasher.HashSecond, hasher.HashFinal, name)
}

// AssignHasher assigns the data in the LogHasher using the ExtractedData fetched from the arithmetization
func AssignHasher(run *wizard.ProverRuntime, hasher LogHasher, fetched ExtractedData) {
	size := fetched.Hi.Size()

	hashFirst := common.NewVectorBuilder(hasher.HashFirst)
	hashSecond := common.NewVectorBuilder(hasher.HashSecond)
	inter := common.NewVectorBuilder(hasher.Inter)

	var (
		hashFinal field.Element
	)

	state := field.Zero() // the initial state is zero
	for i := 0; i < size; i++ {

		isActive := fetched.FilterFetched.GetColAssignmentAt(run, i)
		if isActive.IsZero() {
			break
		}

		// Inter is either the initial state so (zero) for row i=0 or the last
		// state from row i-1
		inter.PushField(state)

		// first, hash the HI part of the fetched log message
		state = mimc.BlockCompression(state, fetched.Hi.GetColAssignmentAt(run, i))
		hashFirst.PushField(state)

		// secondly, hash the Lo part of the fetched log message
		state = mimc.BlockCompression(state, fetched.Lo.GetColAssignmentAt(run, i))
		hashSecond.PushField(state)

		// continuously update HashFinal. AthFinal will be the last value of
		// hashSecond where isActive ends
		hashFinal.Set(&state)
	}

	// assign the hasher columns
	inter.PadAndAssign(run, field.Zero())
	hashSecond.PadAndAssign(run, field.Zero())
	hashFirst.PadAndAssign(run, field.Zero())
	run.AssignColumn(hasher.HashFinal.GetColID(), smartvectors.NewConstant(hashFinal, size))
}
