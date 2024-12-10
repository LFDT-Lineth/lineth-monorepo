package publicInput

import (
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// FunctionalInputExtractor is a collection over LocalOpeningQueries that can be
// used to check the values contained in the Wizard witness are consistent with
// the statement of the outer-proof.
type FunctionalInputExtractor struct {

	// DataNbBytes fetches the byte size of the execution data. It is important
	// to include it as the execution data hashing would be vulnerable to padding
	// attacks.
	DataNbBytes query.LocalOpening

	// DataChecksum returns the hash of the execution data
	DataChecksum query.LocalOpening

	// L2MessagesHash is the hash of the hashes of the L2 messages. Each message
	// hash is encoded as 2 field elements, thus the hash does not need padding.
	//
	// NB: the corresponding field in [FunctionalPublicInputSnark] is the list
	// the individual L2 messages hashes.
	L2MessageHash query.LocalOpening

	// InitialStateRootHash and FinalStateRootHash are resp the initial and
	// root hash of the state for the
	InitialStateRootHash, FinalStateRootHash                  query.LocalOpening
	InitialBlockNumber, FinalBlockNumber                      query.LocalOpening
	InitialBlockTimestamp, FinalBlockTimestamp                query.LocalOpening
	FirstRollingHashUpdate, LastRollingHashUpdate             [2]query.LocalOpening
	FirstRollingHashUpdateNumber, LastRollingHashUpdateNumber query.LocalOpening

	ChainID                query.LocalOpening
	NBytesChainID          query.LocalOpening
	L2MessageServiceAddrHi ifaces.Accessor
	L2MessageServiceAddrLo ifaces.Accessor
}

// Run assigns all the local opening queries
func (fie *FunctionalInputExtractor) Run(run *wizard.ProverRuntime) {

	assignLO := func(q query.LocalOpening) {
		run.AssignLocalPoint(q.ID, q.Pol.GetColAssignmentAt(run, 0))
	}

	assignLO(fie.DataNbBytes)
	assignLO(fie.DataChecksum)
	assignLO(fie.L2MessageHash)
	assignLO(fie.InitialStateRootHash)
	assignLO(fie.InitialBlockNumber)
	assignLO(fie.InitialBlockTimestamp)
	assignLO(fie.FirstRollingHashUpdate[0])
	assignLO(fie.FirstRollingHashUpdate[1])
	assignLO(fie.FirstRollingHashUpdateNumber)
	assignLO(fie.FinalStateRootHash)
	assignLO(fie.FinalBlockNumber)
	assignLO(fie.FinalBlockTimestamp)
	assignLO(fie.LastRollingHashUpdate[0])
	assignLO(fie.LastRollingHashUpdate[1])
	assignLO(fie.LastRollingHashUpdateNumber)
	assignLO(fie.ChainID)
	assignLO(fie.NBytesChainID)
}
