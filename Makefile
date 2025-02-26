GO_CORSET ?= go-corset
GIT_COMMIT := $(shell git rev-parse HEAD)
GIT_TAGS := $(shell git -P tag --points-at)
TIMESTAMP := $(shell date)
GO_CORSET_COMPILE := ${GO_CORSET} compile -Dtags="${GIT_TAGS}" -Dcommit="${GIT_COMMIT}" -Dtimestamp="${TIMESTAMP}"

ALU := alu

BIN := bin   

BLAKE2f_MODEXP_DATA := blake2fmodexpdata

# constraints used in prod for LINEA, with linea block gas limit
BLOCKDATA_FOR_LINEA := $(wildcard blockdata/*.lisp) \
		       $(wildcard blockdata/processing/*.lisp) \
		       $(wildcard blockdata/processing/gaslimit/common.lisp) \
		       $(wildcard blockdata/processing/gaslimit/linea.lisp) \
		       $(wildcard blockdata/lookups/*.lisp)

# with gaslimit for old replay tests (ie without the constraint BLOCK_GAS_LIMIT = LINEA_BLOCK_GAS_LIMIT = 2 000 000 000)
BLOCKDATA_FOR_OLD_REPLAY_TESTS := $(wildcard blockdata/*.lisp) \
				 $(wildcard blockdata/processing/*.lisp) \
				 $(wildcard blockdata/processing/gaslimit/common.lisp) \
				 $(wildcard blockdata/processing/gaslimit/old_replay_test.lisp) \
				 $(wildcard blockdata/lookups/*.lisp)

# with gaslimit for ethereum file (used for reference tests)
BLOCKDATA_FOR_REFERENCE_TESTS := $(wildcard blockdata/*.lisp) \
				 $(wildcard blockdata/processing/*.lisp) \
				 $(wildcard blockdata/processing/gaslimit/common.lisp) \
				 $(wildcard blockdata/processing/gaslimit/ethereum.lisp) \
				 $(wildcard blockdata/lookups/*.lisp)

BLOCKHASH := blockhash

CONSTANTS := constants/constants.lisp

EC_DATA := ecdata

EUC := euc

EXP := exp

GAS := gas

HUB :=  hub

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
	${GO_CORSET_COMPILE} -o $@ ${ZKEVM_MODULES}

# Corset is order sensitive - to compile, we load the constants first
ZKEVM_MODULES_FOR_OLD_REPLAY_TESTS := ${CONSTANTS} \
					 ${ALU} \
				     ${BIN} \
				     ${BLAKE2f_MODEXP_DATA} \
				     ${BLOCKDATA_FOR_OLD_REPLAY_TESTS} \
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


zkevm_for_old_replay_tests.bin: ${ZKEVM_MODULES_FOR_OLD_REPLAY_TESTS}
	${GO_CORSET_COMPILE} -o $@ ${ZKEVM_MODULES_FOR_OLD_REPLAY_TESTS}

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
	${GO_CORSET_COMPILE} -o $@ ${ZKEVM_MODULES_FOR_REFERENCE_TESTS}
