CORSET ?= corset

HUB_COLUMNS :=  $(wildcard hub/columns/*lisp)

HUB :=  $(wildcard hub/columns/*lisp) \
	$(wildcard hub/constraints/account-rows/*lisp) \
	$(wildcard hub/constraints/context-rows/*lisp) \
	$(wildcard hub/constraints/generalities/*lisp) \
	$(wildcard hub/constraints/heartbeat/*lisp) \
	$(wildcard hub/constraints/instruction-handling/call/*lisp) \
	$(wildcard hub/constraints/instruction-handling/call/generalities/*lisp) \
	$(wildcard hub/constraints/instruction-handling/call/finishing_touches/*lisp) \
	$(wildcard hub/constraints/instruction-handling/call/specialized/*lisp) \
	$(wildcard hub/constraints/instruction-handling/call/precompiles/*lisp) \
	$(wildcard hub/constraints/instruction-handling/call/precompiles/common/*lisp) \
	$(wildcard hub/constraints/instruction-handling/call/precompiles/ec_add_mul_pairing/*lisp) \
	$(wildcard hub/constraints/instruction-handling/call/precompiles/modexp/*lisp) \
	$(wildcard hub/constraints/instruction-handling/call/precompiles/blake/*lisp) \
	$(wildcard hub/constraints/instruction-handling/copy/*lisp) \
	$(wildcard hub/constraints/instruction-handling/create/*lisp) \
	$(wildcard hub/constraints/instruction-handling/create/constraints/*lisp) \
	$(wildcard hub/constraints/instruction-handling/halting/*lisp) \
	$(wildcard hub/constraints/instruction-handling/*lisp) \
	$(wildcard hub/constraints/miscellaneous-rows/*lisp) \
	$(wildcard hub/constraints/scenario-rows/shorthands/*lisp) \
	$(wildcard hub/constraints/scenario-rows/*lisp) \
	$(wildcard hub/constraints/storage-rows/*lisp) \
	$(wildcard hub/constraints/tx_skip/*lisp) \
	$(wildcard hub/constraints/tx_prewarm/*lisp) \
	$(wildcard hub/constraints/tx_init/*lisp) \
	$(wildcard hub/constraints/tx_finl/*lisp) \
	$(wildcard hub/constraints/*lisp) \
	$(wildcard hub/lookups/*lisp) \
	hub/constants.lisp


# Missing from the above
# $(wildcard hub/constraints/consistency/stack/*lisp) \
# $(wildcard hub/constraints/consistency/*lisp) \
# $(wildcard hub/constraints/consistency/account/*lisp) \
# $(wildcard hub/constraints/consistency/context/*lisp) \
# $(wildcard hub/constraints/consistency/execution_environment/*lisp) \
# $(wildcard hub/constraints/consistency/storage/*lisp) \

ALU := $(wildcard alu/add/*.lisp) \
       $(wildcard alu/ext/*.lisp) \
       $(wildcard alu/mod/*.lisp) \
       $(wildcard alu/mul/*.lisp)

BIN := bin   

BLAKE2f_MODEXP_DATA := blake2fmodexpdata

BLOCKDATA_FOR_REFERENCE_TESTS := blockdata/columns.lisp \
				 blockdata/constants.lisp

BLOCKDATA := blockdata

BLOCKHASH := blockhash

CONSTANTS := constants/constants.lisp

EC_DATA := ecdata

EUC := euc

EXP := exp

GAS := gas

LIBRARY := library/rlp_constraints_pattern.lisp

LOG_DATA := logdata

LOG_INFO := loginfo

MMU :=  $(wildcard mmu/*.lisp) \
	$(wildcard mmu/lookups/*.lisp) \
	$(wildcard mmu/instructions/*.lisp) \
	$(wildcard mmu/instructions/any_to_ram_with_padding/*.lisp)

MMIO := $(wildcard mmio/*lisp) \
	$(wildcard mmio/lookups/*lisp) 

MXP := mxp

OOB := oob

RLP_ADDR := rlpaddr

RLP_TXN := rlptxn

RLP_TXRCPT := rlptxrcpt			

ROM := rom

ROM_LEX := romlex

SHAKIRA_DATA := shakiradata

SHIFT :=  shf

STP := stp

TABLES := reftables/bin_reftable.lisp \
	  reftables/shf_reftable.lisp \
	  reftables/inst_decoder.lisp

TRM := trm

TXN_DATA := txndata

TXN_DATA_FOR_REFERENCE_TESTS :=  $(wildcard txndata/*.lisp) \
				 txndata/lookups/txndata_into_euc.lisp \
				 txndata/lookups/txndata_into_rlpaddr.lisp \
				 txndata/lookups/txndata_into_rlptxn.lisp \
				 txndata/lookups/txndata_into_rlptxrcpt.lisp \
				 txndata/lookups/txndata_into_romlex.lisp \
				 txndata/lookups/txndata_into_wcp.lisp

#				 txndata/lookups/txndata_into_blockdata.lisp \
#				 txndata/lookups/txndata_into_hub.lispX \

WCP := wcp

ZKEVM_MODULES := ${ALU} \
		 ${BIN} \
		 ${BLAKE2f_MODEXP_DATA} \
		 ${BLOCKDATA} \
		 ${BLOCKHASH} \
		 ${CONSTANTS} \
		 ${EC_DATA} \
		 ${EUC} \
		 ${EXP} \
		 ${GAS} \
		 ${HUB_COLUMNS} \
		 ${LIBRARY} \
		 ${LOG_DATA} \
		 ${LOG_INFO} \
		 ${MMIO} \
		 ${MMU} \
		 ${MXP} \
		 ${OOB} \
		 ${RLP_ADDR} \
		 ${RLP_TXN} \
		 ${RLP_TXRCPT} \
		 ${ROM} \
		 ${ROM_LEX} \
		 ${SHAKIRA_DATA} \
		 ${SHIFT} \
		 ${STP} \
		 ${TABLES} \
		 ${TRM} \
		 ${TXN_DATA} \
		 ${WCP}

# ${HUB} \

define.go: ${ZKEVM_MODULES}
	${CORSET} wizard-iop -vv -o $@ ${ZKEVM_MODULES}

zkevm.bin: ${ZKEVM_MODULES}
	${CORSET} compile -vv -o $@ ${ZKEVM_MODULES}


ZKEVM_MODULES_FOR_REFERENCE_TESTS := ${ALU} \
				     ${BIN} \
				     ${BLAKE2f_MODEXP_DATA} \
				     ${BLOCKDATA_FOR_REFERENCE_TESTS} \
				     ${BLOCKHASH} \
				     ${CONSTANTS} \
				     ${EC_DATA} \
				     ${EUC} \
				     ${EXP} \
				     ${GAS} \
				     ${HUB} \
				     ${LIBRARY} \
				     ${LOG_DATA} \
				     ${LOG_INFO} \
				     ${MMIO} \
				     ${MMU} \
				     ${MXP} \
				     ${OOB} \
				     ${RLP_ADDR} \
				     ${RLP_TXN} \
				     ${RLP_TXRCPT} \
				     ${ROM} \
				     ${ROM_LEX} \
				     ${SHAKIRA_DATA} \
				     ${SHIFT} \
				     ${STP} \
				     ${TABLES} \
				     ${TRM} \
				     ${TXN_DATA_FOR_REFERENCE_TESTS} \
				     ${WCP}

#				     ${BLOCKDATA} \
#				     ${HUB} \

zkevm_for_reference_tests.bin: ${ZKEVM_MODULES_FOR_REFERENCE_TESTS}
	${CORSET} compile -vv -o $@ ${ZKEVM_MODULES_FOR_REFERENCE_TESTS}
