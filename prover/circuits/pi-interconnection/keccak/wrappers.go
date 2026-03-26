package keccak

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/circuits/pi-interconnection/keccak"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

type (
	CompilationParams    []func(iop *wizard.CompiledIOP)
	BlockHasher          keccak.BlockHasher
	Hasher               struct{ *keccak.Hasher }
	StrictHasherCircuit  struct{ *keccak.StrictHasherCircuit }
	StrictHasherCompiler struct{ *keccak.StrictHasherCompiler }
	StrictHasherSnark    struct{ *keccak.StrictHasherSnark }
	CompiledStrictHasher struct{ *keccak.CompiledStrictHasher }
	StrictHasher         struct{ *keccak.StrictHasher }
)

func WizardCompilationParameters() CompilationParams {
	return keccak.WizardCompilationParameters()
}

func NewStrictHasherCompiler(lengthsOfLengths ...int) StrictHasherCompiler {
	c := keccak.NewStrictHasherCompiler(lengthsOfLengths...)
	return StrictHasherCompiler{&c}
}

func (c StrictHasherCircuit) NewHasher(api frontend.API) StrictHasherSnark {
	h := c.StrictHasherCircuit.NewHasher(api)
	return StrictHasherSnark{&h}
}

func (c StrictHasherCompiler) Compile(params CompilationParams) CompiledStrictHasher {
	h := c.StrictHasherCompiler.Compile(([]func(iop *wizard.CompiledIOP))(params)...)
	return CompiledStrictHasher{&h}
}

func (h CompiledStrictHasher) GetCircuit() (StrictHasherCircuit, error) {
	c, err := h.CompiledStrictHasher.GetCircuit()
	return StrictHasherCircuit{&c}, err
}

func (h CompiledStrictHasher) GetHasher() StrictHasher {
	hsh := h.CompiledStrictHasher.GetHasher()
	return StrictHasher{&hsh}
}

func NewHasher(api frontend.API, maxNbKeccakF int) *Hasher {
	h := keccak.NewHasher(api, maxNbKeccakF)
	return &Hasher{h}
}

func (h *Hasher) Sum(nbIn frontend.Variable, bytess ...[32]frontend.Variable) [32]frontend.Variable {
	return h.Hasher.Sum(nbIn, bytess...)
}

func RegisterHints() {
	keccak.RegisterHints()
}

func (h StrictHasher) Skip(b ...[]byte) {
	h.StrictHasher.Skip(b...)
}

func (h StrictHasher) NbHashes() int {
	return h.StrictHasher.NbHashes()
}

func (h StrictHasher) MaxNbKeccakF() int {
	return h.StrictHasher.MaxNbKeccakF()
}

func (h StrictHasher) Assign() (StrictHasherCircuit, error) {
	c, err := h.StrictHasher.Assign()
	return StrictHasherCircuit{&c}, err
}

func (c StrictHasherCompiler) WithStrictHashLengths(l ...int) *StrictHasherCompiler {
	c.StrictHasherCompiler.WithStrictHashLengths(l...)
	return &c
}

func (c StrictHasherCompiler) WithFlexibleHashLengths(l ...int) *StrictHasherCompiler {
	c.StrictHasherCompiler.WithFlexibleHashLengths(l...)
	return &c
}

func (s StrictHasherSnark) Sum(nbIn frontend.Variable, bytess ...[32]frontend.Variable) [32]frontend.Variable {
	return s.StrictHasherSnark.Sum(nbIn, bytess...)
}

func (s StrictHasherSnark) Finalize() error {
	return s.StrictHasherSnark.Finalize()
}

func DummyCompile() CompilationParams {
	return CompilationParams{dummy.Compile}
}
