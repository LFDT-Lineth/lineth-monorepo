CORSET ?= corset

ALU := alu/add/columns.lisp \
	alu/add/constraints.lisp \
	alu/ext/columns.lisp \
	alu/ext/constraints.lisp \
	alu/mod/columns.lisp \
	alu/mod/constraints.lisp \
	alu/mul/columns.lisp \
	alu/mul/constraints.lisp \
	alu/mul/helpers.lisp \
#       alu/add/hub_into_add.lisp \
#       alu/ext/hub_into_ext.lisp \
#       alu/mod/hub_into_mod.lisp \
#       alu/mul/hub_into_mul.lisp

BIN := bin   

CONSTANTS := constants/constants.lisp

EC_DATA := ec_data

EUC := euc

LIBRARY := library/rlp_constraints_pattern.lisp

LOG_DATA := logData

LOG_INFO := logInfo

MMU := mmu

MMIO := mmio

MXP := mxp

PUB_DATA := $(shell find pub/ -iname '*.lisp')

RIPSHA := ripsha

RLP_ADDR := rlpAddr

RLP_TXN := rlp_txn

RLP_TXRCPT := rlp_txrcpt			

ROM := rom

ROM_LEX := romLex

SHIFT :=  shf

STACK := hub/columns.lisp \
	 hub/constraints.lisp

STP := stp/columns.lisp stp/constraints.lisp \
       stp/lookups/stp_into_mod.lisp stp/lookups/stp_into_wcp.lisp

TABLES := reference_tables/binRT.lisp \
	  reference_tables/shfRT.lisp \
	  reference_tables/instruction_decoder.lisp 

TRM := trm/columns.lisp trm/constraints.lisp

TXN_DATA := txn_data 

WCP := wcp

BLAKE2f_MODEXP_DATA := blake2f_modexp_data/

EXP := exp

ZKEVM_MODULES := ${ALU} \
	${BIN} \
	${BLAKE2f_MODEXP_DATA} \
	${CONSTANTS} \
	${EC_DATA} \
	${EUC} \
	${EXP} \
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
