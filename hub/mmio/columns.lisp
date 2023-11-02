(module mmio)

(defcolumns
	CN_A
	CN_B
	CN_C
	
	INDEX_A
	INDEX_B
	INDEX_C

	VAL_A
	VAL_B
	VAL_C

	VAL_A_NEW
	VAL_B_NEW
	VAL_C_NEW

	(BYTE_A :byte)
	(BYTE_B :byte)
	(BYTE_C :byte)

	ACC_A
	ACC_B
	ACC_C

	MICRO_INSTRUCTION_STAMP
	MICRO_INSTRUCTION

	CONTEXT_SOURCE
	CONTEXT_TARGET

	(IS_INIT :binary)

	SOURCE_LIMB_OFFSET
	TARGET_LIMB_OFFSET
	SOURCE_BYTE_OFFSET
	TARGET_BYTE_OFFSET

	SIZE
	(FAST :binary)
	(ERF :binary)
	
	STACK_VALUE_HIGH
	STACK_VALUE_LOW

	(STACK_VALUE_LO_BYTE :byte)
	(STACK_VALUE_HI_BYTE :byte)

	ACC_VAL_HI
	ACC_VAL_LO

	(EXO_IS_ROM :binary)
	(EXO_IS_LOG :binary)
	(EXO_IS_HASH :binary)		;previously EXO_IS_SHA3
	(EXO_IS_TXCD :binary)

	INDEX_X
	VAL_X
	(BYTE_X :byte)
	ACC_X

	TX_NUM
	LOG_NUM ;to be replaced with a single NUM column

	(BIN_1 :binary)
	(BIN_2 :binary)
	(BIN_3 :binary)
	(BIN_4 :binary)
	(BIN_5 :binary)

	ACC_1
	ACC_2
	ACC_3
	ACC_4
	ACC_5
	ACC_6

	POW_256_1
	POW_256_2
	
	COUNTER
)

(defalias
    MICRO_STAMP     MICRO_INSTRUCTION_STAMP
    MICRO_INST      MICRO_INSTRUCTION
    CT              COUNTER
    CN_S            CONTEXT_SOURCE
    CN_T            CONTEXT_TARGET
    SLO             SOURCE_LIMB_OFFSET
    SBO             SOURCE_BYTE_OFFSET
    TLO             TARGET_LIMB_OFFSET
    TBO             TARGET_BYTE_OFFSET
    VAL_HI          STACK_VALUE_HIGH
    VAL_LO          STACK_VALUE_LOW
    BYTE_HI         STACK_VALUE_HI_BYTE
    BYTE_LO         STACK_VALUE_LO_BYTE)