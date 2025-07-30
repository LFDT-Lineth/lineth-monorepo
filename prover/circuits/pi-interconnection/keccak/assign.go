package keccak

import (
	"bytes"
	"errors"
	"hash"
	"math/big"
	"slices"

	"github.com/consensys/linea-monorepo/prover/utils"

	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/compress"
	"github.com/consensys/linea-monorepo/prover/circuits/internal"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/sha3"
)

// The strict hasher abstraction bridges the keccak computation in the circuit assignment and proving phases.
// The basic problem is that they need to do the same hashes in the same order, and that compiling and assigning the
// wizard sub-verifier must be done seamlessly.
// TODO eliminate the order equality requirement using set equality arguments
// The strict hasher compiler collects the expected number and length of hashes to be done.
// It produces the compiled strict hasher, which contains the compiled verifier sub-sub-circuit (and its assigner "module")
// It can be costly to create and hence must be stored on disk alongside the gnark circuit it helps create.
// The compiled strict hasher can on the fly create the gnark sub-circuit used for circuit compilation, and an assigner,
// adhering to the Go hash.Hash interface.
// The assigner creates the wizard proof and the assigned gnark sub-circuit.
// Finally, the gnark sub-circuit can produce a SNARK hasher to be used inside the circuit.Define function.

// TODO Perhaps a permutation argument would help usability
// i.e. compute ∏ (r+ inLen + in₀ s + in₁ s² + ... + in_{maxInLen-1} sᵐᵃˣᴵⁿᴸᵉⁿ + out_0 sᵐᵃˣᴵⁿᴸᵉⁿ⁺¹ + ... + out₃₁ sᵐᵃˣᴵⁿᴸᵉⁿ⁺³²
// on both sides and assert their equality
// (can pack the in-outs first to reduce constraints slightly)

type StrictHasherCompiler []int

// CompiledStrictHasher must be stored and reloaded
type CompiledStrictHasher struct {
	wc           *HashWizardVerifierSubCircuit
	lengths      []int // expected lengths of hashes; Strict lengths are represented as negative numbers, and the absolute value is the length. Flexible lengths are represented as positive numbers.
	maxNbKeccakF int
}

// StrictHasherCircuit is to be embedded in a gnark circuit
type StrictHasherCircuit struct {
	Wc           *wizard.VerifierCircuit
	Ins          [][][2]frontend.Variable // every 32-byte block is prepacked into 2 16-byte blocks
	InLengths    []frontend.Variable      // actual lengths of the inputs
	maxNbKeccakF int
}

// StrictHasher implements hash.Hash
type StrictHasher struct {
	expectedLengths []int
	nbSumsYet       int
	ins             [][]byte
	wc              *HashWizardVerifierSubCircuit
	maxNbKeccakF    int
	buffer          bytes.Buffer
	h               hash.Hash
}

type StrictHasherSnark struct {
	ins       [][][2]frontend.Variable
	h         Hasher
	c         *StrictHasherCircuit
	inLenghts []frontend.Variable
}

// NewStrictHasherCompiler creates a new strict hasher compiler with capacity for a number of hashes equal to the sum of the arguments
func NewStrictHasherCompiler(lengthsOfLengths ...int) StrictHasherCompiler {
	return make([]int, 0, internal.Sum(lengthsOfLengths...))
}

func (h *StrictHasherCompiler) WithFlexibleHashLengths(l ...int) *StrictHasherCompiler {
	for _, li := range l {
		if li%32 != 0 {
			panic("length must divide 32")
		}
	}
	*h = append(*h, l...)
	return h
}

func (h *StrictHasherCompiler) WithStrictHashLengths(l ...int) *StrictHasherCompiler {
	for _, li := range l {
		if li%32 != 0 {
			panic("length must divide 32")
		}
	}
	for _, li := range l {
		*h = append(*h, -li)
	}
	return h
}

