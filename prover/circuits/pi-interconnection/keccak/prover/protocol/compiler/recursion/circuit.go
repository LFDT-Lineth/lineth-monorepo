package recursion

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/fiatshamir"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/mimc/gkrmimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

// RecursionCircuit is a gnark-circuit doing the recursion of a
// wizard-IOP. It embeds the [wizard.VerifierCircuit] and it
// additionally exposes the final FS state, the vortex commitments
// and the vortex statement (in addition to the "regular" public
// inputs of the protocol).
//
// It implements the [frontend.Circuit] interface.
//
// Alex: please don't change the ordering of the arguments as this
// affects the parsing of the witness.
type RecursionCircuit struct {
	X                  frontend.Variable   `gnark:",public"`
	Ys                 []frontend.Variable `gnark:",public"`
	Commitments        []frontend.Variable `gnark:",public"`
	Pubs               []frontend.Variable `gnark:",public"`
	WizardVerifier     *wizard.VerifierCircuit
	withoutGkr         bool                 `gnark:"-"`
	withExternalHasher bool                 `gnark:"-"`
	PolyQuery          query.UnivariateEval `gnark:"-"`
	MerkleRoots        []ifaces.Column      `gnark:"-"`
}

// AllocRecursionCircuit allocates a new RecursionCircuit with the
// given parameters.
func AllocRecursionCircuit(comp *wizard.CompiledIOP, withoutGkr bool, withExternalHasher bool) *RecursionCircuit {

	var (
		pcsCtx      = comp.PcsCtxs.(*vortex.Ctx)
		polyQuery   = pcsCtx.Query
		numRound    = comp.QueriesParams.Round(polyQuery.QueryID) + 1
		merkleRoots = []ifaces.Column{}
	)

	if pcsCtx.Items.Precomputeds.MerkleRoot != nil {
		merkleRoots = append(merkleRoots, pcsCtx.Items.Precomputeds.MerkleRoot)
	}

	for i := range pcsCtx.Items.MerkleRoots {
		if pcsCtx.Items.MerkleRoots[i] != nil {
			merkleRoots = append(merkleRoots, pcsCtx.Items.MerkleRoots[i])
		}
	}

	return &RecursionCircuit{
		withoutGkr:         withoutGkr,
		withExternalHasher: withExternalHasher,
		PolyQuery:          polyQuery,
		MerkleRoots:        merkleRoots,
		WizardVerifier:     wizard.AllocateWizardCircuit(comp, numRound),
		Pubs:               make([]frontend.Variable, len(comp.PublicInputs)),
		Commitments:        make([]frontend.Variable, len(merkleRoots)),
		Ys:                 make([]frontend.Variable, len(polyQuery.Pols)),
	}
}

// Define implements the [frontend.Circuit] interface.
func (r *RecursionCircuit) Define(api frontend.API) error {

	w := r.WizardVerifier

	if !r.withoutGkr {
		w.HasherFactory = gkrmimc.NewHasherFactory(api)
		w.FS = fiatshamir.NewGnarkFiatShamir(api, w.HasherFactory)
	}

	if r.withExternalHasher {
		w.HasherFactory = &mimc.ExternalHasherFactory{Api: api}
	}
	// The below step is responsible for verifying all
	// the verifier actions of all compilation steps.
	w.Verify(api)

	for i := range r.Pubs {
		pub := w.Spec.PublicInputs[i].Acc.GetFrontendVariable(api, w)
		api.AssertIsEqual(r.Pubs[i], pub)
	}

	polyParams := w.GetUnivariateParams(r.PolyQuery.Name())
	api.AssertIsEqual(r.X, polyParams.X)

	for i := range polyParams.Ys {
		api.AssertIsEqual(r.Ys[i], polyParams.Ys[i])
	}

	for i := range r.Commitments {
		api.AssertIsEqual(r.Commitments[i], r.MerkleRoots[i].GetColAssignmentGnarkAt(w, 0))
	}

	return nil
}

// AssignRecursionCircuit assigns a recursion based on a compiled-IOP
// and a proof.
func AssignRecursionCircuit(comp *wizard.CompiledIOP, proof wizard.Proof, pubs []field.Element, finalFsState field.Element) *RecursionCircuit {

	var (
		pcsCtx         = comp.PcsCtxs.(*vortex.Ctx)
		polyQuery      = pcsCtx.Query
		numRound       = comp.QueriesParams.Round(polyQuery.QueryID) + 1
		wizardVerifier = wizard.AssignVerifierCircuit(comp, proof, numRound)
		params         = wizardVerifier.GetUnivariateParams(polyQuery.Name())
		circuit        = &RecursionCircuit{
			WizardVerifier: wizardVerifier,
			X:              params.X,
			Ys:             params.Ys,
			Pubs:           vector.IntoGnarkAssignment(pubs),
			PolyQuery:      polyQuery,
		}
	)

	if pcsCtx.Items.Precomputeds.MerkleRoot != nil {
		mRoot := pcsCtx.Items.Precomputeds.MerkleRoot
		circuit.MerkleRoots = append(circuit.MerkleRoots, mRoot)
		circuit.Commitments = append(circuit.Commitments, mRoot.GetColAssignmentGnarkAt(circuit.WizardVerifier, 0))
	}

	for i := range pcsCtx.Items.MerkleRoots {
		if pcsCtx.Items.MerkleRoots[i] != nil {
			mRoot := pcsCtx.Items.MerkleRoots[i]
			circuit.MerkleRoots = append(circuit.MerkleRoots, mRoot)
			circuit.Commitments = append(circuit.Commitments, mRoot.GetColAssignmentGnarkAt(circuit.WizardVerifier, 0))
		}
	}

	return circuit
}

// SplitPublicInputs parses a vector of field elements and returns the
// parsed arguments.
func SplitPublicInputs[T any](r *Recursion, allPubs []T) (x T, ys, mRoots, pubs []T) {

	var (
		numPubs     = len(r.InputCompiledIOP.PublicInputs)
		pcsCtx      = r.PcsCtx[0]
		numYs       = len(pcsCtx.Query.Pols)
		numMRoots   = 0
		allPubDrain = allPubs
	)

	if pcsCtx.Items.Precomputeds.MerkleRoot != nil {
		numMRoots++
	}

	for i := range pcsCtx.Items.MerkleRoots {
		if pcsCtx.Items.MerkleRoots[i] != nil {
			numMRoots++
		}
	}

	// The order below is based on the field declaration order for the
	// circuit struct.
	//
	// X              frontend.Variable   `gnark:",public"`
	// Ys             []frontend.Variable `gnark:",public"`
	// Commitments    []frontend.Variable `gnark:",public"`
	// Pubs           []frontend.Variable `gnark:",public"`
	//
	x, allPubDrain = allPubDrain[0], allPubDrain[1:]
	ys, allPubDrain = allPubDrain[:numYs], allPubDrain[numYs:]
	mRoots, allPubDrain = allPubDrain[:numMRoots], allPubDrain[numMRoots:]
	pubs, _ = allPubDrain[:numPubs], allPubDrain[numPubs:]

	return x, ys, mRoots, pubs
}
