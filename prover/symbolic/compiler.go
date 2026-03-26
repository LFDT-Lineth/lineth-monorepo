package symbolic

import (
	"unsafe"

	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

// Program is a compiled expression board ready for evaluation.

type opCode uint8

const (
	opLoadConst opCode = iota
	opLoadInput
	opMul
	opLinComb
	opPolyEval
)

// MaxChunkSize controls the chunk size for parallel evaluation.
// Must be ≥16 for avx512 and a power of 2.
var MaxChunkSize = 1 << 8 // 256

func areAllConstants(inp []smartvectors.SmartVector) bool {
	for _, v := range inp {
		// must be sv.Constant or sv.ConstantExt
		switch v.(type) {
		case *smartvectors.Constant:
		case *smartvectors.ConstantExt:
		default:
			return false
		}
	}
	return true
}

func (b *ExpressionBoard) Evaluate(inputs []smartvectors.SmartVector) smartvectors.SmartVector {
	if b.ProgramNodesCount != len(b.Nodes) {
		b.Compile()
	}

	if len(inputs) == 0 {
		panic("no input provided")
	}
	totalSize := inputs[0].Len()
	if totalSize == 0 {
		panic("inputs all have size 0")
	}
	for i := 1; i < len(inputs); i++ {
		if inputs[i].Len() != totalSize {
			utils.Panic("mismatch in the size: len(v0) %v, len(v%v) %v", totalSize, i, inputs[i].Len())
		}
	}
	// MaxChunkSize: larger chunks reduce per-chunk dispatch overhead.
	// Must be ≥16 for avx512 and a power of 2.
	//
	// Cap chunk size for boards with many slots to avoid OOM. Each goroutine
	// (GOMAXPROCS) allocates a VM of NumSlots*chunkSize elements. For large
	// boards (e.g. 214k slots) with 192 cores this can exceed 200 GB.
	maxChunkSize := MaxChunkSize
	if b.NumSlots > 0 {
		var extElem fext.Element
		elemBytes := int(unsafe.Sizeof(extElem))
		// Target at most ~256 MB per VM (ext has both memory + baseMemory ≈ 1.25× ext size)
		const maxVMBytes = 256 << 20
		maxForBoard := maxVMBytes / (b.NumSlots * elemBytes)
		// Round down to largest power of 2
		for maxForBoard > 0 && (maxForBoard&(maxForBoard-1)) != 0 {
			maxForBoard &= maxForBoard - 1
		}
		if maxForBoard >= 16 && maxForBoard < maxChunkSize {
			maxChunkSize = maxForBoard
		}
	}
	chunkSize := min(maxChunkSize, totalSize)
	// Find largest power-of-2 chunk size that divides totalSize.
	for chunkSize > 1 && totalSize%chunkSize != 0 {
		chunkSize >>= 1
	}
	numChunks := totalSize / chunkSize

	isBase := smartvectors.AreAllBase(inputs)

	if isBase {
		res := make([]field.Element, totalSize)
		parallel.Execute(numChunks, func(start, stop int) {
			vm := newVMBase(b, chunkSize)
			for chunkID := start; chunkID < stop; chunkID++ {
				chunkStart := chunkID * chunkSize
				chunkStop := (chunkID + 1) * chunkSize
				vm.execute(inputs, chunkStart, chunkStop)

				lastDst := b.ResultSlot
				copy(res[chunkStart:chunkStop], vm.memory[lastDst*chunkSize:(lastDst+1)*chunkSize])
			}
		})

		if areAllConstants(inputs) {
			return smartvectors.NewConstant(res[0], totalSize)
		}
		return smartvectors.NewRegular(res)
	} else {
		// Probe one chunk to determine if result stays in base field.
		probeVM := newVMExt(b, chunkSize)
		probeVM.execute(inputs, 0, chunkSize)
		lastDst := b.ResultSlot
		resultIsBase := probeVM.slotIsBase[lastDst]

		if resultIsBase {
			resBase := make([]field.Element, totalSize)
			parallel.Execute(numChunks, func(start, stop int) {
				vm := newVMExt(b, chunkSize)
				for chunkID := start; chunkID < stop; chunkID++ {
					chunkStart := chunkID * chunkSize
					chunkStop := (chunkID + 1) * chunkSize
					vm.execute(inputs, chunkStart, chunkStop)
					copy(resBase[chunkStart:chunkStop], vm.baseSlot(lastDst*chunkSize, chunkSize))
				}
			})
			if areAllConstants(inputs) {
				return smartvectors.NewConstantExt(fext.Lift(resBase[0]), totalSize)
			}
			lifted := make([]fext.Element, totalSize)
			for i, v := range resBase {
				lifted[i] = fext.Lift(v)
			}
			return smartvectors.NewRegularExt(lifted)
		}

		resExt := make([]fext.Element, totalSize)
		parallel.Execute(numChunks, func(start, stop int) {
			vm := newVMExt(b, chunkSize)
			for chunkID := start; chunkID < stop; chunkID++ {
				chunkStart := chunkID * chunkSize
				chunkStop := (chunkID + 1) * chunkSize
				vm.execute(inputs, chunkStart, chunkStop)
				copy(resExt[chunkStart:chunkStop], vm.memory[lastDst*chunkSize:(lastDst+1)*chunkSize])
			}
		})
		if areAllConstants(inputs) {
			return smartvectors.NewConstantExt(resExt[0], totalSize)
		}
		return smartvectors.NewRegularExt(resExt)
	}
}

// Compile compiles the expression board into a program.
func (b *ExpressionBoard) Compile() {
	if len(b.Nodes) == 0 {
		b.Bytecode = nil
		b.Constants = nil
		b.NumSlots = 0
		b.ProgramNodesCount = 0
		return
	}

	// 1. Liveness Analysis
	lastUse := make([]int, len(b.Nodes))
	for i := range lastUse {
		lastUse[i] = -1
	}
	// The last node is the output, so it is implicitly used at the end
	lastUse[len(b.Nodes)-1] = len(b.Nodes)

	for i, node := range b.Nodes {
		for _, childID := range node.Children {
			if i > lastUse[childID] {
				lastUse[childID] = i
			}
		}
	}

	// 2. Instruction Generation & Register Allocation
	bytecode := make([]int, 0, len(b.Nodes)*4)
	constants := make([]fext.Element, 0)
	slots := make([]int, len(b.Nodes))
	freeSlots := make([]int, 0)
	nextSlot := 0

	// Helper to get a slot
	getSlot := func() int {
		if len(freeSlots) > 0 {
			s := freeSlots[len(freeSlots)-1]
			freeSlots = freeSlots[:len(freeSlots)-1]
			return s
		}
		s := nextSlot
		nextSlot++
		return s
	}

	inputCursor := 0

	for i, node := range b.Nodes {
		dstSlot := getSlot()
		slots[i] = dstSlot

		switch op := node.Operator.(type) {
		case Constant:
			bytecode = append(bytecode, int(opLoadConst), dstSlot, len(constants))
			constants = append(constants, op.Val.GetExt())
		case Variable:
			bytecode = append(bytecode, int(opLoadInput), dstSlot, inputCursor)
			inputCursor++
		case Product:
			bytecode = append(bytecode, int(opMul), dstSlot, len(node.Children))
			for k, childID := range node.Children {
				bytecode = append(bytecode, slots[childID], op.Exponents[k])
			}
		case LinComb:
			bytecode = append(bytecode, int(opLinComb), dstSlot, len(node.Children))
			for k, childID := range node.Children {
				bytecode = append(bytecode, slots[childID], op.Coeffs[k])
			}
		case PolyEval:
			bytecode = append(bytecode, int(opPolyEval), dstSlot, len(node.Children))
			for _, childID := range node.Children {
				bytecode = append(bytecode, slots[childID])
			}
		default:
			utils.Panic("unknown op %T", op)
		}

		// Free slots of children that are dead
		freed := make(map[int]bool)
		for _, childID := range node.Children {
			cid := int(childID)
			if lastUse[cid] == i {
				if !freed[slots[cid]] {
					freeSlots = append(freeSlots, slots[cid])
					freed[slots[cid]] = true
				}
			}
		}
	}

	b.Bytecode = bytecode
	b.Constants = constants
	b.NumSlots = nextSlot
	b.ResultSlot = slots[len(b.Nodes)-1]
	b.ProgramNodesCount = len(b.Nodes)
}

// VM for Base elements
type vmBase struct {
	memory    []field.Element
	scratch   []field.Element
	chunkSize int
	board     *ExpressionBoard
}

func newVMBase(b *ExpressionBoard, chunkSize int) *vmBase {
	return &vmBase{
		memory:    make([]field.Element, b.NumSlots*chunkSize),
		scratch:   make([]field.Element, chunkSize),
		chunkSize: chunkSize,
		board:     b,
	}
}

func (vm *vmBase) execute(inputs []smartvectors.SmartVector, chunkStart, chunkStop int) {
	chunkLen := chunkStop - chunkStart
	bytecode := vm.board.Bytecode
	constants := vm.board.Constants
	pc := 0

	for pc < len(bytecode) {
		op := opCode(bytecode[pc])
		pc++
		dstSlot := bytecode[pc]
		pc++
		dstOffset := dstSlot * vm.chunkSize
		dst := vm.memory[dstOffset : dstOffset+chunkLen]

		switch op {
		case opLoadConst:
			constIdx := bytecode[pc]
			pc++
			val := constants[constIdx].B0.A0
			for i := 0; i < chunkLen; i++ {
				dst[i] = val
			}
		case opLoadInput:
			inputID := bytecode[pc]
			pc++
			input := inputs[inputID]
			switch rv := input.(type) {
			case *smartvectors.Regular:
				copy(dst, (*rv)[chunkStart:chunkStop])
			default:
				sb := input.SubVector(chunkStart, chunkStop)
				sb.WriteInSlice(dst)
			}
		case opMul:
			numSrc := bytecode[pc]
			pc++
			vRes := field.Vector(dst)
			vTmp := field.Vector(vm.scratch[:chunkLen])
			for k := 0; k < numSrc; k++ {
				srcSlot := bytecode[pc]
				pc++
				exp := bytecode[pc]
				pc++
				srcOffset := srcSlot * vm.chunkSize
				vInput := field.Vector(vm.memory[srcOffset : srcOffset+chunkLen])
				if k == 0 {
					if exp == 1 {
						copy(vRes, vInput)
					} else {
						vRes.Exp(vInput, int64(exp))
					}
				} else if exp == 1 {
					vRes.Mul(vRes, vInput)
				} else {
					vTmp.Exp(vInput, int64(exp))
					vRes.Mul(vRes, vTmp)
				}
			}
		case opLinComb:
			numSrc := bytecode[pc]
			pc++
			vRes := field.Vector(dst)
			vTmp := field.Vector(vm.scratch[:chunkLen])
			var t0 field.Element
			for k := 0; k < numSrc; k++ {
				srcSlot := bytecode[pc]
				pc++
				coeff := bytecode[pc]
				pc++
				srcOffset := srcSlot * vm.chunkSize
				vInput := field.Vector(vm.memory[srcOffset : srcOffset+chunkLen])

				if k == 0 {
					switch coeff {
					case 0:
						for j := range vRes {
							vRes[j].SetZero()
						}
					case 1:
						copy(vRes, vInput)
					case 2:
						vRes.Add(vInput, vInput)
					case -1:
						for j := range vRes {
							vRes[j].SetZero()
						}
						vRes.Sub(vRes, vInput)
					default:
						t0.SetInt64(int64(coeff))
						vRes.ScalarMul(vInput, &t0)
					}
					continue
				}

				switch coeff {
				case 0:
					continue
				case 1:
					vRes.Add(vRes, vInput)
				case 2:
					vRes.Add(vRes, vInput)
					vRes.Add(vRes, vInput)
				case -1:
					vRes.Sub(vRes, vInput)
				default:
					t0.SetInt64(int64(coeff))
					vTmp.ScalarMul(vInput, &t0)
					vRes.Add(vRes, vTmp)
				}
			}
		case opPolyEval:
			numSrc := bytecode[pc]
			pc++
			srcStart := pc
			pc += numSrc

			// input[0] is x (constant)
			xSlot := bytecode[srcStart]
			xOffset := xSlot * vm.chunkSize
			x := vm.memory[xOffset]

			vRes := field.Vector(dst)

			lastSrcSlot := bytecode[srcStart+numSrc-1]
			lastSrcOffset := lastSrcSlot * vm.chunkSize
			copy(vRes, field.Vector(vm.memory[lastSrcOffset:lastSrcOffset+chunkLen]))

			for k := numSrc - 2; k >= 1; k-- {
				srcSlot := bytecode[srcStart+k]
				srcOffset := srcSlot * vm.chunkSize
				vTmp := field.Vector(vm.memory[srcOffset : srcOffset+chunkLen])
				vRes.ScalarMul(vRes, &x)
				vRes.Add(vRes, vTmp)
			}
		}
	}
}

// VM for Ext elements.
// slotIsBase tracks per-slot whether all elements are base-only.
// baseMemory stores base slot data contiguously to avoid stride-4
// scatter/gather into the ext memory layout (4x field.Element per fext.Element).
type vmExt struct {
	memory      []fext.Element  // ext slot data
	baseMemory  []field.Element // base slot data (contiguous)
	scratch     []fext.Element  // ext scratch
	baseScratch []field.Element // base scratch
	slotIsBase  []bool
	chunkSize   int
	board       *ExpressionBoard
}

func newVMExt(b *ExpressionBoard, chunkSize int) *vmExt {
	return &vmExt{
		memory:      make([]fext.Element, b.NumSlots*chunkSize),
		baseMemory:  make([]field.Element, b.NumSlots*chunkSize),
		scratch:     make([]fext.Element, chunkSize),
		baseScratch: make([]field.Element, chunkSize),
		slotIsBase:  make([]bool, b.NumSlots),
		chunkSize:   chunkSize,
		board:       b,
	}
}

// baseSlot returns a field.Vector view of the base memory for the given slot offset and length.
func (vm *vmExt) baseSlot(offset, length int) field.Vector {
	return field.Vector(vm.baseMemory[offset : offset+length])
}

// scatterBaseToExt copies base slot data into ext memory (for transitions).
func (vm *vmExt) scatterBaseToExt(offset, chunkLen int) {
	bSrc := vm.baseMemory[offset : offset+chunkLen]
	eDst := vm.memory[offset : offset+chunkLen]
	for i := range bSrc {
		eDst[i].B0.A0 = bSrc[i]
		eDst[i].B0.A1.SetZero()
		eDst[i].B1.A0.SetZero()
		eDst[i].B1.A1.SetZero()
	}
}

func (vm *vmExt) execute(inputs []smartvectors.SmartVector, chunkStart, chunkStop int) {
	chunkLen := chunkStop - chunkStart
	bytecode := vm.board.Bytecode
	constants := vm.board.Constants
	pc := 0

	for pc < len(bytecode) {
		op := opCode(bytecode[pc])
		pc++
		dstSlot := bytecode[pc]
		pc++
		dstOffset := dstSlot * vm.chunkSize
		dst := vm.memory[dstOffset : dstOffset+chunkLen]
		baseDst := vm.baseSlot(dstOffset, chunkLen)

		switch op {
		case opLoadConst:
			constIdx := bytecode[pc]
			pc++
			val := constants[constIdx]
			isBase := fext.IsBase(&val)
			vm.slotIsBase[dstSlot] = isBase
			if isBase {
				for i := range baseDst {
					baseDst[i] = val.B0.A0
				}
			} else {
				for i := 0; i < chunkLen; i++ {
					dst[i] = val
				}
			}

		case opLoadInput:
			inputID := bytecode[pc]
			pc++
			input := inputs[inputID]
			switch rv := input.(type) {
			case *smartvectors.RotatedExt:
				vm.slotIsBase[dstSlot] = false
				rv.WriteSubVectorInSliceExt(chunkStart, chunkStop, dst)
			case *smartvectors.Rotated:
				vm.slotIsBase[dstSlot] = true
				rv.WriteSubVectorInSlice(chunkStart, chunkStop, baseDst)
			case *smartvectors.Regular:
				vm.slotIsBase[dstSlot] = true
				copy(baseDst, (*rv)[chunkStart:chunkStop])
			case *smartvectors.RegularExt:
				vm.slotIsBase[dstSlot] = false
				copy(dst, (*rv)[chunkStart:chunkStop])
			case *smartvectors.Constant:
				vm.slotIsBase[dstSlot] = true
				val := rv.Value
				for i := range baseDst {
					baseDst[i] = val
				}
			case *smartvectors.ConstantExt:
				isBase := fext.IsBase(&rv.Value)
				vm.slotIsBase[dstSlot] = isBase
				if isBase {
					for i := range baseDst {
						baseDst[i] = rv.Value.B0.A0
					}
				} else {
					for i := 0; i < chunkLen; i++ {
						dst[i].Set(&rv.Value)
					}
				}
			default:
				isBase := smartvectors.IsBase(input)
				vm.slotIsBase[dstSlot] = isBase
				sb := input.SubVector(chunkStart, chunkStop)
				if isBase {
					sb.WriteInSlice(baseDst)
				} else {
					sb.WriteInSliceExt(dst)
				}
			}

		case opMul:
			numSrc := bytecode[pc]
			pc++

			// Fast path: 2-operand base×base with exp=1 (most common case).
			// Writes Mul(src1, src2) → dst directly, skipping the copy+Mul pattern.
			if numSrc == 2 {
				s0, e0 := bytecode[pc], bytecode[pc+1]
				s1, e1 := bytecode[pc+2], bytecode[pc+3]
				if vm.slotIsBase[s0] && vm.slotIsBase[s1] && e0 == 1 && e1 == 1 {
					pc += 4
					b0 := vm.baseSlot(s0*vm.chunkSize, chunkLen)
					b1 := vm.baseSlot(s1*vm.chunkSize, chunkLen)
					baseDst.Mul(b0, b1)
					vm.slotIsBase[dstSlot] = true
					continue
				}
			}

			vRes := extensions.Vector(dst)
			vTmp := extensions.Vector(vm.scratch[:chunkLen])
			resIsBase := false

			for k := 0; k < numSrc; k++ {
				srcSlot := bytecode[pc]
				pc++
				exp := bytecode[pc]
				pc++
				srcOffset := srcSlot * vm.chunkSize
				srcIsBase := vm.slotIsBase[srcSlot]

				if k == 0 {
					resIsBase = srcIsBase
					if srcIsBase {
						bSrc := vm.baseSlot(srcOffset, chunkLen)
						if exp == 1 {
							copy(baseDst, bSrc)
						} else {
							baseDst.Exp(bSrc, int64(exp))
						}
					} else {
						vInput := extensions.Vector(vm.memory[srcOffset : srcOffset+chunkLen])
						if exp == 1 {
							copy(vRes, vInput)
						} else {
							vRes.Exp(vInput, int64(exp))
						}
					}
					continue
				}

				if srcIsBase {
					bSrc := vm.baseSlot(srcOffset, chunkLen)
					operandBase := bSrc
					if exp != 1 {
						bTmp := field.Vector(vm.baseScratch[:chunkLen])
						bTmp.Exp(bSrc, int64(exp))
						operandBase = bTmp
					}
					if resIsBase {
						// base * base → base (contiguous field.Vector.Mul)
						baseDst.Mul(baseDst, operandBase)
					} else {
						// ext * base → ext (vectorized MulByElement)
						vRes.MulByElement(vRes, operandBase)
					}
				} else {
					vInput := extensions.Vector(vm.memory[srcOffset : srcOffset+chunkLen])
					operand := vInput
					if exp != 1 {
						vTmp.Exp(vInput, int64(exp))
						operand = vTmp
					}
					if resIsBase {
						// base * ext → ext (transition)
						vRes.MulByElement(operand, baseDst)
						resIsBase = false
					} else {
						// ext * ext → full mul
						vRes.Mul(vRes, operand)
					}
				}
			}
			vm.slotIsBase[dstSlot] = resIsBase

		case opLinComb:
			numSrc := bytecode[pc]
			pc++

			// Fast path: 2-operand base+base with simple coefficients.
			// Avoids copy+Add/Sub by using Add(src1,src2) or Sub(src1,src2) directly.
			if numSrc == 2 {
				s0, c0 := bytecode[pc], bytecode[pc+1]
				s1, c1 := bytecode[pc+2], bytecode[pc+3]
				if vm.slotIsBase[s0] && vm.slotIsBase[s1] && c0 == 1 {
					b0 := vm.baseSlot(s0*vm.chunkSize, chunkLen)
					b1 := vm.baseSlot(s1*vm.chunkSize, chunkLen)
					switch c1 {
					case 1:
						pc += 4
						baseDst.Add(b0, b1)
						vm.slotIsBase[dstSlot] = true
						continue
					case -1:
						pc += 4
						baseDst.Sub(b0, b1)
						vm.slotIsBase[dstSlot] = true
						continue
					}
				}
			}

			vRes := extensions.Vector(dst)
			vTmp := extensions.Vector(vm.scratch[:chunkLen])
			var t0 field.Element
			resIsBase := false

			for k := 0; k < numSrc; k++ {
				srcSlot := bytecode[pc]
				pc++
				coeff := bytecode[pc]
				pc++
				srcOffset := srcSlot * vm.chunkSize
				srcIsBase := vm.slotIsBase[srcSlot]

				if k == 0 {
					if srcIsBase {
						bSrc := vm.baseSlot(srcOffset, chunkLen)
						resIsBase = true
						switch coeff {
						case 0:
							for j := range baseDst {
								baseDst[j].SetZero()
							}
						case 1:
							copy(baseDst, bSrc)
						case 2:
							baseDst.Add(bSrc, bSrc)
						case -1:
							for j := range baseDst {
								baseDst[j].SetZero()
							}
							baseDst.Sub(baseDst, bSrc)
						default:
							t0.SetInt64(int64(coeff))
							baseDst.ScalarMul(bSrc, &t0)
						}
					} else {
						vInput := extensions.Vector(vm.memory[srcOffset : srcOffset+chunkLen])
						resIsBase = coeff == 0
						switch coeff {
						case 0:
							for j := range baseDst {
								baseDst[j].SetZero()
							}
						case 1:
							copy(vRes, vInput)
						case 2:
							vRes.Add(vInput, vInput)
						case -1:
							for j := range vRes {
								vRes[j].SetZero()
							}
							vRes.Sub(vRes, vInput)
						default:
							t0.SetInt64(int64(coeff))
							vRes.ScalarMulByElement(vInput, &t0)
						}
					}
					continue
				}

				if coeff == 0 {
					continue
				}

				if srcIsBase {
					bSrc := vm.baseSlot(srcOffset, chunkLen)
					if resIsBase {
						// base + base → base (contiguous)
						switch coeff {
						case 1:
							baseDst.Add(baseDst, bSrc)
						case 2:
							baseDst.Add(baseDst, bSrc)
							baseDst.Add(baseDst, bSrc)
						case -1:
							baseDst.Sub(baseDst, bSrc)
						default:
							t0.SetInt64(int64(coeff))
							bTmp := field.Vector(vm.baseScratch[:chunkLen])
							bTmp.ScalarMul(bSrc, &t0)
							baseDst.Add(baseDst, bTmp)
						}
					} else {
						// ext + base: add base to B0.A0 of ext result
						switch coeff {
						case 1:
							for i := range vRes {
								vRes[i].B0.A0.Add(&vRes[i].B0.A0, &bSrc[i])
							}
						case 2:
							for i := range vRes {
								vRes[i].B0.A0.Add(&vRes[i].B0.A0, &bSrc[i])
								vRes[i].B0.A0.Add(&vRes[i].B0.A0, &bSrc[i])
							}
						case -1:
							for i := range vRes {
								vRes[i].B0.A0.Sub(&vRes[i].B0.A0, &bSrc[i])
							}
						default:
							t0.SetInt64(int64(coeff))
							for i := range vRes {
								var tmp field.Element
								tmp.Mul(&bSrc[i], &t0)
								vRes[i].B0.A0.Add(&vRes[i].B0.A0, &tmp)
							}
						}
					}
					continue
				}

				// Extension input
				vInput := extensions.Vector(vm.memory[srcOffset : srcOffset+chunkLen])
				if resIsBase {
					// Transition: scatter base result to ext memory
					vm.scatterBaseToExt(dstOffset, chunkLen)
					resIsBase = false
				}
				switch coeff {
				case 1:
					vRes.Add(vRes, vInput)
				case 2:
					vRes.Add(vRes, vInput)
					vRes.Add(vRes, vInput)
				case -1:
					vRes.Sub(vRes, vInput)
				default:
					t0.SetInt64(int64(coeff))
					vTmp.ScalarMulByElement(vInput, &t0)
					vRes.Add(vRes, vTmp)
				}
			}
			vm.slotIsBase[dstSlot] = resIsBase

		case opPolyEval:
			numSrc := bytecode[pc]
			pc++
			srcStart := pc
			pc += numSrc

			xSlot := bytecode[srcStart]
			xOffset := xSlot * vm.chunkSize
			xIsBase := vm.slotIsBase[xSlot]

			vRes := extensions.Vector(dst)

			// Check if all coefficients are base and x is ext.
			// In that case, use power-sum with MulAccByElement instead of Horner.
			// This replaces ext×ext ScalarMul with base×ext MulAcc (~4× fewer muls).
			allCoeffsBase := !xIsBase
			if allCoeffsBase {
				for k := 1; k < numSrc; k++ {
					if !vm.slotIsBase[bytecode[srcStart+k]] {
						allCoeffsBase = false
						break
					}
				}
			}

			if allCoeffsBase && numSrc > 2 {
				// Power-sum: result = c₁ + c₂·x + c₃·x² + ... + cₙ·x^(n-1)
				x := vm.memory[xOffset]
				numCoeffs := numSrc - 1

				// Precompute x powers: [1, x, x², ..., x^(n-2)]
				powers := make([]fext.Element, numCoeffs)
				powers[0].SetOne()
				for i := 1; i < numCoeffs; i++ {
					powers[i].Mul(&powers[i-1], &x)
				}

				// Initialize vRes from c₁ (power x⁰ = 1, just scatter base to ext)
				firstSlot := bytecode[srcStart+1]
				firstOffset := firstSlot * vm.chunkSize
				bFirst := vm.baseSlot(firstOffset, chunkLen)
				for i := range vRes {
					vRes[i].B0.A0 = bFirst[i]
					vRes[i].B0.A1.SetZero()
					vRes[i].B1.A0.SetZero()
					vRes[i].B1.A1.SetZero()
				}

				// Accumulate: vRes[i] += cₖ[i] * x^(k-1)
				for k := 2; k < numSrc; k++ {
					srcSlot := bytecode[srcStart+k]
					srcOffset := srcSlot * vm.chunkSize
					bSrc := vm.baseSlot(srcOffset, chunkLen)
					vRes.MulAccByElement(bSrc, &powers[k-1])
				}

				vm.slotIsBase[dstSlot] = false
			} else {
				// Horner with base-tracking: use cheaper multiplies when accumulator is base
				lastSrcSlot := bytecode[srcStart+numSrc-1]
				lastSrcOffset := lastSrcSlot * vm.chunkSize
				resIsBase := vm.slotIsBase[lastSrcSlot]
				if resIsBase {
					bSrc := vm.baseSlot(lastSrcOffset, chunkLen)
					copy(baseDst, bSrc)
				} else {
					copy(vRes, extensions.Vector(vm.memory[lastSrcOffset:lastSrcOffset+chunkLen]))
				}

				for k := numSrc - 2; k >= 1; k-- {
					srcSlot := bytecode[srcStart+k]
					srcOffset := srcSlot * vm.chunkSize

					// Multiply step: vRes *= x
					if resIsBase {
						if xIsBase {
							// base × base → base (1 base mul per element)
							xBase := vm.baseMemory[xOffset]
							baseDst.ScalarMul(baseDst, &xBase)
						} else {
							// base × ext → ext (MulByElement: 4 base muls vs 9 for full ext×ext)
							x := vm.memory[xOffset]
							for i := range vRes {
								vRes[i].MulByElement(&x, &baseDst[i])
							}
							resIsBase = false
						}
					} else {
						if xIsBase {
							xBase := vm.baseMemory[xOffset]
							vRes.ScalarMulByElement(vRes, &xBase)
						} else {
							x := vm.memory[xOffset]
							vRes.ScalarMul(vRes, &x)
						}
					}

					// Add step: vRes += coefficient[k]
					if vm.slotIsBase[srcSlot] {
						bSrc := vm.baseSlot(srcOffset, chunkLen)
						if resIsBase {
							baseDst.Add(baseDst, bSrc)
						} else {
							for i := range vRes {
								vRes[i].B0.A0.Add(&vRes[i].B0.A0, &bSrc[i])
							}
						}
					} else {
						if resIsBase {
							// Transition: scatter base to ext before adding ext coefficient
							vm.scatterBaseToExt(dstOffset, chunkLen)
							resIsBase = false
						}
						vTmp := extensions.Vector(vm.memory[srcOffset : srcOffset+chunkLen])
						vRes.Add(vRes, vTmp)
					}
				}

				vm.slotIsBase[dstSlot] = resIsBase
			}
		}
	}
}
