# Changelog

## beta-v1.2/v0.1.0-rc1

* Feat/blockdata separate eth linea constants (#560)
* Replace `NEGATIVE_OF_BLOCKHASH` with a `(defun ...)` expression (#561)
* fix callToC1Membership function (#558)
* feat: enable build for `zkevm.go.bin` (#559)
* `BLOCKHASH` redesign (#555)
* Implementation of TX_INIT and TX_FINL fix (#517)
* 526 blockdata redesign (#543)
* `EXTCODECOPY` requires the `CFI` as its `MMU_SRC_ID` only for addresses not currently under deployment (#554)
* Fix of the fix (Only unexceptional `ACCOUNT` instructions that `touchForeignAddress` update its warmth) (#553)
* Only unexceptional `ACCOUNT` instructions that `touchForeignAddress` update its warmth (#552)
* Fix mix up between account address / code address when executing `CODESIZE` / `SELFBALANCE` (#550)
* fix: `undoDepStatusAndNumberUpdate` wrong constraint fix (#551)
* Fixed divergence from spec in the "return data size" macro for `ECADD`/`ECMUL`/`ECPAIRING` (#549)
* Fix typos in 'instructions' and 'instruction' keywords (#544)
* File: README.md (#545)
* Include `HUB` into `ZKEVM_MODULES_FOR_REFERENCE_TESTS` (#548)
* Remove double negative from `ICPX` transfer `HUB -> MMU` (#547)
* CODECOPY sanity check fix (#546)
* `SELFDESTRUCT` typo fix (#542)
* Typo fix for `partialCopyOfReturnData` for `ECADD`/`MUL`/`PAIRING` (#541)
* fix: row offset of final context row in success case
* fix: provide OOB_INST_MODEXP_LEAD with ebs
* this corrects a typo where we were providing the OOB module with bbs twice
* MODEXP flag sums and NSR sums fixes (#540)
* Re-enabling certain lookups + TXN instructions fix (#539)
* Fixing PRC related shorthands (plenty of missing products) (#538)
* `SELFDESTRUCT` offset correction for account undoing operation (#537)
* Removing extraneous files (#536)
* Update daniel_discussion.md (#531)
* Type change for the scenario/XXX_GAS columns (:i32 bit to :i64) (#535)
* Splitting of the setting `NSR` and peeking flags constraint (#534)
* `CREATE` typo (#533)
* 527 extra `scenario/CREATE` shorthand (#528)
* Removing trailing white spaces (#530)
* Update columns.lisp file (#523)
* `CREATE` pricing in `scenario/CREATE_FAILURE` cases (#520)
* `REFUND` increment rationalization for `SSTORE` (#525)
* Provide HUB -> RLPADDR with correct `init_code_hash` (#522)
* ras: formatting
* `HUB` debugging continued (#514)
* The `HUB` debugging never stops (#512)
* fix: exo sum must be decoded for LIMB_VANISHES MMIO inst (#509)
* remove nbAdded and nbAdded (#511)
* feat(mmio): plug MMIO (#313)
* fix: REVERT was incoherently updating return data twice
* fixes a typo
* ras: renaming and formatting
* More HUB debugging related changes (#507)
* MXP to ID lookup fix 52 (#479)
* fix: splitting "setting-the-CREATE-scenario" into subconstraints
* Implement simplified return data setting for `RETURN`'s from deployments (#505)
* Make the constraints compile again (#503)
* REMAINING TODOs exp and mxp (#487)
* Yet more veridise debugging (#480)
* Remaining todos endgame (#493)
* More `HUB` debugging (#502)
* Remove `(vanishes! 0)` (#496)
* Adding `HUB` constraints (#494)
* feat: update `corset` to the latest version (#498)
* some HUB constraints debugging (#485)
* fix(logdata): fix nBYTES check only if logs data (#491)
* Duplicate and unused code snippets EXP OOB MXP (#489)
* delete duplicate constraint (#471)
* Update Makefile (#490)
* Adding unpermuted account columns (#482)
* More Veridise debugging (#477)
* revert add HUB constraints (#483)
* add HUB constraints
* typo (#475)
* homogenize lookups rlptxn <-> hub (#473)
* Specification glitches (#466)
* Implementation of HUB / MXP interface bugfix (#469)
* Typo and bug fixes for the `ACC` instruction family (#462)
* Make `HEIGHT` and `HEIGHT_NEW` explicitly `hub-stamp-constant` (#464)
* Fix order of stack items in lookup HUB -> EXT (#460)
* 363 add gas to makefile (#380)
* No longer impose vanishing of GAS_COST in case of nonOogException (#458)
* Constraints update for GAS columns generalities (#457)
* Using `stack/STATIC_GAS` rather than `GAS_CONST_G_SELFDESTRUCT` (#455)
* fix: add CFI to the lookup txnData into rlpTxn (#439)
* RETURNDATACOPY must always load the current execution context (#449)
* Re-introduction of generic constraints for `HEIGHT_NEW` and `HEIGHT` in relation to `ALPHA` and `DELTA` (#453)
* delete size into nBytes in the lookup mmio into rom (#443)
* Add `GAS_LIMIT` to `HUB -> TXN_DATA` lookup (#451)
* `MXP` Missing Type Annotations (#412)
* `EC_DATA` Missing Type Annotations (#410)
* fix: typing of columns (#408)
* Createe reverts with child (itself, that is) (#433)
* Lookup selector's for `gas-into-wcp` hadn't been updated (#435)
* Add missing _NEW's to TX_INIT and TX_SKIP sections (#444)
* MSTORE8 now treated as type 3 instruction in STACKRAM instruction family and typo fix (#445)
* Copy instructions use stack items in the wrong order (#447)
* 424 implicitdebug constraints in implementation (#440)
* fix(constants): correct refund constants (#417)
* Deployment transactions should check more vanishing constraints (#438)
* Various tiny fixes (#437)
* MXP and MMU stamp increments (#432)
* Added missing CALL_FLAG and CREATE_FLAG to HUB -> ID lookup (#431)
* Fix value constraints for EXTCODESIZE / EXTCODECOPY (#419)
* Typing for gas columns in the HUB and STP (#423)
* Fix for type issues in MUL (#421)
* fix(mxp): add constancy constraints for CN and DEPLOYS (#415)
* clean(OOB): delete binary dupliacte constraints + from `call_gas` to `callee_gas` (#413)
* fix(rlpTxn): constraint ADDR during AccessList tuple & ChainId (#403)
* `TXN_DATA`: fix for`priorityFeePerGas` (#406)
* TXN_DATA missing constraints + switch to positive conditions (#401)
* `BLOCKDATA` fixes in the wake of that module blowing up for Leo and Alex (#398)
* fix: make it compile
* fix: splitting of constraints + formatting
* fix(mmio): missing constraint + typo + overconstrained (#371)
* fix(shf): remove unnecessary binary constraint (#367)
* fix(blockData): missing finalization constraint (#374)
* fix(ROM): missing and duplicate constraint (#365)
* fix(rlprcpt): precondition constraint error (#376)
* fix: initializing the  `EXPONENT_BIT_ACCUMULATOR` (#391)
* Implementation of `MMU` update (#388)
* Implementation of `MMIO` update (#386)
* fix(txnData): missing constraint (#378)
* fix(BIN): remove unnecessary preconditions (#381)
* fix typo in RLP_TXN phase Beta (#369)
* Constraining the `BIT_3` column in the `EXT` module (#395)
* Fix: type of `SHB_k_XX` columns (#393)
* fix(lookups): notation for selectors (#383)
* ras
* Fixed HUB typo in `(stateless-instruction---precondition)` (#362)
* fix(oob): modexp extract and blake params (#360)
* fix(rlpTxn): typo for small signature (#356)
* fix(exp): index lookups (#358)
* fix(exp): remove not necessary computation (#353)
* fix(ecdata): add counter constancy on NOT_ON_G2 and NOT_ON_G2_ACC (#351)
* Remove `GAS` from `ZKEVM_MODULES` and `ZKEVM_MODULES_FOR_REFERENCE_TESTS` (#349)
* fix(lookups): folder structure (#347)
* Lookup fix (#345)
* fix(gas): lookup row 1 (#344)
* delete unwanted file (#342)
* Separate zkevm.bin for reference tests and associated make instruction (#340)
* Fixed the debug constraints (#338)
* fix(mmu): add missing constraint (#324)
* clean: delete unused function containig a typo (#326)
* fix(exp): add finalization constraint (#334)
* fix(exp): use CT_MAX_CMPTN_MODEXP_LOG (#336)
* Consistency argument implementation + `FIRST`, `FINAL` now work at the block level for state manager (#310)
* Removing refunds for `SELDESTRUCT`'s (#328)
* fix(oob): inst modexp pricing f of max (#320)
* fix(lookups): source-into-target-... kebab-case (#322)
* feat(constants): more constants (#318)
* fix(mmu): typo (#316)
* Re-enable STP module in constraints (#315)
* feat(TxnData): switch on module constraint (#312)
* fix(hub): BATCH_NUMBER => RELATIVE_BLOCK_NUMBER
* fix(oob): kill not necessary prc hypothesis (#308)
* fix(oob): modexp lead constraints (#306)
* Added new XAHOY column to HUB -> GAS lookup (#272)
* fix(oob): improved notation (for compassion to future ourselves) and remove old constraint OOB_INST_BLAKE_PARAMS (#304)
* fix(oob): oob inst modexp lead (#300)
* Removed deprecated column names (#303)
* Feat/issue 270/relispify gas (#292)
* Implement EIP 3607 (#298)
* SHF: quick and dirty fix (#290)
* fix(txn_data): revert wrong fix (#283)
* HUB update to handle the `MAX_NONCE` business (#288)
* Fix/issue 295/fix while debuging the hub (#296)
* feat: add EIP2681_MAX_NONCE global constant (#294)
* feat(txn-data): implement max nonce comparaison (#291)
* feat(oob): introduce max_nonce for CREATE (#285)
* docs: retires zk-EVM (#260)
* feat(exp): activate (#281)
* fix(makefile): comment out HUB, EXP and STP (#280)
* fix(txn_data): typo (#278)
* Feat/issue 261/lispify exp revision (#277)
* feat(exp): update constraints (#275)
* fix(mmu): typo in BIN and OUT constancies (#274)
* STP lisp update and review (#269)
* CALL instruction stack pattern related fixes (#267)
* Missing type annotations for EXT (#263)
* Missing type annotations for the MUL module (#265)
* Hub constraints debugging (#224)
* 257 user docs (#259)
* fix(ecdata): turn index to i16 (#256)
* ras
* Fix/issue 253/spliting scenario call abort (#254)
* fix(rlprcpt): index column overflows (#252)
* fix(ecdata): ecdata debug (#251)
* fix(ecdata): refined constraints (#249)
* fix(ecdata): ACCPC (#247)
* MODEXP implementation done
* perf(constants): add MAX_CODE_SIZE constant to constants.lisp (#244)
* fix(ecdata): ecdata debug (#242)
* feat(ecdata): implemented missing ecpairing constraints (#239) (#240)
* feat(ecdata): implemented constraints for ecadd ecmul and ecpairing (#235) (#237)
* Fix `loginfo` guards (#238)
* Fix build rule for `define.go` (#234)
* Removed hub.transaction/PRIORITY_FEE_PER_GAS column
* style(loginfo): rewrite verticalisation constraint guard (#233)
* fix(gas): gas constants (#228)
* fix: disable hub (#227)
* fix: remove a deprecated corset argument -P from Makefile (#226)
* typo
* zkevm.bin compiles again + bugfixes
* feat(hub): Lispification of CALL's + precompiles (#193)
*     - MODEXP is still wip
*     - ECADD, ECMUL, ECPAIRING have only the common phase
*     - BLAKE2f is as of yet untouched
* fix(oob): debug constraints (#219)
* feat: enable blakemodexp (#217)
* feat(blakemodexp): implem module (#214)
* feat: disable OOB, BLAKE2f_MODEXP_DATA and EXP modules (#216)
* feat(oob): update opcodes constraints (#212)
* fix(logdata): useless constraints (#194)
* fix(mxp): extracted constants (#211)
* feat(romlex): add codehash columns (#209)
* feat(shakira): add result bit selector (#207)
* feat: updated OOB constraints for precompiles (#42) (#205)
* feat: updated OOB constraints for precompiles (#42)
* fix(shfreftable): turned mshp from i8 to byte (#203)
* fix(stp): missing constancy constraint (#195)
* feat(shakira): delegate to wcp (#201)
* fix: remove gas and shakiradata from Makefile (#198)
* Feat/issue 189/add result keccak selector (#196)
* feat(hub): lispification of CREATE's + type annotations for HUB columns (#192)
* fix(exp): fixed constants (#191)
* fix(oob): capitalized columns names (#188)
* fix: renaming issues and disable hub (#181)
* Resolves: #180
* Signed-off -by: Tsvetan Dimitrov <tsvetan.dimitrov@consensys.net>
* fix(ecdata and oob): constants naming and visibility (#179)
* Resolves: #178
* feat(ecdata): ECRECOVER constraints for use by the prover team (#175)
* feat(hub): corset implementation of all HUB features except for CALLs and CREATEs (#174)
* Resolves issue #128
* feat(txndata): implement eip 3529 (#169)
* fix(gas): added @prove to BYTE (#172)
* fix(bin): low4 bytehood constraint useless (#171)
* fix(rom): missing bytehood constraint (#170)
* feat: implem blockdata and blockhash (#159)
* feat: add package naming consistency (#161)
* Resolves: #160
* feat(mod): slight modif as spec was updated (#158)
* fix: rename txn_data to txndata (#156)
* fix: fix some faulty shakira constraints (#155)
* feat(txnData): update (#146)
* fix: rename shakira package (#154)
* fix(exp): removed and operator (#153)
* feat(rlpAddr): add lookup to trm (#150)
* feat: unify interface with prover when giving data to kec (#145)
* fix(mmu): more debuging (#148)
* fix(mmu, mmio): debuging (#142)
* feat: new mmu  (#78)
* fix(shakira): rename ripsha to shakira_data (#136)
* build: update corset
* minor typo
* Moved constants (#135)
* Gas constants (#132)
* Gas module (#130)
* Feat/issue 115/add yields nontrivial mmu operation column to the mxp (#129)
* Update constants.lisp
* values of empty KECCAK, SHA2-256 and RIPEMD-160
* Update constants.lisp
* VALUE -> WEIGHT
* Unambiguous decoding of EXO flags (#127)
* added MXP flag sum constants (#126)
* feat(constant): add exo sum values (#125)
* OOB added columns and constants (#123)
* fix: re-type RlpTxRcpt columns
* fix: disable Hubv2
* fix(txnData): typo (#121)
* fix(logIngo): PHASE can be two bytes (#120)
* Hub v2 (#101)
* fix(txnData): INITIAL_BALANCE could be greater than i64 (#116)
* RIPSHA module Corset implementation (#114)
* fix(binRT): we can't IOMF the lookup to RT (#110)
* fix(shf): lookup shf to shfRT (#104)
* EVM instruction constants (#106)
* Feat/issue 97/bin and shf lookup to ref table (#98)
* fixed EXP instruction names (#100)
* feat(blake2f-modexp-data): add constraints for module BLAKE2f_MODEXP_DATA (#96)
* Resolves: #75
* EXP renamed perspectives (#95)
* EXP constraints final version (#93)
* feat(rlp*): new phase column + add constant (#91)
* feat(txnData): add check non empty tx call data (#89)
* perf: bin adaptative line (#66)
* perf(wcp): isZero is zero (#87)
* feat: revert addition of microdata constants (#84)
* Resolves: #82
* feat(microdata): add microdata module, containing constants (#83)
* Resolves: #82
* perf(EUC): less constraint (#80)
* perf: better typing + range proof (#77)
* build: update Corset
* build: update Corset
* fix(EUC): bug fix + proving binarity (#72)
* deps: update Corset to 9.3.0 (#73)
* feat(modexp-data): add contraints for MODEXP_DATA module (#70)
* Resolves: #68
* feat: implement EUC module (#65)
* perf: smart nb of lines for ADD (#62)
* perf: several wcp perf (#60)
* ci: cleaner access to Corset binaries
* feat: add CI
* feat: plug txnData (#56)
* fix: plug txnData constraints (#47)
* fix: hide txnData constraints (#54)
* Feat/issue 49/reimplement bin module (#53)
* Feat/issue 11/stp reimplementation (#40)
* Feat/issue 16/implement log info and log data (#41)
* fix: move mmio and mmu module constraints to the root of the project (#45)
* fix(tx-chain-id): fix for missing chainId on current transaction (#37)
* TRM: missing binary constraints (#38)
* kill that fucking file
* Trm constraints (#34)
* chore: plookup -> lookup
* chore: update to Corset 9 (#35)
* feat: ROM/ID lookup (#33)
* ADD padding fix (#31)
* Feat/issue 5/txn data module (#27)
* new instruction decoder (#28)
* Update constraints.lisp (#25)
* Omission spotted by Alex V.
* build: clean up the Makefile
* Kill that f****** thing
* Merge: 94388b1 1dc98f8
* Merge branch 'mxp_v2'
* Merge: fd23dbb 678c328
* Merge pull request #26 from Consensys/feat/issue-24/ROM
* Feat/issue 24/rom
* style: styme review
* refactor: ras
* refactor: ras
* style: rename column
* feat(romLex): add constraints
* fix: fix lookup filter
* feat: implem constraints and lookup
* feat: rom romLex columns implem
* Fixed constraints related to MXP module
* feat(rom): wip
* feat(rom): wip
* feat: aaddresses RLP encoding
* Fixes #14
* Fixed names of constraints
* MXP: created lookup folder
* Feat/issue 13/implementation rlp txrcpt (#22)
* Transaction RLP (#15)
* MXP revision after spec modification
* Merge: b0b8c0e 6defc7c
* Merge pull request #3 from macfarla/trm-constraints
* Update constraints.lisp
* Respect order of specification + added missing constraints
* Update columns.lisp
* reflect column order in specification
* fix: change comp column to boolean (#20)
* fixes #19
* fixed shift syntax
* WIP bit-decomposition-associated-with-ONES
* fixed  byte decomposition constraint for ACC_T
* removed comments
* finished pbit constraints
* Merge: eeea459 ffaad81
* merge main
* Merge: 8837035 1d9252b
* Merge pull request #17 from Consensys/feat/issue-5/txn-data-module
* Implement `txnData` module
* Merge: 5ed55ef f6436d0
* Merge pull request #12 from Consensys/feat/issue-11/stp-reimplementation
* STP module constraints (new specification)
* fix: tagged exo_inst as byte column
* fix(corset-compiles): made corset compile
* moved lookups to a dedicated folder
* feat: finished the bingo card constraints
* feat(implementation): transcribed the re-specified STP module
* style: white space
* feat: added COINBASE columns; REQUIRES_EVM_INSTRUCTION is boolean
* feat(lookups): added wcp lookup; others are still TODO
* fix: made corset compile & added a missing column
* feat: finished module constraints (gas and gas price)
* Resolves #10 and #6
* refactor: mixed up source and target in ec_data module lookups
* feat: implementation of the comparison constraints
* feat: added aliases; verticalization constraints
* feat: implemented constraints up to an including section 2.5 Cumulative gas
* style: changed column names and added aliases
* feat(constraints): first constraints; added missing columns;
* Pertains to #5
* feat(column-names): created the columns.lisp file
* Pertains to #5
* hotfix: disable the ROM
* Disable txRLP
* Re-enable txRlp
* Update old functions
* Unclutch TX_RLP for now
* Remove obsolete modules
* new syntax 'definterleaved'
* feat: implement the `txRlp` module (#4)
* Unused columns
* Revert error
* Make it compile
* added more constraints
* added 2.5 target constraints
* Merge: 657f5e1 adc2b58
* Merge branch 'trm-constraints' of github.com:macfarla/zkevm-constraints into trm-constraints
* added byte decomp for ACC_T
* added vanishing conditions for other cols
* removed theta
* function names
* Merge: 00e9022 19f0e1c
* merge
* refactor: stdlib renaming
* Update function names
* Update constraints to loobeans
* new column: ALL_CHECKS_PASSED
* fix bug
* merge c1 membership conditions
* fix: only check heartbeat outside of padding
* fix bug
* remove tau(u)
* Update constraints.lisp
* I added the constraints around the pivot bit column PBIT
* fix: insert missing columns
* remove useless constraint (ecmul wcp s == 0)
* style
* fix bug
* fix bug
* fix bug
* factorize
* fix bug
* standardization
* standardization of wcp ecmul lookup
* fix lookup-ecpairing-wcp
* fix bug lookup-ecmul-wcp
* typo
* fix bug lookup-ecrecover-wcp
* fix lookup-ecmul-wcp
* fix bug in lookups
* fix bug lookup-ecadd-ext
* fix bug not-on-g2-acc-activation-condition
* fix bug connection-constraints
* fix bug point-infinity
* typo
* reordering columns
* add a TODO (hub_into_ecdata)
* update makefile
* plookups
* done
* almost done
* wip
* update flags
* Add missing columns
* fix bug: switch target / source in mxp_into_instruction_decoder
* fixed compile errors
* add to makefile
* fixed col names
* Merge: 7f63582 2e66c0f
* Merge branch 'master' of github.com:ConsenSys/zkevm-constraints into trm-constraints
* 0x
* ones
* gitignore (#2)
* trm
* fix bug (is-not-zero ROOB)
* Small update
* Add MXP constraints
* prettier arrows
* Update to new permutation syntax
* disable RLP for now
* Add Makefile
* Adjustments
* Move some files
* Initial import from zk-geth
