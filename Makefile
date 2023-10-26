CORSET=corset
ROM := rom/columns.lisp \
	rom/constraints.lisp \
	# rom/rom_into_instructionDecoder.lisp

ROM_LEX := romLex/columns.lisp romLex/constraints.lisp \
	romLex/romLex_into_rom.lisp

STACK := hub/columns.lisp \
	hub/constraints.lisp

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

BIN := bin/columns.lisp \
	   bin/constraints.lisp \
	   $(wildcard lookup_tables/binRT/*lisp) \
	   lookup_tables/plookups/bin_into_binRT.lisp \
	   # bin/hub_into_bin.lisp \

SHIFT :=  shf/columns.lisp \
	  shf/constraints.lisp \
	  $(wildcard lookup_tables/shfRT/*lisp) \
	 lookup_tables/plookups/shf_into_shfRT.lisp \
	  # shf/hub_into_shf.lisp \

WCP := wcp/columns.lisp \
	wcp/constraints.lisp \
	   # wcp/hub_into_wcp.lisp \

TRM := trm/columns.lisp \
	trm/constraints.lisp

MXP := mxp/columns.lisp \
	mxp/constraints.lisp \
	mxp/lookups/mxp_into_instruction_decoder.lisp
		   # mxp/lookups/hub_into_mxp.lisp

EC_DATA := ec_data/columns.lisp \
	ec_data/constraints.lisp \
	ec_data/ecdata_into_ext.lisp \
	ec_data/ecdata_into_wcp.lisp \
	ec_data/hub_into_ecdata.lisp \

RLP_TXN := rlp_txn/columns.lisp rlp_txn/constraints.lisp \
			rlp_txn/rlpTxn_into_rom.lisp

TABLES := $(wildcard lookup_tables/tables/*lisp)

PUB_DATA := $(shell find pub/ -iname '*.lisp')

MEMORY := $(wildcard hub/mmio/columns.lisp) \
		  $(wildcard hub/mmu/columns.lisp) \
		  lookup_tables/mmuID/columns.lisp

RLP_ADDR := rlpAddr/columns.lisp \
	  rlpAddr/constraints.lisp

RLP_TXRCPT := rlp_txrcpt/columns.lisp rlp_txrcpt/constraints.lisp

LIBRARY := rlp_patterns/constraints.lisp

ZKEVM_MODULES := ${ALU} \
	${BIN} \
	${EC_DATA} \
	${LIBRARY} \
	${MEMORY} \
	${MXP} \
	${PUB_DATA} \
	${RLP_ADDR} \
	${RLP_TXN} \
	${RLP_TXRCPT} \
	${ROM} \
	${ROM_LEX} \
	${SHIFT} \
	${STACK} \
	${TABLES} \
	${WCP}

define.go: ${ZKEVM_MODULES}
	${CORSET} wizard-iop -vv -P define -o $@ ${ZKEVM_MODULES}

zkevm.bin: ${ZKEVM_MODULES}
	${CORSET} compile -vv -o $@ ${ZKEVM_MODULES}
