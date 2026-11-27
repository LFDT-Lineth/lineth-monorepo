## [unreleased]

### 🚀 Features

- *(prover-ray)* Gnark wrapper for logderivsum and lookups (#4576)
- *(prover-ray)* Run the rollup guest in dev-zkvm and dev-partial (#4612)
- *(prover-ray)* Return and accept guestOutput next to publicInputs (#4613)
- *(prover-ray)* Ensuring that gnark wrapper does not need proof templates (#4465)
- *(prover-ray)* Add in-circuit E6Check for the gnark verifier (#4629)

### ⚡ Performance

- *(prover-ray)* Batch the chaining of group digests into pair leaves (#4623)
## [1.0.0] - 2026-11-26

### 🚀 Features

- *(prover-ray)* Decode the rollup guest output by job type (#4572)
- *(docker,misc)* [**breaking**] Harden Dockerfiles — non-root users, digest pinning, re-enable KICS (#4471)
## [0.5.0] - 2026-11-24

### 🚀 Features

- *(prover-ray)* Encode the rollup guest input (#4571)
- *(prover-ray)* Run zkc Execute with gogen by default (#4573)

### 🐛 Bug Fixes

- *(prover-ray)* Make the NonNative query sound (#4494)

### 🚜 Refactor

- *(coordinator, prover-ray, riscv-guest)* Rename the DA carry-over fields to match DaStreamPosition (#4556)
## [0.4.0] - 2026-11-23

### 🚀 Features

- *(prover-ray)* Dev-zkvm returns a devmode proof instead of marker bytes (#4569)
## [0.3.0] - 2026-11-23

### 🚀 Features

- *(prover-ray)* Support constant entries in lookup, permutation and message-bus tables (#4296)
- *(misc)* Adding blob commitment and a proof to Rollup input (#4329)
- *(prover-ray)* FRI over a fixed maximum domain (#4322)
- *(prover-ray)* Register program vk to the public inputs (#4309)
- *(prover-ray)* Prepare centralized prover (#4242)
- *(prover-ray)* Conglomerate the shard proofs behind a Merger interface (#4372)
- *(prover-ray)* Merge conglomeration nodes of one kind only (#4404)
- *(prover-ray)* Public-input chip for the recursion machine (#4437)
- *(prover-ray)* Add partial mode for l2 execution (#4455)
- *(prover-ray)* Remove deduplication (#4435)
- *(prover-ray)* Check the pipeline config at startup (#4463)
- *(prover-ray)* Decode dev-zkvm public inputs from the guest output (#4462)
- *(prover-ray)* Recursion merger with leaf, internal and root node programs (#4429)
- *(prover-ray)* In-machine verifier interface with a mock (#4438)
- *(prover-ray)* Bn packed hash (#4458)
- *(prover-ray)* Make the native runner optional in dev-zkvm (#4528)
- *(prover-ray)* Trace and prove shards in parallel (#4525)

### 🐛 Bug Fixes

- *(prover-ray)* Fix lookup soundness bug due to improper reuse of th… (#4363)
- *(prover-ray)* Adopt the guest log syscall in the prover-ray (#4371)
- *(prover-ray)* Bump arithmetization commit to support u16 addresses (#4396)
- *(prover-ray)* Fail the job when a shard does not fit in a column (#4464)
- *(prover-ray)* Send the proof's program VK as programVk, not the guest Program ID (zero in dev modes) (#4527)
- *(prover-ray)* Bump zkc to v1.2.37 to keep the RAM paged (#4536)

### 🚜 Refactor

- *(prover-ray)* Refactor the guest output fetching (#4370)
- *(prover-ray)* Drop the localvanishing and zkcdriver interners (#4480)

### ⚡ Performance

- *(prover-ray)* Preserve expression sharing through the localvanishing lift (#4155)
- *(prover-ray)* Parallelise and pool the R5 commits, opening, Lagrange evaluation and lookup (#4202)
- *(prover-ray)* Evaluate the quotient, log-derivative and grand-product rows with vector programs (#4203)
- *(prover-ray)* Evaluate each shared verifier subexpression once (#4157)
- *(prover-ray)* Intern every vanishing on registration (#4476)

### ⚙️ Miscellaneous Tasks

- *(misc)* Ship license files and metadata in all published artifacts (#4336)
- *(prover-ray)* Ship guest programs separately from the prover-ray image (#4515)
- *(prover-ray)* Remove nonnative zkcdriver tests (#4555)
## [0.2.0] - 2026-10-28

### 🚀 Features

- *(prover-ray)* Reduce Poseidon2 row authentication cost  (#4115)
- *(prover-ray)* Update ZkC for LazyTrace API (#4170)
- *(prover-ray)* Cyclic logderivativesum running sums (#4171)
- *(prover-ray)* Strictly decode native-runner --json output (#4231)
- *(prover-ray)* Start the Machine implementation (#4198)
- *(prover-ray)* Recursion machine Pack/Bits (#4215)
- *(prover-ray)* E6 and Select chips (#4216)
- *(prover-ray)* Gnark wrapper for the prover, trunk + FRI+ global vanishings (#4012)
- *(prover-ray)* Pack up to 4 fractions per log-derivative column (#4211)
- *(prover-ray)* Make Reed–Solomon blowup configurable per PCS (#4069)
- *(prover-ray)* Configurable PCS Merkle cap height (opt-in) (#4269)
- *(prover-ray)* Opt-in module groups in the PCS commitment layout (#4271)
- *(prover-ray)* Uniform pair leaves for module-grouped commitments (#4272)
- *(prover-ray)* Minimum size per dynamic module (#4288)
- *(riscv-guest)* Bump zesu to 2835883 and pass system contracts at runtime (#4265)
- *(prover-ray)* Per-module alpha_DEEP powers for module-grouped comitments (#4313)

### 🐛 Bug Fixes

- *(prover-ray)* Round arrengment for  program and trace (#4154)
- *(prover-ray)* Fetch guest output module (#4245)
- *(prover-ray)* Reject column elision and Merkle caps with module groups (#4286)
- *(misc)* Moved filtered addresses into the list of the PI (#4166)
- *(riscv-guest)* Aligning l2-execution guest program to the recently… (#4297)
- *(misc, riscv-guest, prover, coordinator)* Changing the spec to allow zero rolling hash and message number for the L2 Message Service (#4314)
- *(riscv-guest)* Renaming the kebab-case dirs to sneak case, which is more conventional in Zig world (#4320)

### 🚜 Refactor

- *(prover-ray)* Align request/response decoding with the current spec (#4229)
- *(prover-ray)* Configure wiop compilers per system and centralize the full pipeline- #4055 (#4256)
- *(prover-ray)* Merkle nodes take several injected digests (#4266)
- *(prover-ray)* Commit each group of a size with its own leaf hash (#4267)

### ⚡ Performance

- *(prover-ray)* Intern compound subexpressions in zkcdriver (#4153)

### ⚙️ Miscellaneous Tasks

- *(prover-ray)* Use minimal-elf guests in R5 tests instead of the verifier-ray (#4118)
- *(riscv-guest)* Bump zesu to a72dec7 (#4221)
- *(prover-ray,verifier-ray)* Bump arithmetization pin to #4255 (#4273)
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
