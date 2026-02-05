GO_CORSET ?= go-corset
GIT_COMMIT := $(shell git rev-parse HEAD)
GIT_TAGS := $(shell git -P tag --points-at)
TIMESTAMP := $(shell date)
GO_CORSET_COMPILE := ${GO_CORSET} compile -Dtags="${GIT_TAGS}" -Dcommit="${GIT_COMMIT}" -Dtimestamp="${TIMESTAMP}"

# Modules setting
## Some modules set below are fork specific. Eg. For OOB, OOB_LONDON is the OOB module for London and OOB_SHANGHAI the OOB module for Shanghai.
## The discrimination is done by having one bin file per fork - see command line below

ALU := alu/add.zkasm alu/ext.zkasm alu/mod.zkasm alu/mul.zkasm

BIN := bin/bin.zkasm

BLAKE2f := blake2f/blake2f.zkasm

BLAKE2f_MODEXP_DATA  := blake2fmodexpdata

# constraints used in prod for LINEA, with linea block gas limit
BLOCKDATA := blockdata

BLOCKHASH := blockhash

BLS := blsdata

CONSTANTS := constants/constants.lisp
CONSTANTS_OSAKA := constants/osaka/constants.zkasm

EC_DATA := ecdata

EUC := euc/euc.zkasm

EXP := exp/exp.zkasm

GAS := gas/gas.zkasm

HUB_OSAKA :=  hub/osaka

LIBRARY := library

LOG_DATA := logdata

LOG_INFO := loginfo

MMIO := mmio

MMU := mmu

MXP := mxp

OOB := oob/oob.zkasm

RLP_ADDR := rlpaddr

RLP_TXN := rlptxn

RLP_TXN_RCPT := rlptxrcpt

RLP_TXRCPT := rlptxrcpt

RLP_UTILS := rlputils/rlputils.zkasm

ROM := rom

ROM_LEX := romlex

SHAKIRA_DATA := shakiradata

SHIFT :=  shf/shf.zkasm

STP := stp/stp.zkasm

TABLES := reftables

TRM := trm/trm.zkasm

TXN_DATA := txndata

WCP := wcp/wcp.zkasm

LISPX := $(shell find * -name "*.lispX")
# Warn about any lispX files
define warn_lispX
	@for FILE in ${LISPX}; do (echo "WARNING: $$FILE"); done
endef

ZKEVM_MODULES_COMMON := ${CONSTANTS} \
		 ${TABLES} \
		 ${ALU} \
		 ${BIN} \
		 ${BLAKE2f} \
		 ${BLAKE2f_MODEXP_DATA} \
		 ${BLOCKDATA} \
		 ${BLOCKHASH} \
		 ${BLS} \
		 ${EC_DATA} \
		 ${EUC} \
		 ${EXP} \
		 ${GAS} \
		 ${LIBRARY} \
		 ${LOG_DATA} \
		 ${LOG_INFO} \
		 ${MMIO} \
		 ${MMU} \
		 ${MXP} \
		 ${OOB} \
		 ${RLP_ADDR} \
		 ${RLP_TXN} \
		 ${RLP_TXN_RCPT} \
		 ${RLP_UTILS} \
		 ${ROM} \
		 ${ROM_LEX} \
		 ${SHAKIRA_DATA} \
		 ${SHIFT} \
		 ${STP} \
		 ${TRM} \
		 ${TXN_DATA} \
		 ${WCP}

ZKEVM_MODULES_OSAKA := ${ZKEVM_MODULES_COMMON} \
		 ${CONSTANTS_OSAKA} \
		 ${HUB_OSAKA}

all: zkevm_osaka.bin

zkevm_osaka.bin: ${ZKEVM_MODULES_OSAKA}
	${GO_CORSET_COMPILE} -o $@ ${ZKEVM_MODULES_OSAKA}
	@$(call warn_lispX)
