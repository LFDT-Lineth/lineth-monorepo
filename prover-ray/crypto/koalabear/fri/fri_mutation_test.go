package fri

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/stretchr/testify/require"
)

// The reflection-based mutation test below treats field.Octuplet, field.Ext,
// and field.Element as atomic leaves.
var (
	octupletType = reflect.TypeOf(field.Octuplet{})
	extType      = reflect.TypeOf(field.Ext{})
	elementType  = reflect.TypeOf(field.Element{})
)

type mutationKind int

const (
	mutateValue mutationKind = iota // change an atomic leaf (Octuplet/Ext/Element)
	mutateDrop                      // drop the last element of a slice
	mutateDup                       // duplicate the last element of a slice
)

// proofMutation describes a single mutation by the path (sequence of struct-field
// / slice indices) to the target inside a Proof and the kind of change.
type proofMutation struct {
	name string
	path []int
	kind mutationKind
}

func isAtomicLeaf(t reflect.Type) bool {
	return t == octupletType || t == extType || t == elementType
}

// collectMutations walks v (a Proof) and records every value mutation (one per
// atomic leaf) and every length mutation (drop + duplicate per non-empty slice).
func collectMutations(v reflect.Value, path []int, name string, out *[]proofMutation) {

	if isAtomicLeaf(v.Type()) {
		*out = append(*out, proofMutation{name, clonePath(path), mutateValue})
		return
	}

	switch v.Kind() {
	case reflect.Struct:
		for i := range v.NumField() {
			if !v.Type().Field(i).IsExported() {
				continue
			}
			collectMutations(v.Field(i), append(path, i),
				name+"."+v.Type().Field(i).Name, out)
		}
	case reflect.Slice:
		if v.Len() > 0 {
			*out = append(*out, proofMutation{name + "[drop]", clonePath(path), mutateDrop})
			*out = append(*out, proofMutation{name + "[dup]", clonePath(path), mutateDup})
		}
		for i := range v.Len() {
			collectMutations(v.Index(i), append(path, i), fmt.Sprintf("%s[%d]", name, i), out)
		}
	case reflect.Array:
		for i := range v.Len() {
			collectMutations(v.Index(i), append(path, i), fmt.Sprintf("%s[%d]", name, i), out)
		}
	default:
		// pointers (e.g. nil AuxSiblings entries) and scalars carry no mutation.
	}
}

func clonePath(p []int) []int {
	c := make([]int, len(p))
	copy(c, p)
	return c
}

// navigate descends from root following the path, choosing field vs index access
// by the value's kind at each step.
func navigate(root reflect.Value, path []int) reflect.Value {
	v := root
	for _, step := range path {
		switch v.Kind() {
		case reflect.Struct:
			v = v.Field(step)
		case reflect.Slice, reflect.Array:
			v = v.Index(step)
		default:
			panic(fmt.Sprintf("navigate: cannot descend into %s", v.Kind()))
		}
	}
	return v
}

func applyMutation(root reflect.Value, m proofMutation) {
	v := navigate(root, m.path)
	switch m.kind {
	case mutateValue:
		switch x := v.Interface().(type) {
		case field.Octuplet:
			one := field.One()
			x[0].Add(&x[0], &one)
			v.Set(reflect.ValueOf(x))
		case field.Ext:
			one := field.Lift(field.One())
			x.Add(&x, &one)
			v.Set(reflect.ValueOf(x))
		case field.Element:
			one := field.One()
			x.Add(&x, &one)
			v.Set(reflect.ValueOf(x))
		default:
			panic(fmt.Sprintf("applyMutation: unexpected atomic type %T", x))
		}
	case mutateDrop:
		v.Set(v.Slice(0, v.Len()-1))
	case mutateDup:
		v.Set(reflect.Append(v, v.Index(v.Len()-1)))
	}
}

