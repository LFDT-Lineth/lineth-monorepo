package fri

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
)

// The reflection-based mutation test below treats field.Octuplet and field.Ext
// as atomic leaves (they hold unexported field storage, so we mutate the whole
// value rather than descend into them).
var (
	octupletType = reflect.TypeOf(field.Octuplet{})
	extType      = reflect.TypeOf(field.Ext{})
)

type mutationKind int

const (
	mutateValue mutationKind = iota // change an atomic leaf (Octuplet/Ext)
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
	return t == octupletType || t == extType
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
		for i := 0; i < v.NumField(); i++ {
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
		for i := 0; i < v.Len(); i++ {
			collectMutations(v.Index(i), append(path, i), fmt.Sprintf("%s[%d]", name, i), out)
		}
	case reflect.Array:
		for i := 0; i < v.Len(); i++ {
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
func safeVerify(p Params, levelRoots []field.Octuplet, levelDs []int,
	prf Proof, alphas []field.Ext, positions []int) (err error, panicked bool) {

	defer func() {
		if r := recover(); r != nil {
			panicked = true
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	err = Verify(p, levelRoots, levelDs, prf, alphas, positions)
	return
}

// TestVerifyRejectsProofMutations is mutation testing over the Proof object: any
// single-field change — a tweaked leaf/root/final value, or a dropped/duplicated
// entry in any vector — must be rejected by Verify, and never panic. A panic
// signals a missing structural check; an acceptance signals a soundness hole.
func TestVerifyRejectsProofMutations(t *testing.T) {

	prng := rand.New(utils.NewRandSource(20240607))
	p, err := NewParams(16, 8, 4)
	if err != nil {
		t.Fatalf("NewParams: %v", err)
	}

	// One main level (D=8) plus one extra level (D=2) to also exercise the
	// LevelQueries path.
	levels := []Level{newRandomLevel(prng, p, 8), newRandomLevel(prng, p, 2)}
	alphas := make([]field.Ext, p.numRounds)
	for i := range alphas {
		alphas[i] = field.PseudoRandExt(prng)
	}
	// Positions chosen to probe every final-layer index (size N>>numRounds = 2),
	// so that mutating any FinalPolyExt entry is detected by some query.
	positions := []int{1, 5, 9, 13}

	// Canonical proof (Prove sorts levels in place, so derive verifier inputs after).
	base := proverForTest(p, levels, alphas, positions)
	levelRoots := make([]field.Octuplet, len(levels))
	levelDs := make([]int, len(levels))
	for i := range levels {
		levelRoots[i] = levels[i].Tree.Root()
		levelDs[i] = levels[i].D
	}
	if err := Verify(p, levelRoots, levelDs, base, alphas, positions); err != nil {
		t.Fatalf("honest proof was rejected: %v", err)
	}

	var muts []proofMutation
	collectMutations(reflect.ValueOf(&base).Elem(), nil, "Proof", &muts)
	if len(muts) == 0 {
		t.Fatal("no mutations were collected")
	}

	for _, m := range muts {
		t.Run(m.name, func(t *testing.T) {
			// Re-derive the canonical proof deterministically, then mutate it.
			prf := proverForTest(p, levels, alphas, positions)
			applyMutation(reflect.ValueOf(&prf).Elem(), m)

			err, panicked := safeVerify(p, levelRoots, levelDs, prf, alphas, positions)
			if panicked {
				t.Fatalf("mutation made Verify panic (missing structural check): %v", err)
			}
			if err == nil {
				t.Fatalf("mutation was accepted by Verify (soundness hole)")
			}
		})
	}
}
