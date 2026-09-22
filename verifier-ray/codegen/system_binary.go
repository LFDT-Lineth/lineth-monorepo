package codegen

import (
	"encoding/binary"
	"fmt"
	"io"
)

const compiledSystemBinaryMagic = "VRS1"

type binaryEncoder struct {
	w   io.Writer
	err error
}

func (e *binaryEncoder) bytes(value []byte) {
	if e.err != nil {
		return
	}
	_, e.err = e.w.Write(value)
}

func (e *binaryEncoder) u(value uint64) {
	var buf [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(buf[:], value)
	e.bytes(buf[:n])
}

func (e *binaryEncoder) i(value int64) {
	zigzag := uint64(value<<1) ^ uint64(value>>63)
	e.u(zigzag)
}

func (e *binaryEncoder) b(value bool) {
	if value {
		e.bytes([]byte{1})
	} else {
		e.bytes([]byte{0})
	}
}

func (e *binaryEncoder) ints(values []int) {
	e.u(uint64(len(values)))
	for _, value := range values {
		e.u(uint64(value))
	}
}

func (e *binaryEncoder) signedInts(values []int) {
	e.u(uint64(len(values)))
	for _, value := range values {
		e.i(int64(value))
	}
}

func (e *binaryEncoder) scalarRef(ref ScalarCellRef) {
	e.u(uint64(ref.Round))
	e.u(uint64(ref.Index))
}

func (e *binaryEncoder) moduleSize(size ModuleSize) {
	if size.Dynamic {
		e.u(1) // rowlimit.ModuleSize.dynamic / vanishing.ModuleSize.dynamic
		e.u(uint64(size.DynamicIndex))
	} else {
		e.u(0)
		e.u(uint64(size.StaticSize))
	}
}

func (e *binaryEncoder) routing(r CoinRouting) {
	e.ints(r.RoundCoinCounts)
	e.ints(r.RoundCoinOffsets)
	e.u(uint64(r.TotalRoundCoins))
	e.u(uint64(r.DynamicModuleCount))
	e.u(1 << 22) // protocol.Spec.column_size_max_supported
	if r.SharedRandomnessCoinRound >= 0 {
		e.b(true)
		e.u(uint64(r.SharedRandomnessCoinRound))
	} else {
		e.b(false)
	}
	e.u(uint64(len(r.SharedRandomnessGammaRefs)))
	for _, ref := range r.SharedRandomnessGammaRefs {
		e.u(uint64(ref.Round))
		e.u(uint64(ref.Index))
	}
}

func (e *binaryEncoder) publicInput(s PublicInputSystem) {
	e.ints(s.RoundCellCounts)
	e.u(uint64(len(s.Refs)))
	for _, ref := range s.Refs {
		e.u(uint64(ref.StatementIndex))
		e.u(uint64(ref.Round))
		e.u(uint64(ref.Index))
	}
}

func operatorTag(op Operator) (uint64, error) {
	switch op {
	case OperatorAdd:
		return 0, nil
	case OperatorMul:
		return 1, nil
	case OperatorSub:
		return 2, nil
	case OperatorDiv:
		return 3, nil
	case OperatorDouble:
		return 4, nil
	case OperatorSquare:
		return 5, nil
	case OperatorNegate:
		return 6, nil
	case OperatorInverse:
		return 7, nil
	default:
		return 0, fmt.Errorf("unknown vanishing operator %q", op)
	}
}

func (e *binaryEncoder) vanishing(s VanishingSystem) {
	e.u(uint64(len(s.Modules)))
	for _, module := range s.Modules {
		e.moduleSize(module.Size)
		e.u(uint64(len(module.Expressions)))
		for _, expr := range module.Expressions {
			e.u(uint64(expr.Kind))
			switch expr.Kind {
			case ExprColumnClaim:
				e.u(uint64(expr.ColumnClaim))
			case ExprCellValue:
				e.u(uint64(expr.Cell.Round))
				e.u(uint64(expr.Cell.Index))
			case ExprCoinValue:
				e.u(uint64(expr.Coin.FlatIndex))
			case ExprConstant:
				e.u(expr.Constant.Uint64())
			case ExprOp:
				tag, err := operatorTag(expr.Operator)
				if err != nil {
					e.err = err
					return
				}
				e.u(tag)
				// Operands are two fixed fields, not a length-prefixed slice:
				// every operator emitted here is unary or binary, and the
				// verifier reads `rhs` only for the binary ones. See
				// verifier-ray/src/query/vanishing.zig's ExprOp for why the
				// slice was removed. Guard the invariant rather than silently
				// truncating if a wider operator is ever added upstream.
				if len(expr.Operands) == 0 || len(expr.Operands) > 2 {
					e.err = fmt.Errorf("expression operator %q has %d operands; only 1 or 2 are supported", expr.Operator, len(expr.Operands))
					return
				}
				e.u(uint64(expr.Operands[0]))
				if len(expr.Operands) == 2 {
					e.u(uint64(expr.Operands[1]))
				} else {
					e.u(0)
				}
			case ExprLagrangeSelector:
				e.i(int64(expr.SelectorPosition))
			default:
				e.err = fmt.Errorf("unknown expression kind %d", expr.Kind)
				return
			}
		}
		e.u(uint64(len(module.Buckets)))
		for _, bucket := range module.Buckets {
			e.u(uint64(bucket.Ratio))
			e.u(uint64(len(bucket.Vanishings)))
			for _, vanishing := range bucket.Vanishings {
				e.u(uint64(vanishing.Expression))
				e.signedInts(vanishing.CancelledPositions)
			}
			e.u(uint64(bucket.QuotientClaimOffset))
		}
		e.u(uint64(module.WitnessClaimOffset))
		e.u(uint64(module.MergeCoinIndex))
		e.u(uint64(module.EvalCoinIndex))
	}
	e.u(uint64(s.DynamicModuleCount))
	e.u(uint64(s.TotalWitnessClaims))
	e.u(uint64(s.TotalQuotientClaims))
}

func (e *binaryEncoder) logDeriv(s LogDerivSystem) {
	e.u(uint64(len(s.Queries)))
	for _, query := range s.Queries {
		e.u(uint64(len(query.ZFinalRefs)))
		for _, ref := range query.ZFinalRefs {
			e.scalarRef(ref)
		}
		e.scalarRef(query.ResultRef)
		e.b(query.ResultIsZero)
	}
}

func (e *binaryEncoder) grandProduct(s GrandProductSystem) {
	e.u(uint64(len(s.Queries)))
	for _, query := range s.Queries {
		e.u(uint64(len(query.ZFinalRefs)))
		for _, ref := range query.ZFinalRefs {
			e.scalarRef(ref)
		}
		e.scalarRef(query.ResultRef)
		e.b(query.HasExpected)
		if query.HasExpected {
			e.u(query.Expected)
		}
	}
}

func (e *binaryEncoder) rowLimit(s RowLimitSystem) {
	e.u(uint64(len(s.Checks)))
	for _, check := range s.Checks {
		e.u(uint64(len(check.IncludedModules)))
		for _, size := range check.IncludedModules {
			e.moduleSize(size)
		}
		e.u(uint64(len(check.IncludingsModules)))
		for _, size := range check.IncludingsModules {
			e.moduleSize(size)
		}
		e.u(check.Limit)
	}
}

func (e *binaryEncoder) sharedRandomness(s SharedRandomnessSystem) {
	e.u(uint64(len(s.Rounds)))
	for _, round := range s.Rounds {
		e.u(uint64(round.RoundIndex))
		e.b(round.HasCommitment)
	}
	e.u(uint64(len(s.ContributionRefs)))
	for _, ref := range s.ContributionRefs {
		e.scalarRef(ref)
	}
}

func (e *binaryEncoder) pcs(s PcsSystem) {
	// fri.Params
	e.u(uint64(s.LogCodewordSize))
	e.u(uint64(s.LogPlaintextSize))
	e.u(uint64(s.LogFinalPolySize))
	e.u(uint64(s.NumQueries))

	e.u(uint64(len(s.Columns)))
	for _, col := range s.Columns {
		e.u(uint64(col.BatchIdx))
		e.b(col.IsExt)
		if col.IsDynamic {
			e.u(1) // pcs.SizeSource.dynamic
			e.u(uint64(col.DynamicIndex))
			e.u(uint64(col.DynamicMinSizeLog2))
		} else {
			e.u(0) // pcs.SizeSource.static
			e.u(uint64(col.SizeLog2))
		}
		e.signedInts(col.Shifts)
		e.u(uint64(len(col.ClaimCells)))
		for _, ref := range col.ClaimCells {
			e.u(uint64(ref.Round))
			e.u(uint64(ref.Index))
		}
	}
	e.u(uint64(s.NumBatches))
	e.u(uint64(len(s.BatchRoots)))
	for _, root := range s.BatchRoots {
		if root.Precomputed {
			e.u(1) // pcs.BatchRoot.precomputed
			for _, limb := range root.Root {
				e.u(limb.Uint64())
			}
		} else {
			e.u(0) // pcs.BatchRoot.round
			e.u(uint64(root.RoundIndex))
		}
	}
	e.u(uint64(len(s.WitnessMap)))
	for _, ref := range s.WitnessMap {
		e.u(uint64(ref.ColDeclIdx))
		e.u(uint64(ref.Shift))
	}
	e.u(uint64(len(s.QuotientMap)))
	for _, ref := range s.QuotientMap {
		e.u(uint64(ref.ColDeclIdx))
		e.u(uint64(ref.Shift))
	}
	e.b(true) // zeta_coin_index is mandatory for a full CompiledSystem
	e.u(uint64(s.ZetaCoinIndex))
	e.u(uint64(s.MaxEntries))
	e.u(uint64(s.MaxSizeLog2))
}

// WriteCompiledSystemBinary writes the exact generic Zig Bundle schema used by
// system_runtime.decodeBundle. Strings and codegen-only source names are not
// part of the verifier metadata and are deliberately omitted.
func WriteCompiledSystemBinary(w io.Writer, system CompiledSystem) error {
	if system.Pcs == nil {
		return fmt.Errorf("codegen: WriteCompiledSystemBinary: PCS system is required")
	}
	e := binaryEncoder{w: w}
	e.bytes([]byte(compiledSystemBinaryMagic))
	e.routing(system.Routing)
	e.publicInput(system.PublicInput)
	e.vanishing(system.Vanishing)
	e.logDeriv(system.LogDeriv)
	e.grandProduct(system.GrandProduct)
	e.rowLimit(system.RowLimit)
	e.sharedRandomness(system.SharedRandomness)
	e.pcs(*system.Pcs)
	return e.err
}
