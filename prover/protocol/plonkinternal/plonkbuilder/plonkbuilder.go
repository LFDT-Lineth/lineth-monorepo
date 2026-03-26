// Package plonkbuilder defines circuit builder for implementing PLONK-in-Wizard
package plonkbuilder

import (
	"fmt"
	"math/big"

	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
)

// Builder is an interface grouping the necessary methods
// for implementing wide commitment over a PLONK-in-Wizard builder.
type Builder interface {
	// PlonkInWizardBuilder wraps the given builder
	PlonkInWizardBuilder
	// WideCommitter allows to commit to variable and obtain extension field element
	frontend.WideCommitter
}

// ensure that the widecommitter struct implements all required interfaces
var _ Builder = &widecommitter{}

// PlonkInWizardBuilder is the interface for a builder for implementing
// PLONK-in-Wizard protocols. It is [frontend.Builder] with some additional
// methods for allowing to:
// - manage key-value pairs storage (redefinition of gnark internal kvstore methods)
// - retrieve wire constraints (for using in range checker implementation)
// - use plonk-specific APIs for manually adding PLONK constraints
// - manage commitment inputs and outputs for PLONK commitments
//
// The builder returned by [scs.NewBuilder] in gnark implements this interface,
// even though not explicitly stated (due to possible misuse outside of Wizard).
// A builder implementing this interface can be provided to [From] to obtain a
// builder implementing [frontend.WideCommitter].
type PlonkInWizardBuilder interface {
	// Builder is the underlying frontend builder
	frontend.Builder[constraint.U32]

	// redefinition of gnark internal [bitsComparatorConstant]
	MustBeLessOrEqCst(aBits []frontend.Variable, bound *big.Int, aForDebug frontend.Variable)

	// redefinition of gnark internal kvstore methods
	SetKeyValue(key, value any)
	GetKeyValue(key any) (value any)

	// GetWireConstraints and GetWiresConstraintExact retrieves the constraints
	// associated to the given wires. The first method deduplicates while the second
	// does not.
	GetWireConstraints(wires []frontend.Variable, addMissing bool) ([][2]int, error)
	GetWiresConstraintExact(wires []frontend.Variable, addMissing bool) ([][2]int, error)

	// PlonkAPI allows to use plonk-specific APIs in the circuit definition
	frontend.PlonkAPI

	// AddPlonkCommitmentInputs and AddPlonkCommitmentOutputs manage commitment
	// inputs and outputs. When using commitment over large fields this is
	// automatically handled in the [Commit] method. But we implement these
	// methods here for wide commitment as gnark PLONK builder does not have
	// wide committer interfaces.
	AddPlonkCommitmentInputs(inputs []frontend.Variable) []int
	AddPlonkCommitmentOutputs(committed []int, outs []frontend.Variable) error
}

type widecommitter struct {
	PlonkInWizardBuilder
}

// From creates a new builder implementing the [frontend.WideCommitter] interface
// from a base builder implementing [PlonkInWizardBuilder].
func From(newBaseBuilder frontend.NewBuilderU32) frontend.NewBuilderU32 {
	return func(field *big.Int, config frontend.CompileConfig) (frontend.Builder[constraint.U32], error) {
		baseBuilder, err := newBaseBuilder(field, config)
		if err != nil {
			return nil, fmt.Errorf("new base builder: %w", err)
		}
		scb, ok := baseBuilder.(PlonkInWizardBuilder)
		if !ok {
			return nil, fmt.Errorf("base builder doesn't implement PlonkInWizardBuilder")
		}
		return &widecommitter{
			PlonkInWizardBuilder: scb,
		}, nil
	}
}

// WideCommit implements the [frontend.WideCommitter] interface. It allows to
// compute an efficient commitment over variables and returning an extension
// field element representing the commitment. The method errors when the
// requested width is different from the field extension degree, calling the
// commitment hint fails or when we fail to add the commitment information to
// the constraint system.
//
// The method uses [PlaceholderWideCommitHint] as a placeholder hint function,
// which must be replaced by actual implementation at solving time!
//
// The method filters out the constant from the inputs and then performs
// deduplication to ensure we only have a single constraint for every committed
// variable.
func (wc *widecommitter) WideCommit(width int, vars ...frontend.Variable) ([]frontend.Variable, error) {

	// the random coin is sampled from the extension field. In practice we could
	// support smaller width, but it would most probably lead to unsound
	// construction of subprotocols, so we prohibit it. In case we want to
	// support larger width than the extension degree, then we would have to
	// sample multiple random coins and combine them, but lets solve it when we
	// need it. Currently there doesn't seem to be a use case for it.
	//
	// @alex: this behaviour is currently patched because the gnark non-native
	// circuits use a width of 8 and we assume a width of 4. What we do is that
	// we concretely pad with zeroes to the right.
	if width < fext.ExtensionDegree {
		return nil, fmt.Errorf("wide commit: expected width %d, got %d", fext.ExtensionDegree, width)
	}

	// filter the variables which are only "canonical" (not constant).
	filtered := make([]frontend.Variable, 0, len(vars))
	for _, v := range vars {
		if _, isConstant := wc.ConstantValue(v); !isConstant {
			filtered = append(filtered, v)
		}
	}
	// secondly, deduplicate the variables using HashCode. We use a slice to ensure determinism.
	deduplicatedMap := make(map[[16]byte]frontend.Variable)
	// build the deduplicated inputs to the commitment inputs. We set the first element to zero to indicate
	// that commitment depth is zero. We currently only support single-round commitments but in principle
	// could run gnark circuits over mulitiple rounds if necessary.
	deduplicated := make([]frontend.Variable, 1, len(filtered)+1)
	deduplicated[0] = 0
	for _, v := range filtered {
		if hashable, ok := v.(interface{ HashCode() [16]byte }); ok {
			hash := hashable.HashCode()
			if _, found := deduplicatedMap[hash]; found {
				continue
			}
			deduplicated = append(deduplicated, v)
			deduplicatedMap[hash] = v

		} else {
			panic("not hashable variable")
		}
	}
	clear(deduplicatedMap)

	committed := wc.AddPlonkCommitmentInputs(deduplicated[1:])
	outs, err := wc.Compiler().NewHint(PlaceholderWideCommitHint, width, deduplicated...)
	if err != nil {
		return nil, fmt.Errorf("creating hint for wide commit: %w", err)
	}
	if err := wc.AddPlonkCommitmentOutputs(committed, outs); err != nil {
		return nil, fmt.Errorf("adding wide commit outputs: %w", err)
	}

	if len(outs) < width {
		for _ = range width - len(outs) {
			outs = append(outs, 0)
		}
	}

	return outs, nil
}

// PlaceholderWideCommitHint is a placeholder hint function for wide commitment.
// It should be replaced at solving time by the actual commitment computation.
// If not replaced, then it panics.
func PlaceholderWideCommitHint(field *big.Int, inputs []*big.Int, outputs []*big.Int) error {
	panic("calling mocked wide commitment hint, it should be replaced at solving time")
}

// Compiler returns the compiler of the underlying builder.
func (wc *widecommitter) Compiler() frontend.Compiler {
	return wc
}

func init() {
	solver.RegisterHint(PlaceholderWideCommitHint)
}
