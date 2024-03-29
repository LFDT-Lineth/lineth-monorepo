CORSET ?= corset

HUB_V2 := $(wildcard hub_v2/columns/*lisp) \
	  $(wildcard hub_v2/constraints/heartbeat/*lisp) \
	  $(wildcard hub_v2/constraints/generalities/*lisp) \
	  $(wildcard hub_v2/lookups/*lisp) \
	  hub_v2/constants.lisp

ALU := alu/add/columns.lisp \
       alu/add/constraints.lisp \
       alu/ext/columns.lisp \
       alu/ext/constraints.lisp \
       alu/mod/columns.lisp \
       alu/mod/constraints.lisp \
       alu/mul/columns.lisp \
       alu/mul/constraints.lisp \
       alu/mul/helpers.lisp
# alu/add/hub_into_add.lisp \
	# alu/ext/hub_into_ext.lisp \
	# alu/mod/hub_into_mod.lisp \
	# alu/mul/hub_into_mul.lisp

BIN := bin   

BLAKE2f_MODEXP_DATA := blake2f_modexp_data

CONSTANTS := constants/constants.lisp

EC_DATA := ec_data

EUC := euc

EXP := exp

GAS := gas

LIBRARY := library/rlp_constraints_pattern.lisp

LOG_DATA := logData

LOG_INFO := logInfo

MMU := mmu

MMIO := mmio \
mmio/consistency.lisp

MXP := mxp

PUB_DATA := $(shell find pub/ -iname '*.lisp')

SHAKIRA := shakira_data

RLP_ADDR := rlpAddr

RLP_TXN := rlp_txn

RLP_TXRCPT := rlp_txrcpt			

ROM := rom

ROM_LEX := romLex

SHAKIRA := shakira

SHIFT :=  shf

STACK := hub/columns.lisp \
	 hub/constraints.lisp

STP := stp

TABLES := reference_tables/binRT.lisp \
	  reference_tables/shfRT.lisp \
	  reference_tables/instruction_decoder.lisp 

TRM := trm

TXN_DATA := txn_data 

WCP := wcp

ZKEVM_MODULES := ${ALU} \
	${BIN} \
	${BLAKE2f_MODEXP_DATA} \
	${CONSTANTS} \
	${EC_DATA} \
	${EUC} \
	${EXP} \
	${GAS} \
	${LIBRARY} \
	${LOG_DATA} \
	${LOG_INFO} \
	${MMU} \
	${MMIO} \
	${MXP} \
	${PUB_DATA} \
	${RIPSHA} \
	${RLP_ADDR} \
	${RLP_TXN} \
	${RLP_TXRCPT} \
	${ROM} \
	${ROM_LEX} \
	${SHAKIRA} \
	${SHIFT} \
	${STACK} \
	${STP} \
	${TABLES} \
	${TRM} \
	${TXN_DATA} \
	${WCP}

define.go: ${ZKEVM_MODULES}
	${CORSET} wizard-iop -vv -P define -o $@ ${ZKEVM_MODULES}
    
zkevm.bin: ${ZKEVM_MODULES}
	${CORSET} compile -vv -o $@ ${ZKEVM_MODULES}