// nolint -- ignores: error should be the last return parameters
func safeVerify(fx *ldtFixture, alphaDeep field.Ext, foldAlphas []field.Ext,
	positions []int, proof OpeningProof) (err error, panicked bool) {

	defer func() {
		if r := recover(); r != nil {
			panicked = true
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	err = fx.verify(alphaDeep, foldAlphas, positions, proof)
	return
}

// TestVerifyRejectsProofMutations mutates the PCS OpeningProof: every single
// field or length change must be rejected by pcs.Verify without panicking.
func TestVerifyRejectsProofMutations(t *testing.T) {

	prng := rand.New(utils.NewRandSource(20240607))

	// One main level (D=8) plus one extra level (D=2) to also exercise the
	// LevelQueries path.
	fx := newLDTFixture(t, 16, 8, 4)
	fx.addLevel(t, 3, field.VecPseudoRandExt(prng, 16))
	fx.addLevel(t, 1, field.VecPseudoRandExt(prng, 4))

	alphaDeep := field.PseudoRandExt(prng)
	foldAlphas := make([]field.Ext, fx.pcs.Params.numRounds)
	for i := range foldAlphas {
		foldAlphas[i] = field.PseudoRandExt(prng)
	}
	// Positions chosen to probe every final-layer index (size N>>numRounds = 2),
	// so that mutating any FinalPolyExt entry is detected by some query.
	positions := []int{1, 5, 9, 13}

	// Canonical proof: fx.open is deterministic given the same challenges, so
	// it can be re-derived per mutation below without re-registering openings.
	base := fx.open(t, alphaDeep, foldAlphas, positions)
	require.NoError(t, fx.verify(alphaDeep, foldAlphas, positions, base))

	var muts []proofMutation
	collectMutations(reflect.ValueOf(&base).Elem(), nil, "OpeningProof", &muts)
	require.NotEmpty(t, muts)

	for _, m := range muts {
		t.Run(m.name, func(t *testing.T) {
			// Re-derive the canonical proof deterministically, then mutate it.
			proof := fx.open(t, alphaDeep, foldAlphas, positions)
			applyMutation(reflect.ValueOf(&proof).Elem(), m)

			err, panicked := safeVerify(fx, alphaDeep, foldAlphas, positions, proof)
			require.False(t, panicked, "mutation made Verify panic: %v", err)
			require.Error(t, err)
		})
	}
}

func TestPCSVerifyRejectsMutations(t *testing.T) {
	one := field.One()
	oneExt := field.Lift(one)

	tests := []struct {
		name    string
		mutate  func(*pcsOpenVerifyFixture)
		wantErr string
	}{
		{
			name: "wrong claim",
			mutate: func(fx *pcsOpenVerifyFixture) {
				fx.input.ClaimedValues[0][2].Ext[0][0].Add(&fx.input.ClaimedValues[0][2].Ext[0][0], &oneExt)
			},
			wantErr: "folded value mismatch",
		},
		{
			name: "tampered branch",
			mutate: func(fx *pcsOpenVerifyFixture) {
				leaf := fx.proof.FRIProof.FRIQueries[0][0][0].Leaf
				leaf[0].Add(&leaf[0], &one)
				fx.proof.FRIProof.FRIQueries[0][0][0].Leaf = leaf
			},
			wantErr: "Merkle proof invalid",
		},
		{
			name: "garbage unused row slot",
			mutate: func(fx *pcsOpenVerifyFixture) {
				fx.proof.RowOpenings[0][0][0].Leaf.Ext = []field.Ext{oneExt}
			},
			wantErr: "row shape mismatch",
		},
		{
			name: "sibling on non-top row slot",
			mutate: func(fx *pcsOpenVerifyFixture) {
				fx.proof.RowOpenings[0][0][1].Sibling.Ext = []field.Ext{oneExt}
			},
			wantErr: "sibling shape mismatch",
		},
		{
			name: "domain point claim",
			mutate: func(fx *pcsOpenVerifyFixture) {
				fx.input.Zeta = domainPointExt(fx.pcs.Params.domainsLight[0], 0)
			},
			wantErr: "claim point on domain",
		},
		{
			name: "zero zeta with multiple shifts",
			mutate: func(fx *pcsOpenVerifyFixture) {
				fx.input.Zeta = field.Ext{}
				fx.input.Shifts[0][2].Ext[0] = []int{0, 1}
			},
			wantErr: "shifts with zero zeta",
		},
		{
			name: "alpha mismatch",
			mutate: func(fx *pcsOpenVerifyFixture) {
				fx.input.Challenges.AlphaDeep = field.UintsToExt(41, 0, 0, 0, 0, 0)
			},
			wantErr: "folded value mismatch",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fx := newPCSOpenVerifyFixture(t)
			tc.mutate(&fx)
			err := fx.pcs.Verify(fx.input, fx.proof)
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
