(module hub)

(defconst
  PATTERN_ZERO_ITEMS    0
  PATTERN_ONE_ITEM      1
  PATTERN_TWO_ITEMS     2
  PATTERN_STANDARD      3
  PATTERN_DUP           4
  PATTERN_SWAP          5
  PATTERN_RETURN_REVERT 6
  PATTERN_COPY          7
  PATTERN_LOG           8
  PATTERN_CALL          9
  PATTERN_CREATE        10)

(defcolumns
    INSTRUCTION
    INSTRUCTION_STAMP
    INSTRUCTION_ARGUMENT_HI
    INSTRUCTION_ARGUMENT_LO
    BYTECODE_ADDRESS_HI
    BYTECODE_ADDRESS_LO
    (IS_INITCODE :binary)
    PC

    CONTEXT_NUMBER
    MAXIMUM_CONTEXT
    (CONTEXT_TYPE :binary)

    CALLER_CONTEXT
    RETURNER_CONTEXT

    CALLDATA_OFFSET
    CALLDATA_SIZE
    RETURN_OFFSET
    RETURN_CAPACITY
    RETURNDATA_OFFSET
    RETURNDATA_SIZE

    (CONTEXT_REVERTS :binary)
    (CONTEXT_REVERTS_BY_CHOICE :binary)
    (CONTEXT_REVERTS_BY_FORCE :binary)
    CONTEXT_REVERT_STORAGE_STAMP
    (CONTEXT_RUNS_OUT_OF_GAS :binary)
    (CONTEXT_ERROR :binary)

    CALLSTACK_DEPTH
    VALUE

    ;; =====================================================================
    ;; INSTRUCTION DECODED FLAGS
    ;;=====================================================================
    (STATICCALL_FLAG :binary)
    (DELEGATECALL_FLAG :binary)
    (CODECALL_FLAG :binary)
    ;;(JUMP_FLAG :binary)
    ;;(PUSH_FLAG :binary)

    ;; =====================================================================
    ;; STACK STUFF
    ;; =====================================================================
    STACK_STAMP
    STACK_STAMP_NEW
    HEIGHT
    HEIGHT_NEW

    (ITEM_HEIGHT :array [1:4])
    (VAL_LO :array [1:4])
    (VAL_HI :array [1:4])
    (ITEM_STACK_STAMP :array [1:4])
    (POP :array [1:4] :binary)

    ALPHA
    DELTA
    HEIGHT_UNDER
    HEIGHT_OVER

    (STACK_EXCEPTION			:binary)
    (STACK_UNDERFLOW_EXCEPTION	:binary)
    (STACK_OVERFLOW_EXCEPTION	:binary)

    ;; Imported from the ID
    STATIC_GAS
    INST_PARAM
    (TWO_LINES_INSTRUCTION :binary)
    STACK_PATTERN
    (COUNTER :binary)

    (FLAG_1 :binary)
    (FLAG_2 :binary)
    (FLAG_3 :binary)

    TX_NUM

    ALU_STAMP
    ;; BIN_STAMP
    RAM_STAMP
    ;; SHF_STAMP
    STO_STAMP
    ;; WCP_STAMP
    ;; WRM_STAMP

    (ARITHMETIC_INST		:binary)
    (BINARY_INST            :binary)
    (RAM_INST               :binary)
    (SHIFT_INST             :binary)
    (STORAGE_INST           :binary)
    (WORD_COMPARISON_INST   :binary)
    ;; (WRM_INST			:binary)

    (ALU_ADD_INST	:binary)
    (ALU_EXT_INST	:binary)
    (ALU_MOD_INST	:binary)
    (ALU_MUL_INST	:binary))

(definterleaved CN_POW_4 (CONTEXT_NUMBER CONTEXT_NUMBER CONTEXT_NUMBER CONTEXT_NUMBER))
(definterleaved HEIGHT_1234 ([ITEM_HEIGHT 1] [ITEM_HEIGHT 2] [ITEM_HEIGHT 3] [ITEM_HEIGHT 4]))
(definterleaved STACK_STAMP_1234 ([ITEM_STACK_STAMP 1] [ITEM_STACK_STAMP 2] [ITEM_STACK_STAMP 3] [ITEM_STACK_STAMP 4]))
(definterleaved POP_1234 ([POP 1] [POP 2] [POP 3] [POP 4]))
(definterleaved VAL_HI_1234 ([VAL_HI 1] [VAL_HI 2] [VAL_HI 3] [VAL_HI 4]))
(definterleaved VAL_LO_1234 ([VAL_LO 1] [VAL_LO 2] [VAL_LO 3] [VAL_LO 4]))

(defpermutation
  (SRT_CN_POW_4 SRT_HEIGHT_1234 SRT_STACK_STAMP_1234 SRT_POP_1234 SRT_VAL_HI_1234 SRT_VAL_LO_1234)
  ((↓ CN_POW_4) (↓ HEIGHT_1234) (↓ STACK_STAMP_1234) POP_1234 VAL_HI_1234 VAL_LO_1234)
  ())
