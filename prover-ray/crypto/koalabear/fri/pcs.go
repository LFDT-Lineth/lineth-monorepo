package fri

import "github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"

type Layout []struct {
	// the index of the size of the polynomial, size(poly) = 2^sizeIndex
	SizeIndex            int
	PositionInSizeBucket int
}

type Commitment struct {
	MerkleRoot field.Octuplet
	Layout     Layout
}

// tracks the prover informations about the committed values like the RS encoding
// or the rows and the merkle trees.
type CommitterState struct{}
type PolynomialCommitmentScheme struct{}
type PcsProverState struct{}
type PositionOpening struct {
	FriOpening Query
	EvalOPening struct{
		MerkleBranch
		[][][]field.Element,
	}
}

type Claim struct {
	X []field.Ext
	Y []field.Ext
}

func Commit(polys []field.Vec) (Commitment, CommitterState)

// initializes the prover state with the witness
// identifies the biggest layer
// compute the folded quotient for that layer (combining all the round) using alpha
// 		-> running = \sum_i \alpha^i (P_i - y_i) / (X - x_i) [for every claim i for the polynomials of the large size]
// merkleize the folded quotient
// track the folded quotient as the "running" in FRI
//
// Do we do this as a separate step or fold it in the Fold fiunction?
func StartOpening([]Commitment, []CommitterState, [][]Claim, alpha_0 field.Ext) (*PcsProverState, friCommitment field.Octuplet) 

// even, odd <- split(running_i)
// running_(i+1) <- even + \alpha_i odd + \alpha_i^2 \sum_i \alpha^i (P_i - y_i) / (X - x_i) [for every claim i for the polynomials whose size match the one of even and odd]
func (state *PcsProverState) Fold(alpha2 field.Ext) (friCommitment field.Octuplet)

func (state *PcsProverState) OpenPositions(positions []int) []PositionOpening


