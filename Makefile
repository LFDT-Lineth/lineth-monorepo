GO_CORSET ?= go-corset
GIT_COMMIT := $(shell git rev-parse HEAD)
GIT_TAGS := $(shell git -P tag --points-at)
TIMESTAMP := $(shell date)
GO_CORSET_COMPILE := ${GO_CORSET} compile -Dtags="${GIT_TAGS}" -Dcommit="${GIT_COMMIT}" -Dtimestamp="${TIMESTAMP}"

# Modules setting
## Some modules set below are fork specific. Eg. For OOB, OOB_LONDON is the OOB module for London and OOB_SHANGHAI the OOB module for Shanghai.
## The discrimination is done by having one bin file per fork - see command line below

ALU := alu/add/add.zkasm alu/ext alu/mod alu/mul

BIN := bin   

BLAKE2f_MODEXP_DATA := blake2fmodexpdata

# constraints used in prod for LINEA, with linea block gas limit
BLOCKDATA_LONDON := blockdata/london

BLOCKDATA_PARIS := blockdata/paris

BLOCKDATA_CANCUN := blockdata/cancun

BLOCKHASH := blockhash

BLS_CANCUN := $(wildcard blsdata/cancun/*.lisp) \
	       $(wildcard blsdata/cancun/generalities/cancun_restriction.lisp) \
		   $(wildcard blsdata/cancun/generalities/constancy_conditions.lisp) \
		   $(wildcard blsdata/cancun/generalities/constraining_address_sum.lisp) \
		   $(wildcard blsdata/cancun/generalities/constraining_flag_sum.lisp) \
		   $(wildcard blsdata/cancun/generalities/constraints_for_bls_stamp.lisp) \
		   $(wildcard blsdata/cancun/generalities/constraints_for_ct.lisp) \
		   $(wildcard blsdata/cancun/generalities/id_increment_constraints.lisp) \
		   $(wildcard blsdata/cancun/generalities/legal_transition_constraints.lisp) \
		   $(wildcard blsdata/cancun/generalities/setting_acc_inputs.lisp) \
		   $(wildcard blsdata/cancun/generalities/setting_ct_max.lisp) \
		   $(wildcard blsdata/cancun/generalities/setting_index_max.lisp) \
		   $(wildcard blsdata/cancun/generalities/setting_index.lisp) \
		   $(wildcard blsdata/cancun/generalities/setting_is_first_input_and_is_second_input.lisp) \
		   $(wildcard blsdata/cancun/generalities/setting_phase.lisp) \
		   $(wildcard blsdata/cancun/generalities/setting_total_size.lisp) \
		   $(wildcard blsdata/cancun/generalities/shorthands.lisp) \
	       $(wildcard blsdata/cancun/lookups/*.lisp) \
	       $(wildcard blsdata/cancun/specialized_constraints/*.lisp) \
	       $(wildcard blsdata/cancun/top_level_flags_mint_mext_wtrv_wnon/*.lisp) \
		   $(wildcard blsdata/cancun/utilities/*.lisp) \

BLS_PRAGUE := $(wildcard blsdata/cancun/*.lisp) \
		   $(wildcard blsdata/cancun/generalities/constancy_conditions.lisp) \
		   $(wildcard blsdata/cancun/generalities/constraining_address_sum.lisp) \
		   $(wildcard blsdata/cancun/generalities/constraining_flag_sum.lisp) \
		   $(wildcard blsdata/cancun/generalities/constraints_for_bls_stamp.lisp) \
		   $(wildcard blsdata/cancun/generalities/constraints_for_ct.lisp) \
		   $(wildcard blsdata/cancun/generalities/id_increment_constraints.lisp) \
		   $(wildcard blsdata/cancun/generalities/legal_transition_constraints.lisp) \
		   $(wildcard blsdata/cancun/generalities/setting_acc_inputs.lisp) \
		   $(wildcard blsdata/cancun/generalities/setting_ct_max.lisp) \
		   $(wildcard blsdata/cancun/generalities/setting_index_max.lisp) \
		   $(wildcard blsdata/cancun/generalities/setting_index.lisp) \
		   $(wildcard blsdata/cancun/generalities/setting_is_first_input_and_is_second_input.lisp) \
		   $(wildcard blsdata/cancun/generalities/setting_phase.lisp) \
		   $(wildcard blsdata/cancun/generalities/setting_total_size.lisp) \
		   $(wildcard blsdata/cancun/generalities/shorthands.lisp) \
	       $(wildcard blsdata/cancun/lookups/*.lisp) \
	       $(wildcard blsdata/cancun/specialized_constraints/*.lisp) \
	       $(wildcard blsdata/cancun/top_level_flags_mint_mext_wtrv_wnon/*.lisp) \
		   $(wildcard blsdata/cancun/utilities/*.lisp) \

CONSTANTS := constants/constants.lisp

CONSTANTS_LONDON := constants/london/constants.lisp

CONSTANTS_CANCUN := constants/cancun/constants.lisp

CONSTANTS_PRAGUE := constants/prague/constants.lisp

EC_DATA := ecdata

EUC := euc

EXP := exp/exp.zkasm

GAS := gas/gas.zkasm

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

OOB_CANCUN := $(wildcard oob/cancun/lookups/*.lisp) \
		   $(wildcard oob/cancun/opcodes/*.lisp) \
		   $(wildcard oob/cancun/precompiles/*.lisp) \
		   $(wildcard oob/cancun/binarities.lisp) \
		   $(wildcard oob/cancun/cancun_restriction.lisp) \
		   $(wildcard oob/cancun/columns.lisp) \
		   $(wildcard oob/cancun/constancies.lisp) \
		   $(wildcard oob/cancun/constants.lisp) \
		   $(wildcard oob/cancun/decoding.lisp) \
		   $(wildcard oob/cancun/heartbeat.lisp) \
		   $(wildcard oob/cancun/shorthands.lisp) \
		   $(wildcard oob/cancun/specialized.lisp) \

OOB_PRAGUE := $(wildcard oob/cancun/lookups/*.lisp) \
		   $(wildcard oob/cancun/opcodes/*.lisp) \
		   $(wildcard oob/cancun/precompiles/*.lisp) \
		   $(wildcard oob/cancun/binarities.lisp) \
		   $(wildcard oob/cancun/columns.lisp) \
		   $(wildcard oob/cancun/constancies.lisp) \
		   $(wildcard oob/cancun/constants.lisp) \
		   $(wildcard oob/cancun/decoding.lisp) \
		   $(wildcard oob/cancun/heartbeat.lisp) \
		   $(wildcard oob/cancun/shorthands.lisp) \
		   $(wildcard oob/cancun/specialized.lisp) \

RLP_ADDR := rlpaddr

RLP_TXN_LONDON := rlptxn/london

RLP_TXN_CANCUN := rlptxn/cancun

RLP_TXRCPT := rlptxrcpt			

RLP_UTILS_CANCUN :=rlputils/cancun

ROM := rom

ROM_LEX := romlex

SHAKIRA_DATA := shakiradata

SHIFT :=  shf/shf.zkasm

STP := stp

TABLES_LONDON := reftables/*.lisp \
				reftables/london/inst_decoder.lisp

TABLES_CANCUN := reftables/*.lisp \
				reftables/cancun/bls_reftable.lisp \
				reftables/cancun/inst_decoder.lisp \
				reftables/cancun/power.lisp

# reftables/cancun/bls_reftable.lisp is only used in PRAGUE, but adding it in CANCUN already allows to do not duplicate OOB

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
		 ${BLOCKDATA_CANCUN} \
		 ${BLS_CANCUN} \
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
		 ${BLOCKDATA_CANCUN} \
		 ${BLS_PRAGUE} \
		 ${HUB_CANCUN} \
		 ${LOG_INFO_CANCUN} \
		 ${MMIO_CANCUN} \
		 ${MXP_CANCUN} \
		 ${OOB_PRAGUE} \
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