// For testing purposes, wizard compilation is skipped if no options are given.
func (h *StrictHasherCompiler) Compile(wizardCompilationOpts ...func(iop *wizard.CompiledIOP)) CompiledStrictHasher {
	nbKeccakF := 0 // Since the output size is smaller than the block size the squeezing phase is trivial TODO @Tabaie check with @azam.soleimanian that this is correct

	const blockNbBytesIn = lanesPerBlock * 8
	for _, l := range *h {
		nbKeccakF += utils.Abs(l)/blockNbBytesIn + 1 // extra room for padding
	}

	logrus.Infof("Public-input interconnection requires %v keccak permutations", nbKeccakF)

	var wc *HashWizardVerifierSubCircuit
	if len(wizardCompilationOpts) != 0 {
		wc = NewWizardVerifierSubCircuit(nbKeccakF, wizardCompilationOpts...)
	}

	return CompiledStrictHasher{
		wc:           wc,
		lengths:      *h,
		maxNbKeccakF: nbKeccakF,
	}
}

func allocateIns(lengths []int) [][][2]frontend.Variable {
	ins := make([][][2]frontend.Variable, len(lengths))
	for i := range ins {
		ins[i] = make([][2]frontend.Variable, utils.Abs(lengths[i])/32)
	}
	return ins
}

func (h *CompiledStrictHasher) GetCircuit() (c StrictHasherCircuit, err error) {
	c.Ins = allocateIns(h.lengths)
	c.InLengths = make([]frontend.Variable, len(h.lengths))
	c.maxNbKeccakF = h.maxNbKeccakF
	if h.wc == nil {
		logrus.Warn("WIZARD SUB-PROVER NOT COMPILED. KECCAK HASH RESULTS WILL NOT BE VERIFIED. THIS SHOULD ONLY OCCUR IN A UNIT TEST.")
	} else {
		c.Wc, err = h.wc.Compile()
	}
	return
}

func (h *CompiledStrictHasher) GetHasher() StrictHasher {
	return StrictHasher{
		expectedLengths: h.lengths,
		ins:             make([][]byte, 0, len(h.lengths)),
		wc:              h.wc,
		maxNbKeccakF:    h.maxNbKeccakF,
		buffer:          bytes.Buffer{},
		h:               sha3.NewLegacyKeccak256(),
	}
}

func (h *CompiledStrictHasher) MaxNbKeccakF() int {
	return h.maxNbKeccakF
}

func (h *StrictHasher) Assign() (c StrictHasherCircuit, err error) {
	if h.nbSumsYet < len(h.expectedLengths) {
		return c, errors.New("fewer hashes than expected")
	}
	c.maxNbKeccakF = h.maxNbKeccakF
	if h.wc == nil {
		logrus.Warn("WIZARD SUB-PROOF NOT GENERATED. KECCAK HASH RESULTS WILL NOT BE CHECKED. THIS SHOULD ONLY OCCUR IN A UNIT TEST.")
	} else {
		c.Wc = h.wc.Assign(h.ins)
	}
	c.Ins = make([][][2]frontend.Variable, len(h.ins))
	c.InLengths = make([]frontend.Variable, len(h.ins))
	for i, in := range h.ins {
		c.Ins[i] = make([][2]frontend.Variable, utils.Abs(h.expectedLengths[i])/32) // already checked that the lengths are multiples of 32
		for j := range c.Ins[i] {
			if j < len(in)/32 {
				c.Ins[i][j][0] = in[j*32 : j*32+16]
				c.Ins[i][j][1] = in[j*32+16 : j*32+32]
			} else {
				c.Ins[i][j] = [2]frontend.Variable{0, 0}
			}
		}
		c.InLengths[i] = len(in) / 32
	}
	return
}

func (h *StrictHasherCircuit) NewHasher(api frontend.API) StrictHasherSnark {
	return StrictHasherSnark{
		c: h,
		h: Hasher{
			api:     api,
			nbLanes: lanesPerBlock * h.maxNbKeccakF,
		},
		ins:       h.Ins,
		inLenghts: utils.ToVariableSlice(h.InLengths),
	}
}

func (h *StrictHasher) Write(p []byte) (n int, err error) {
	return h.buffer.Write(p)
}

