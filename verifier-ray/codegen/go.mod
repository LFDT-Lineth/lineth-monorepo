module github.com/consensys/linea-monorepo/verifier-ray/codegen

go 1.25.7

require (
	github.com/LFDT-Lineth/lineth-monorepo/prover-ray v0.0.0-20260907102024-e1ed771dd796
	github.com/LFDT-Lineth/zkc v1.2.32
	github.com/stretchr/testify v1.11.1
)

require (
	github.com/LFDT-Lineth/lineth-monorepo/arithmetization v0.0.0 // indirect
	github.com/bits-and-blooms/bitset v1.24.4 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/consensys/gnark v0.14.1-0.20260219004710-bbfb2f70a565 // indirect
	github.com/consensys/gnark-crypto v0.20.2-0.20260807171631-a71790dd0fe3 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fxamacker/cbor/v2 v2.9.1 // indirect
	github.com/google/pprof v0.0.0-20260402051712-545e8a4df936 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.21 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rogpeppe/go-internal v1.16.0 // indirect
	github.com/ronanh/intcomp v1.1.1 // indirect
	github.com/rs/zerolog v1.35.1 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// prover-ray's zkc-r5 backend imports arithmetization/gopkg/{elfmapping,predecoding},
// and prover-ray/go.mod requires arithmetization at the placeholder v0.0.0,
// resolving it with a relative-path replace (=> ../arithmetization). Go applies
// replace directives only from the main module, so that placeholder is
// unresolvable here and has to be mapped to a real revision. Pinned to the same
// commit as prover-ray above so both come from one monorepo snapshot; bump the
// two together.
replace github.com/LFDT-Lineth/lineth-monorepo/arithmetization v0.0.0 => github.com/LFDT-Lineth/lineth-monorepo/arithmetization v0.0.0-20260907102024-e1ed771dd796
