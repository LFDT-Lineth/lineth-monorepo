ROM := rom/columns.lisp \
	rom/constraints.lisp

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

MXP := mxp/columns.lisp \
	   mxp/constraints.lisp \
	   mxp/mxp_into_instruction_decoder.lisp
	   

TABLES := $(wildcard lookup_tables/tables/*lisp)

PUB_DATA := $(wildcard public_data_commitments/data/*lisp) \
		$(wildcard public_data_commitments/hash/*lisp)

MEMORY := $(wildcard hub/mmio/*lisp) \
	  $(wildcard hub/mmu/*lisp) \
	  $(wildcard hub/*lisp) \

RLP := rlp/columns.lisp \
	  rlp/constraints.lisp

ZKEVM_FILES := ${ROM} ${STACK} ${ALU} ${BIN} ${SHIFT} ${WCP} ${TABLES} ${PUB_DATA} ${MXP} # ${RLP}

zkevm.go: ${ZKEVM_FILES}
	corset wizard-iop -vv -P define -o $@ ${ZKEVM_FILES}

zkevm.bin: ${ZKEVM_FILES}
	corset compile -vv -o $@ ${ZKEVM_FILES}