func (h *StrictHasher) NbHashes() int {
	return len(h.ins)
}

func (h *StrictHasher) MaxNbKeccakF() int {
	return h.maxNbKeccakF
}

// sumLenAllowed is len an acceptable length for the hash at offset + current position?
func (h *StrictHasher) sumLenAllowed(len, offset int) bool {
	allowed := h.expectedLengths[h.nbSumsYet+offset]
	return len <= allowed || len == -allowed
}

func (h *StrictHasher) Sum(b []byte) []byte {
	if b != nil {
		panic("not supported")
	}
	actualLen := h.buffer.Len()
	if h.nbSumsYet >= len(h.expectedLengths) {
		panic("more hashes than expected")
	}
	if !h.sumLenAllowed(actualLen, 0) {
		panic("hash length mismatch")
	}

	h.h.Reset()
	h.h.Write(h.buffer.Bytes())
	// h.buffer.Write(make([]byte, utils.Abs(h.expectedLengths[h.nbSumsYet])-actualLen)) // pad with zeros for input matching
	h.ins = append(h.ins, slices.Clone(h.buffer.Bytes()))
	//h.buffer.Truncate(actualLen)

	h.nbSumsYet++
	return h.h.Sum(nil)
}

func (h *StrictHasher) Reset() {
	h.buffer.Reset()
}

func (h *StrictHasher) Size() int {
	return h.h.Size()
}

func (h *StrictHasher) BlockSize() int {
	return h.h.BlockSize()
}

// Skip records the given input without actually computing a hash
func (h *StrictHasher) Skip(b ...[]byte) {
	if len(b) > len(h.expectedLengths)-h.nbSumsYet {
		panic("more hashes than expected")
	}
	for i := range b {
		if !h.sumLenAllowed(len(b[i]), i) {
			panic("hash length mismatch")
		}
	}
	h.ins = append(h.ins, b...)
	h.nbSumsYet += len(b)
}

func (h *StrictHasher) SkipN(n int) {
	maxSize := 0
	toSkip := h.expectedLengths[h.nbSumsYet : h.nbSumsYet+n]
	for i := range toSkip {
		maxSize = max(maxSize, toSkip[i])
	}
	b := make([]byte, maxSize)
	for _, l := range toSkip {
		h.ins = append(h.ins, b[:l])
	}
	h.nbSumsYet += n
}

func (s *StrictHasherSnark) Sum(nbIn frontend.Variable, bytess ...[32]frontend.Variable) [32]frontend.Variable {
	api := s.h.api

	if len(s.ins) == 0 {
		panic("more snark hashes than expected")
	}

	// check matching expected input
	radix := big.NewInt(256)
	expectedBytess := s.ins[0]
	if len(bytess) != len(expectedBytess) {
		panic("unexpected hash size")
	}
	var inRange *internal.Range
	if nbIn != nil {
		api.AssertIsEqual(s.inLenghts[0], nbIn)
		inRange = internal.NewRange(api, nbIn, len(bytess))
	}
	for i := range bytess {
		left, right := compress.ReadNum(api, bytess[i][:16], radix), compress.ReadNum(api, bytess[i][16:], radix)

		if nbIn == nil {
			api.AssertIsEqual(expectedBytess[i][0], left)
			api.AssertIsEqual(expectedBytess[i][1], right)
		} else {
			inRange.AssertEqualI(i, expectedBytess[i][0], left) // no need to check if past the end of the input
			inRange.AssertEqualI(i, expectedBytess[i][1], right)
		}
	}
	s.ins, s.inLenghts = s.ins[1:], s.inLenghts[1:]

	// create lanes for wizard proof
	return s.h.Sum(nbIn, bytess...)
}

func (s *StrictHasherSnark) Finalize() error {
	if len(s.ins) != 0 {
		return errors.New("fewer snark hashes than assignment hashes")
	}
	return s.h.Finalize(s.c.Wc)
}

func RegisterHints() {
	solver.RegisterHint(keccakHint, divByLanesPerBlockHint)
}
