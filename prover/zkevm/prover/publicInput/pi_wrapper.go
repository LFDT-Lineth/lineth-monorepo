package publicInput

import (
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	invalidity "github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput/invalidity_pi"
	"github.com/ethereum/go-ethereum/common"
)

// PublicInputs holds both the execution and invalidity public input modules.
// Both are always constructed together so that the invalidity PI can reuse
// shared fetchers (logs, root hash) from the execution PI.
type PublicInputs struct {
	ExecutionPI  PublicInput
	InvalidityPI *invalidity.InvalidityPI
}

// Assign assigns values to both the execution and invalidity public input columns.
func (p *PublicInputs) Assign(run *wizard.ProverRuntime, l2BridgeAddress common.Address, blockHashList []types.FullBytes32, execData fext.Element) {
	p.ExecutionPI.Assign(run, l2BridgeAddress, blockHashList, execData)
	p.InvalidityPI.Assign(run)
}
