package fiatshamir

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFiatShamirSafeguardUpdate(t *testing.T) {

	fs := NewMiMCFiatShamir()

	a := fs.RandomField()
	b := fs.RandomField()

	// Two consecutive call to fs do not return the same result
	require.NotEqual(t, a.String(), b.String())
}

func TestFiatShamirRandomVec(t *testing.T) {

	for _, testCase := range randomIntVecTestCases {
		testName := fmt.Sprintf("%v-integers-of-%v-bits", testCase.NumIntegers, testCase.IntegerBitSize)
		t.Run(testName, func(t *testing.T) {

			fs := NewMiMCFiatShamir()
			fs.Update(field.NewElement(420))

			var oldState, newState field.Element

			if err := oldState.SetBytesCanonical(fs.hasher.Sum(nil)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			a := fs.RandomManyIntegers(testCase.NumIntegers, 1<<testCase.IntegerBitSize)
			t.Logf("the generated vector= %v", a)

			if err := newState.SetBytesCanonical(fs.hasher.Sum(nil)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			assert.Len(t, a, testCase.NumIntegers)
			assert.NoError(t, errIfHasDuplicate(a...))

			if testCase.ShouldUpdate {
				assert.NotEqualf(t, oldState.String(), newState.String(), "the state was not updated (%v)", oldState.String())
			} else {
				assert.Equal(t, oldState.String(), newState.String(), "the state was updated")
			}
		})
	}
}

func TestBatchUpdates(t *testing.T) {

	fs := NewMiMCFiatShamir()
	fs.Update(field.NewElement(2))
	fs.Update(field.NewElement(2))
	fs.Update(field.NewElement(1))
	expectedVal := fs.RandomField()

	t.Run("for a variadic call", func(t *testing.T) {
		fs := NewMiMCFiatShamir()
		fs.Update(field.NewElement(2), field.NewElement(2), field.NewElement(1))
		actualValue := fs.RandomField()
		assert.Equal(t, expectedVal.String(), actualValue.String())
	})

	t.Run("for slice of field elements", func(t *testing.T) {
		fs := NewMiMCFiatShamir()
		fs.UpdateVec(vector.ForTest(2, 2, 1))
		actualValue := fs.RandomField()
		assert.Equal(t, expectedVal.String(), actualValue.String())
	})

	t.Run("for multi-slice of field elements", func(t *testing.T) {
		fs := NewMiMCFiatShamir()
		fs.UpdateVec(vector.ForTest(2, 2), vector.ForTest(1))
		actualValue := fs.RandomField()
		assert.Equal(t, expectedVal.String(), actualValue.String())
	})

	t.Run("for a smart-vector", func(t *testing.T) {
		sv := smartvectors.RightPadded(vector.ForTest(2, 2), field.NewElement(1), 3)
		fs := NewMiMCFiatShamir()
		fs.UpdateSV(sv)
		actualValue := fs.RandomField()
		assert.Equal(t, expectedVal.String(), actualValue.String())
	})

}

// TestSamplingFromSeed tests that sampling from seed is consistent, does not forget
// bytes from the seed and does not depends on the state.
func TestSamplingFromSeed(t *testing.T) {

	t.Run("dependence-on-name", func(t *testing.T) {

		var (
			// #nosec G404 --we don't need a cryptographic RNG for testing purpose
			rng          = rand.New(utils.NewRandSource(789))
			initialState = field.PseudoRand(rng)
			seed         = field.PseudoRand(rng)
			resMap       = map[field.Element]struct{}{}
			name         = ""
			totalSize    = 1000
		)

		fs := NewMiMCFiatShamir()
		fs.SetState([]field.Element{initialState})

		for i := 0; i < totalSize; i++ {

			x := fs.RandomFieldFromSeed(seed, name)
			if _, found := resMap[x]; found {
				t.Errorf("found a collision for i=%v", i)
			}

			resMap[x] = struct{}{}
			name += "a"
		}
	})

	t.Run("non-dependance-curr-state", func(t *testing.T) {

		var (
			// #nosec G404 --we don't need a cryptographic RNG for testing purpose
			rng    = rand.New(utils.NewRandSource(789))
			state1 = field.PseudoRand(rng)
			state2 = field.PseudoRand(rng)
			fs1    = NewMiMCFiatShamir()
			fs2    = NewMiMCFiatShamir()
			seed   = field.PseudoRand(rng)
			name   = "string-name"
		)

		fs1.SetState([]field.Element{state1})
		fs2.SetState([]field.Element{state2})

		y1 := fs1.RandomFieldFromSeed(seed, name)
		y2 := fs2.RandomFieldFromSeed(seed, name)

		if y1 != y2 {
			t.Errorf("starting from different state does not give the same results")
		}
	})

	t.Run("does-not-modify-state", func(t *testing.T) {

		var (
			// #nosec G404 --we don't need a cryptographic RNG for testing purpose
			rng          = rand.New(utils.NewRandSource(789))
			initialState = field.PseudoRand(rng)
			seed         = field.PseudoRand(rng)
			name         = "ddqsdjqskljd"
			fs           = NewMiMCFiatShamir()
		)

		fs.SetState([]field.Element{initialState})

		fs.RandomFieldFromSeed(seed, name)

		newState := fs.State()
		if initialState != newState[0] {
			t.Errorf("state was modified")
		}
	})

	t.Run("is-repeatable", func(t *testing.T) {

		var (
			// #nosec G404 --we don't need a cryptographic RNG for testing purpose
			rng          = rand.New(utils.NewRandSource(789))
			initialState = field.PseudoRand(rng)
			seed         = field.PseudoRand(rng)
			name         = "ddqsdjqskljd"
			fs           = NewMiMCFiatShamir()
		)

		fs.SetState([]field.Element{initialState})

		y1 := fs.RandomFieldFromSeed(seed, name)
		y2 := fs.RandomFieldFromSeed(seed, name)

		if y1 != y2 {
			t.Errorf("state was modified")
		}
	})

}

// errIfHasDuplicate returns an error if a duplicate is found in the caller's slice
// indicating the positions and the value corresponding to the first duplicate.
func errIfHasDuplicate[T comparable](slice ...T) error {
	m := map[T]int{}
	for i, f := range slice {
		if prevPos, ok := m[f]; ok {
			return fmt.Errorf("found a duplicate value: %v at positions %v and %v", f, prevPos, i)
		}

		m[f] = i
	}
	return nil
}

// Test vectors that are used to test the RandomIntVec feature
var randomIntVecTestCases = []struct {
	ShouldUpdate   bool
	NumIntegers    int
	IntegerBitSize int
}{
	{
		ShouldUpdate:   false,
		NumIntegers:    0,
		IntegerBitSize: 16,
	},
	{
		ShouldUpdate:   true,
		NumIntegers:    1,
		IntegerBitSize: 16,
	},
	{
		ShouldUpdate:   true,
		NumIntegers:    15,
		IntegerBitSize: 16,
	},
	{
		ShouldUpdate:   true,
		NumIntegers:    16,
		IntegerBitSize: 16,
	},
	{
		ShouldUpdate:   true,
		NumIntegers:    17,
		IntegerBitSize: 16,
	},
	{
		ShouldUpdate:   true,
		NumIntegers:    45,
		IntegerBitSize: 16,
	},
}
