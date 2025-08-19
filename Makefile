GO_CORSET ?= go-corset
GIT_COMMIT := $(shell git rev-parse HEAD)
GIT_TAGS := $(shell git -P tag --points-at)
TIMESTAMP := $(shell date)
GO_CORSET_COMPILE := ${GO_CORSET} compile -Dtags="${GIT_TAGS}" -Dcommit="${GIT_COMMIT}" -Dtimestamp="${TIMESTAMP}"

# Modules setting
## Some modules set below are fork specific. Eg. For OOB, OOB_LONDON is the OOB module for London and OOB_SHANGHAI the OOB module for Shanghai.
## The discrimination is done by having one bin file per fork - see command line below

ALU_LONDON := alu/add/london alu/ext alu/mod alu/mul

ALU := alu/add/add.zkasm alu/ext alu/mod alu/mul

BIN := bin   

BLAKE2f_MODEXP_DATA := blake2fmodexpdata

# constraints used in prod for LINEA, with linea block gas limit
BLOCKDATA_LONDON := blockdata/london

BLOCKDATA_PARIS := blockdata/paris

BLOCKDATA_CANCUN := blockdata/cancun

BLOCKHASH := blockhash

CONSTANTS := constants/constants.lisp

CONSTANTS_LONDON := constants/london/constants.lisp

CONSTANTS_CANCUN := constants/cancun/constants.lisp

CONSTANTS_PRAGUE := constants/prague/constants.lisp

EC_DATA := ecdata

EUC := euc

EXP := exp

GAS := gas

HUB_LONDON :=  hub/london

HUB_SHANGHAI :=  hub/shanghai

HUB_CANCUN :=  hub/cancun

LIBRARY := library

LOG_DATA := logdata

LOG_INFO_LONDON := loginfo/london

LOG_INFO_CANCUN := loginfo/cancun

MMU :=  mmu

MMIO_LONDON := mmio/london

MMIO_CANCUN := mmio/cancun

MXP_LONDON := mxp/london

MXP_CANCUN := mxp/cancun

OOB_LONDON := oob/london

OOB_SHANGHAI := oob/shanghai

OOB_CANCUN := oob/cancun

RLP_ADDR := rlpaddr

RLP_TXN_LONDON := rlptxn/london

RLP_TXN_CANCUN := rlptxn/cancun

RLP_TXRCPT := rlptxrcpt			

RLP_UTILS_CANCUN :=rlputils/cancun

ROM := rom

ROM_LEX := romlex

SHAKIRA_DATA := shakiradata

SHIFT :=  shf

STP := stp

