GO_CORSET ?= go-corset

ALU := alu

BIN := bin   

BLAKE2f_MODEXP_DATA := blake2fmodexpdata

# with gaslimit for ethereum file
BLOCKDATA_FOR_REFERENCE_TESTS := $(wildcard blockdata/*.lisp) \
				 $(wildcard blockdata/processing/*.lisp) \
				 $(wildcard blockdata/processing/gaslimit/common.lisp) \
				 $(wildcard blockdata/processing/gaslimit/ethereum.lisp) \
				 $(wildcard blockdata/lookups/*.lisp)

# with gaslimit for linea file
BLOCKDATA_FOR_LINEA := $(wildcard blockdata/*.lisp) \
		       $(wildcard blockdata/processing/*.lisp) \
		       $(wildcard blockdata/processing/gaslimit/common.lisp) \
		       $(wildcard blockdata/processing/gaslimit/linea.lisp) \
		       $(wildcard blockdata/lookups/*.lisp)

BLOCKHASH := blockhash

CONSTANTS := constants/constants.lisp

EC_DATA := ecdata

EUC := euc

EXP := exp

GAS := gas

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
	$(wildcard hub/constraints/tx_init/rows/*lisp) \
	$(wildcard hub/constraints/tx_finl/*lisp) \
	$(wildcard hub/constraints/tx_finl/rows/*lisp) \
	$(wildcard hub/constraints/*lisp) \
	$(wildcard hub/lookups/*lisp) \
	hub/constants.lisp

## full HUB:
# HUB :=  hub
	# $(wildcard hub/constraints/consistency/stack/*lisp) \
	# $(wildcard hub/constraints/consistency/*lisp) \
	# $(wildcard hub/constraints/consistency/account/*lisp) \
	# $(wildcard hub/constraints/consistency/context/*lisp) \
	# $(wildcard hub/constraints/consistency/execution_environment/*lisp) \
	# $(wildcard hub/constraints/consistency/storage/*lisp) \

LIBRARY := library

LOG_DATA := logdata

LOG_INFO := loginfo

MMU :=  mmu

MMIO := mmio 

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

TABLES := reftables

TRM := trm

TXN_DATA := txndata

WCP := wcp

# Corset is order sensitive - to compile, we load the constants first
ZKEVM_MODULES := ${CONSTANTS} \
		 ${ALU} \
		 ${BIN} \
		 ${BLAKE2f_MODEXP_DATA} \
		 ${BLOCKDATA_FOR_LINEA} \
		 ${BLOCKHASH} \
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
		 ${TXN_DATA} \
		 ${WCP}

zkevm.bin: ${ZKEVM_MODULES}
	${GO_CORSET} compile -o $@ ${ZKEVM_MODULES}

# Corset is order sensitive - to compile, we load the constants first
ZKEVM_MODULES_FOR_REFERENCE_TESTS := ${CONSTANTS} \
					 ${ALU} \
				     ${BIN} \
				     ${BLAKE2f_MODEXP_DATA} \
				     ${BLOCKDATA_FOR_REFERENCE_TESTS} \
				     ${BLOCKHASH} \
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
				     ${TXN_DATA} \
				     ${WCP}


zkevm_for_reference_tests.bin: ${ZKEVM_MODULES_FOR_REFERENCE_TESTS}
	${GO_CORSET} compile -o $@ ${ZKEVM_MODULES_FOR_REFERENCE_TESTS}
