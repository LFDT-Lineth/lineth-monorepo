GO_CORSET ?= go-corset
GIT_COMMIT := $(shell git rev-parse HEAD)
GIT_TAGS := $(shell git -P tag --points-at)
TIMESTAMP := $(shell date)
GO_CORSET_COMPILE := ${GO_CORSET} compile -Dtags="${GIT_TAGS}" -Dcommit="${GIT_COMMIT}" -Dtimestamp="${TIMESTAMP}"

# Modules setting
## Some modules set below are fork specific. Eg. For OOB, OOB_LON is the OOB module for London and OOB_SHAN the OOB module for Shanghai.
## The discrimination is done by having one bin file per fork - see command line below

ALU := alu

BIN := bin   

BLAKE2f_MODEXP_DATA := blake2fmodexpdata

# constraints used in prod for LINEA, with linea block gas limit
BLOCKDATA_LON := $(wildcard blockdata/london/*.lisp) \
	             $(wildcard blockdata/london/processing/*.lisp) \
	             $(wildcard blockdata/london/processing/gaslimit/common.lisp) \
	       		 $(wildcard blockdata/london/processing/gaslimit/linea.lisp) \
	       		 $(wildcard blockdata/london/lookups/*.lisp)

BLOCKDATA_PAR := $(wildcard blockdata/paris/*.lisp) \
	             $(wildcard blockdata/paris/processing/*.lisp) \
	             $(wildcard blockdata/paris/processing/gaslimit/common.lisp) \
	       		 $(wildcard blockdata/paris/processing/gaslimit/linea.lisp) \
	       		 $(wildcard blockdata/paris/lookups/*.lisp)

BLOCKHASH := blockhash

CONSTANTS := constants/constants.lisp

EC_DATA := ecdata

EUC := euc

EXP := exp

GAS := gas

HUB_LON :=  $(wildcard hub/london/*.lisp) \
			$(wildcard hub/london/**/*.lisp) \
			$(wildcard hub/london/**/**/*.lisp) \
			$(wildcard hub/london/**/**/**/*.lisp) \
			$(wildcard hub/london/**/**/**/**/*.lisp) \
			$(wildcard hub/london/**/**/**/**/**/*.lisp)

HUB_SHAN :=  $(wildcard hub/shanghai/*.lisp) \
	 		 $(wildcard hub/shanghai/**/*.lisp) \
	 		 $(wildcard hub/shanghai/**/**/*.lisp) \
			 $(wildcard hub/shanghai/**/**/**/*.lisp) \
			 $(wildcard hub/shanghai/**/**/**/**/*.lisp) \
			 $(wildcard hub/shanghai/**/**/**/**/**/*.lisp)

LIBRARY := library

LOG_DATA := logdata

LOG_INFO := loginfo

MMU :=  mmu

MMIO := mmio 

MXP := mxp

OOB_LON := $(wildcard oob/london/*.lisp) \
	       $(wildcard oob/london/**/*.lisp) \
	       $(wildcard oob/london/**/**/*.lisp)

OOB_SHAN := $(wildcard oob/shanghai/*.lisp) \
	        $(wildcard oob/shanghai/**/*.lisp) \
	        $(wildcard oob/shanghai/**/**/*.lisp)

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

TXN_DATA_LON := $(wildcard txndata/london/*.lisp) \
                $(wildcard txndata/london/**/*.lisp)

TXN_DATA_SHAN := $(wildcard txndata/shanghai/*.lisp) \
                 $(wildcard txndata/shanghai/**/*.lisp)

WCP := wcp

ZKEVM_MODULES_COMMON := ${CONSTANTS} \
		 ${ALU} \
		 ${BIN} \
		 ${BLAKE2f_MODEXP_DATA} \
		 ${BLOCKHASH} \
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
		 ${WCP}

ZKEVM_MODULES_LONDON := ${ZKEVM_MODULES_COMMON} \
		 ${BLOCKDATA_LON} \
		 ${HUB_LON} \
		 ${OOB_LON} \
		 ${TXN_DATA_LON}

ZKEVM_MODULES_PARIS := ${ZKEVM_MODULES_COMMON} \
		 ${BLOCKDATA_PAR} \
		 ${HUB_LON} \
		 ${OOB_LON} \
		 ${TXN_DATA_LON}

 ZKEVM_MODULES_SHANGHAI := ${ZKEVM_MODULES_COMMON} \
 		 ${BLOCKDATA_PAR} \
		 ${HUB_SHAN} \
		 ${OOB_SHAN} \
		 ${TXN_DATA_SHAN}

all: zkevm_london.bin zkevm_paris.bin zkevm_shanghai.bin

zkevm_london.bin: ${ZKEVM_MODULES_LONDON}
	${GO_CORSET_COMPILE} -o $@ ${ZKEVM_MODULES_LONDON}

zkevm_paris.bin: ${ZKEVM_MODULES_PARIS}
	${GO_CORSET_COMPILE} -o $@ ${ZKEVM_MODULES_PARIS}

zkevm_shanghai.bin: ${ZKEVM_MODULES_SHANGHAI}
	${GO_CORSET_COMPILE} -o $@ ${ZKEVM_MODULES_SHANGHAI}