TABLES_LONDON := reftables/*.lisp \
				reftables/london/inst_decoder.lisp

TABLES_CANCUN := reftables/*.lisp \
				reftables/cancun/inst_decoder.lisp \
				reftables/cancun/power.lisp

TRM := trm

TXN_DATA_LONDON := txndata/london

TXN_DATA_SHANGHAI := txndata/shanghai

TXN_DATA_CANCUN := txndata/cancun

WCP := wcp

LISPX := $(shell find * -name "*.lispX")
# Warn about any lispX files
define warn_lispX
	@for FILE in ${LISPX}; do (echo "WARNING: $$FILE"); done
endef

ZKEVM_MODULES_COMMON := ${CONSTANTS} \
		 ${BIN} \
		 ${BLAKE2f_MODEXP_DATA} \
		 ${BLOCKHASH} \
		 ${EC_DATA} \
		 ${EUC} \
		 ${EXP} \
		 ${GAS} \
		 ${LIBRARY} \
		 ${LOG_DATA} \
		 ${MMU} \
		 ${RLP_ADDR} \
		 ${RLP_TXRCPT} \
		 ${ROM} \
		 ${ROM_LEX} \
		 ${SHAKIRA_DATA} \
		 ${SHIFT} \
		 ${STP} \
		 ${TRM} \
		 ${WCP}

ZKEVM_MODULES_LONDON := ${ZKEVM_MODULES_COMMON} \
		 ${CONSTANTS_LONDON} \
		 ${TABLES_LONDON} \
		 ${ALU_LONDON} \
		 ${BLOCKDATA_LONDON} \
		 ${HUB_LONDON} \
		 ${LOG_INFO_LONDON} \
		 ${MMIO_LONDON} \
		 ${MXP_LONDON} \
		 ${OOB_LONDON} \
		 ${RLP_TXN_LONDON} \
		 ${TXN_DATA_LONDON}

ZKEVM_MODULES_PARIS := ${ZKEVM_MODULES_COMMON} \
		 ${CONSTANTS_LONDON} \
		 ${TABLES_LONDON} \
		 ${ALU_LONDON} \
		 ${BLOCKDATA_PARIS} \
		 ${HUB_LONDON} \
		 ${LOG_INFO_LONDON} \
		 ${MMIO_LONDON} \
		 ${MXP_LONDON} \
		 ${OOB_LONDON} \
		 ${RLP_TXN_LONDON} \
		 ${TXN_DATA_LONDON}

ZKEVM_MODULES_SHANGHAI := ${ZKEVM_MODULES_COMMON} \
		 ${CONSTANTS_LONDON} \
		 ${TABLES_LONDON} \
		 ${ALU} \
		 ${BLOCKDATA_PARIS} \
		 ${HUB_SHANGHAI} \
		 ${LOG_INFO_LONDON} \
		 ${MMIO_LONDON} \
		 ${MXP_LONDON} \
		 ${OOB_SHANGHAI} \
		 ${RLP_TXN_LONDON} \
		 ${TXN_DATA_SHANGHAI}

ZKEVM_MODULES_CANCUN := ${ZKEVM_MODULES_COMMON} \
         ${CONSTANTS_CANCUN} \
		 ${TABLES_CANCUN} \
		 ${ALU} \
		 ${BLOCKDATA_CANCUN} \
		 ${HUB_CANCUN} \
		 ${LOG_INFO_CANCUN} \
		 ${MMIO_CANCUN} \
		 ${MXP_CANCUN} \
		 ${OOB_CANCUN} \
		 ${RLP_TXN_CANCUN} \
		 ${RLP_UTILS_CANCUN} \
		 ${TXN_DATA_CANCUN}

ZKEVM_MODULES_PRAGUE := ${ZKEVM_MODULES_COMMON} \
		 ${CONSTANTS_PRAGUE} \
		 ${TABLES_CANCUN} \
		 ${ALU} \
		 ${BLOCKDATA_CANCUN} \
		 ${HUB_CANCUN} \
		 ${LOG_INFO_CANCUN} \
		 ${MMIO_CANCUN} \
		 ${MXP_CANCUN} \
		 ${OOB_CANCUN} \
		 ${RLP_TXN_CANCUN} \
		 ${RLP_UTILS_CANCUN} \
		 ${TXN_DATA_CANCUN}

all: zkevm_london.bin zkevm_paris.bin zkevm_shanghai.bin zkevm_cancun.bin zkevm_prague.bin

zkevm_london.bin: ${ZKEVM_MODULES_LONDON}
	${GO_CORSET_COMPILE} -o $@ ${ZKEVM_MODULES_LONDON}
	@$(call warn_lispX)

zkevm_paris.bin: ${ZKEVM_MODULES_PARIS}
	${GO_CORSET_COMPILE} -o $@ ${ZKEVM_MODULES_PARIS}
	@$(call warn_lispX)

zkevm_shanghai.bin: ${ZKEVM_MODULES_SHANGHAI}
	${GO_CORSET_COMPILE} -o $@ ${ZKEVM_MODULES_SHANGHAI}

zkevm_cancun.bin: ${ZKEVM_MODULES_CANCUN}
	${GO_CORSET_COMPILE} -o $@ ${ZKEVM_MODULES_CANCUN}
	@$(call warn_lispX)

zkevm_prague.bin: ${ZKEVM_MODULES_PRAGUE}
	${GO_CORSET_COMPILE} -o $@ ${ZKEVM_MODULES_PRAGUE}
	@$(call warn_lispX)
