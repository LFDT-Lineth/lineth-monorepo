## [0.1.1] - 2026-10-06

### 🐛 Bug Fixes

- *(prover-ray)* Set endBlockNumber in dev-mock responses (#4137)
## [0.1.0] - 2026-10-06

### 🚀 Features

- *(prover-ray)* Support dynamic module sizes in global compiler (#3097)
- *(prover-ray)* Migrate KoalaBear extension field from degree-4 to degree-6 (#3111)
- *(prover-ray)* Add LagrangeSelectors in the wizard framework (#3275)
- *(prover-ray)* Implements FRI in the cryptography package (#3337)
- *(prover-ray)* Proper Prove / Verify API with an explicit Proof object (#3334)
- *(prover-ray)* Implements a basic mutator for fuzz testing the prover (#3321)
- *(prover-ray)* Adding logbus queries, docs, compiler and tests. pre-sampling FS hooks, and dynamic support for logderivsum (#3358)
- *(prover-ray)* Cleanup behind FRI in crypto (#3353)
- *(prover-ray)* Fri pcs api proposal (#3415)
- *(prover-ray)* PCS wrapper for FRI (#3440)
- *(prover-ray)* Update WIOP to support the FRI compiler (#3441)
- *(prover-ray)* Implement the FRI PCS compiler (#3445)
- *(prover-ray)* Adding public inputs (#3467)
- *(prover-ray)* Message bus extensions: permutations and grand-products (#3496)
- *(prover-ray)* Added pcs compilation in the pipeline (#3579)
- *(prover-ray)* Mock backend for prover-ray (#3530)
- *(prover-ray)* Update zkc integration (#3580)
- *(prover-ray)* Implement non-native query (#3575)
- *(prover-ray)* Added pcs benchmark (#3571)
- *(prover-ray)* Fixed static check (#3645)
- *(prover-ray)* Ssz encoder (#3599)
- *(prover-ray)* Filesystem job adapter (#3600)
- *(prover-ray)* Implement basic secp256k1 operations in zkc (#3605)
- *(prover-ray)* Zkcdriver R5 integration (#3691)
- *(prover-ray)* Add the public-input-layout for the recursion (#3679)
- *(prover-ray)* Remove Visibility in wiop (#3476)
- *(prover-ray)* Preflight helpers (#3743)
- *(prover-ray)* Export lookup row-limit verifier action (#3765)
- *(prover-ray)* Shared randomness public input (#3766)
- *(prover-ray)* Bumping zkc version (#3809)
- *(prover-ray)* Rename guestProgramId to programVk (#3815)
- *(prover-ray)* Added guest output to the public inputs (#3817)
- *(riscv-guest)* L2 execution guest rollup (#3783)
- *(riscv-guest)* L2 exec guest coverage test harness (#3784)
- *(prover-ray)* Proof serde toward verifier-ray (#3794)
- *(prover-ray)* Export grandproduct.RowLimitAction for verifier-ray (#3915)
- *(prover-ray)* Update prover-ray to ZkC version v1.2.31 (#3884)
- *(prover-ray)* Add guest ELFs to cover testing the full RISC-V instruction set (#3862)
- *(riscv-guest)* Rollup guest program stub with shared SSZ package (#3894)
- *(riscv-guest)* Added a makefile target to run reference tests wit… (#3906)
- *(prover-ray)* Add mock rollup and aggregation proof paths (#3997)
- *(riscv-guest)* Adding fallback precompile implementations (#3931)
- *(prover-ray)* Add dev-mock and dev-zkvm mode (#4033)
- *(riscv-guest)* RiscV guests release workflow (#4102)
- *(riscv-guest)* Setting up the release workflow (#4119)

### 🐛 Bug Fixes

- *(prover-ray)* Verify prover-supplied FRI query points (#3356)
- *(prover-ray)* Repair broken logderivative pipeline tests (#3591)
- *(prover-ray)* FRI folding edge cases for modules of size=1 (#3610)
- *(prover-ray)* Bench program static errors (#3649)
- *(prover-ray)* Harden blob input parity checks (#3648)
- *(prover-ray)* Guard SSZ allocation sizes (#3669)
- *(prover-ray)* Adding limit checks for the lookups and permutations (#3619)
- *(prover-ray)* Fixes the broken tests (#3770)
- *(prover-ray)* Range check bug resolved (#3792)
- *(prover-ray)* Bug in proofserialization roundcount (#3829)
- *(riscv-guest)* Fixing the write_output usage (#3909)
- *(prover-ray)* Update minimal-elf import
- *(prover-ray)* Remove presampling and hooking from Fiat-Shamir path (#3950)
- *(prover-ray)* Fetch data from hash module (#4023)
- *(riscv-guest)* Rename parentProcessedFtxNumber to parentFtxNumber (#4011)
- *(prover-ray)* Align Amsterdam SSZ inputs with the guest (#4084)
- *(prover-ray)* Create smoke-test work dir under RUNNER_TEMP (#4136)

### 🚜 Refactor

- *(prover-ray)* Rename misleading field names to digest/fext (#3178)
- *(prover-ray)* Export logderivativesum internals for verifier-ray codegen (#3354)
- *(prover-ray)* FRI cleanups (#3541)
- *(prover-ray)* Refactor backend core and add unit tests (#3598)
- *(prover-ray)* Aux leaves as pairs (#3573)
- *(prover-ray)* Prover backend (#3661)
- *(prover-ray)* Export PCS batch/layout helpers and OpeningVeri… (#3724)
- *(prover-ray)* Use the arithmetization Go pkg (#3877)
- *(prover-ray)* Move `TestABIAgreement` to verifier-ray (#3945)
- *(prover-ray)* Refactor bus message (#3940)

### ⚡ Performance

- *(prover-ray)* Switch U_alpha from evaluation form to monomial coefficient form  (#3092)
- *(prover-ray)* `Vanishing -> VanishingManual` for lifted local constraint (#3241)
- *(prover-ray)* Speed up FRI PCS commit/open/verify (27–82×) (#3650)
- *(prover-ray)* 5x faster R5 prove (#3736)
- *(prover-ray)* 2x faster FRI commit (#3749)
- *(prover-ray)* Optimizes the CI test for the prover town to 50sec. (#3810)
- *(prover-ray)* Merkle capping (#3828)

### ⚙️ Miscellaneous Tasks

- *(prover-ray)* Update the go module to match LFDT github path (#3335)
- *(misc)* Enrich R5 request/response samples (#3806)
- *(riscv-guest)* More tests (#3785)
- *(riscv-guest)* Error codes and their coverage (#3786)
- *(riscv-guest)* More coverage for the multi block case. Added an … (#3795)
- *(prover-ray)* Add image draft releases (#4104)
