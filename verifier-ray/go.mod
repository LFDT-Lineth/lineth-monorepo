module github.com/consensys/linea-monorepo/verifier-ray

go 1.27.1

require (
	github.com/LFDT-Lineth/lineth-monorepo/arithmetization v0.0.0-20261124111202-ad191bf5dec9
	github.com/LFDT-Lineth/lineth-monorepo/prover-ray v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.12.1
)

require (
	github.com/LFDT-Lineth/zkc v1.2.37 // indirect
	github.com/bits-and-blooms/bitset v1.25.0 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/consensys/gnark v0.16.5-0.20261104230419-d5262f734b5b // indirect
	github.com/consensys/gnark-crypto v0.22.1-0.20261103155519-1a9aede5b1c3 // indirect
	github.com/fxamacker/cbor/v2 v2.9.2 // indirect
	github.com/google/pprof v0.0.0-20260802141513-ef3492d7dac3 // indirect
	github.com/mattn/go-colorable v0.1.15 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/ronanh/intcomp v1.1.1 // indirect
	github.com/rs/zerolog v1.35.1 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.57.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
)

// verifier-ray builds against the prover-ray checked out next to it, not a
// pinned pseudo-version: the generated verifier systems must track prover-ray
// HEAD. arithmetization stays on prover-ray/go.mod's pin so the two cannot
// drift.
replace github.com/LFDT-Lineth/lineth-monorepo/prover-ray => ../prover-ray
